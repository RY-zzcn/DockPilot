package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/version"
)

type App struct {
	cfg       Config
	store     *Store
	auth      *AuthService
	notifier  *Notifier
	hub       *AgentHub
	scheduler *Scheduler
	releases  *ReleaseService
	mux       *http.ServeMux
}

func NewApp(cfg Config) (*App, error) {
	if loc, err := time.LoadLocation(cfg.TimeZone); err == nil {
		time.Local = loc
		_ = os.Setenv("TZ", cfg.TimeZone)
	}
	store, err := OpenStore(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureAdmin(cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return nil, err
	}
	notifier := NewNotifier(store)
	releases := NewReleaseService(cfg.ReleaseRepo, cfg.ReleaseCacheTTL)
	app := &App{
		cfg:      cfg,
		store:    store,
		auth:     NewAuthService(cfg.AuthSecret),
		notifier: notifier,
		releases: releases,
		mux:      http.NewServeMux(),
	}
	app.hub = NewAgentHub(cfg, store, notifier)
	app.scheduler = NewScheduler(store, app.hub, notifier, releases, cfg)
	app.routes()
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	go a.scheduler.Run(ctx)
	server := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           a,
		ReadHeaderTimeout: 15 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("dockpilot server listening on %s", a.cfg.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *App) Close() error {
	return a.store.Close()
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	a.mux.ServeHTTP(w, r)
}

func (a *App) routes() {
	a.mux.HandleFunc("/api/agent/ws", a.hub.HandleWebSocket)
	a.mux.HandleFunc("/api/auth/login", a.handleLogin)
	a.mux.Handle("/api/", a.authenticated(http.HandlerFunc(a.handleAPI)))
	a.mux.HandleFunc("/", a.serveStatic)
}

func (a *App) authenticated(next http.Handler) http.Handler {
	return a.auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := CurrentUser(r)
		id, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid user")
			return
		}
		user, err := a.store.GetUserByID(id)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		if user.Username != claims.Username || user.Role != claims.Role {
			writeError(w, http.StatusUnauthorized, "user changed")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.store.GetUserByUsername(body.Username)
	if err != nil || !VerifyPassword(body.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, err := a.auth.Issue(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (a *App) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	switch {
	case path == "/auth/me" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, CurrentUser(r))
	case path == "/auth/refresh" && r.Method == http.MethodPost:
		a.handleRefresh(w, r)
	case path == "/overview" && r.Method == http.MethodGet:
		a.handleOverview(w, r)
	case path == "/version" && r.Method == http.MethodGet:
		a.handleVersion(w, r)
	case path == "/settings/runtime" && r.Method == http.MethodGet:
		RequireAdmin(a.handleRuntimeSettings)(w, r)
	case path == "/settings/runtime" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
		RequireAdmin(a.handleSaveRuntimeSettings)(w, r)
	case path == "/nodes" && r.Method == http.MethodGet:
		a.handleListNodes(w, r)
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodGet:
		a.handleGetNode(w, r, strings.TrimPrefix(path, "/nodes/"))
	case strings.HasPrefix(path, "/nodes/") && (r.Method == http.MethodPatch || r.Method == http.MethodPut):
		nodeID := strings.TrimPrefix(path, "/nodes/")
		RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
			a.handleUpdateNode(w, r, nodeID)
		})(w, r)
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodDelete:
		nodeID := strings.TrimPrefix(path, "/nodes/")
		RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
			a.handleDeleteNode(w, r, nodeID)
		})(w, r)
	case path == "/docker/state" && r.Method == http.MethodGet:
		a.handleDockerState(w, r)
	case path == "/docker/compose/import" && r.Method == http.MethodPost:
		RequireAdmin(a.handleImportCompose)(w, r)
	case path == "/docker/compose" && r.Method == http.MethodPost:
		RequireAdmin(a.handleSaveCompose)(w, r)
	case path == "/tasks" && r.Method == http.MethodGet:
		a.handleListTasks(w, r)
	case path == "/tasks" && r.Method == http.MethodPost:
		RequireAdmin(a.handleCreateTask)(w, r)
	case path == "/tasks" && r.Method == http.MethodDelete:
		RequireAdmin(a.handleClearTasks)(w, r)
	case path == "/update-records" && r.Method == http.MethodGet:
		a.handleUpdateRecords(w, r)
	case strings.HasSuffix(path, "/logs") && strings.HasPrefix(path, "/tasks/") && r.Method == http.MethodGet:
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/logs")
		a.handleTaskLogs(w, r, taskID)
	case strings.HasSuffix(path, "/cancel") && strings.HasPrefix(path, "/tasks/") && r.Method == http.MethodPost:
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/tasks/"), "/cancel")
		RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
			a.handleCancelTask(w, r, taskID)
		})(w, r)
	case path == "/policies" && r.Method == http.MethodGet:
		a.handleListPolicies(w, r)
	case path == "/policies" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
		RequireAdmin(a.handleUpsertPolicy)(w, r)
	case path == "/notifications" && r.Method == http.MethodGet:
		RequireAdmin(a.handleListNotifications)(w, r)
	case path == "/notifications" && (r.Method == http.MethodPost || r.Method == http.MethodPut):
		RequireAdmin(a.handleUpsertNotification)(w, r)
	case path == "/users" && r.Method == http.MethodGet:
		RequireAdmin(a.handleListUsers)(w, r)
	case path == "/users" && r.Method == http.MethodPost:
		RequireAdmin(a.handleCreateUser)(w, r)
	case path == "/settings/install" && r.Method == http.MethodGet:
		RequireAdmin(a.handleInstallInfo)(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	claims := CurrentUser(r)
	id, _ := strconv.ParseInt(claims.Subject, 10, 64)
	user, err := a.store.GetUserByID(id)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}
	token, err := a.auth.Issue(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := a.store.Overview()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleVersion(w http.ResponseWriter, r *http.Request) {
	info := version.Current()
	releaseCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	release := a.releases.Latest(releaseCtx, info.Version, r.URL.Query().Get("refresh") == "1")
	settings := a.runtimeSettings()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     info.Version,
		"commit":      info.Commit,
		"build_date":  info.BuildDate,
		"time_zone":   a.cfg.TimeZone,
		"server_time": time.Now().In(time.Local).Format(time.RFC3339),
		"release":     release,
		"settings":    settings,
	})
}

type RuntimeSettings struct {
	ReleaseRepo                    string `json:"release_repo"`
	ReleaseCacheSeconds            int64  `json:"release_cache_seconds"`
	AgentAutoUpdate                bool   `json:"agent_auto_update"`
	AgentAutoUpdateVersion         string `json:"agent_auto_update_version"`
	AgentAutoUpdateIntervalSeconds int64  `json:"agent_auto_update_interval_seconds"`
}

func (a *App) runtimeSettings() RuntimeSettings {
	return RuntimeSettings{
		ReleaseRepo:                    a.cfg.ReleaseRepo,
		ReleaseCacheSeconds:            int64(a.cfg.ReleaseCacheTTL.Seconds()),
		AgentAutoUpdate:                parseStoredBool(a.store.Setting("agent_auto_update", strconv.FormatBool(a.cfg.AgentAutoUpdate)), a.cfg.AgentAutoUpdate),
		AgentAutoUpdateVersion:         nonEmpty(a.store.Setting("agent_auto_update_version", a.cfg.AgentAutoUpdateVersion), "latest"),
		AgentAutoUpdateIntervalSeconds: int64(a.cfg.AgentAutoUpdateInterval.Seconds()),
	}
}

func (a *App) handleRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.runtimeSettings())
}

func (a *App) handleSaveRuntimeSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentAutoUpdate        *bool  `json:"agent_auto_update"`
		AgentAutoUpdateVersion string `json:"agent_auto_update_version"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.AgentAutoUpdate != nil {
		if err := a.store.SetSetting("agent_auto_update", strconv.FormatBool(*body.AgentAutoUpdate)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if strings.TrimSpace(body.AgentAutoUpdateVersion) != "" {
		if err := a.store.SetSetting("agent_auto_update_version", version.Clean(body.AgentAutoUpdateVersion)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, a.runtimeSettings())
}

func (a *App) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.store.ListNodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (a *App) handleGetNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	node, err := a.store.GetNode(nodeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	state, _ := a.store.DockerState(nodeID)
	writeJSON(w, http.StatusOK, map[string]any{
		"node":   node,
		"online": a.hub.IsOnline(nodeID),
		"docker": state,
	})
}

func (a *App) handleUpdateNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	var body struct {
		Name string `json:"name"`
		Note string `json:"note"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	node, err := a.store.UpdateNode(nodeID, body.Name, body.Note)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (a *App) handleDeleteNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	if _, err := a.store.GetNode(nodeID); err != nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	a.hub.Disconnect(nodeID)
	if err := a.store.DeleteNode(nodeID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *App) handleDockerState(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}
	state, err := a.store.DockerState(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (a *App) handleSaveCompose(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID    string `json:"node_id"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Path      string `json:"path"`
		Content   string `json:"content"`
		DeployNow bool   `json:"deploy_now"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Path = filepath.Clean(strings.TrimSpace(body.Path))
	if body.NodeID == "" || body.Name == "" || body.Path == "" {
		writeError(w, http.StatusBadRequest, "node_id, name and path are required")
		return
	}
	if !filepath.IsAbs(body.Path) {
		writeError(w, http.StatusBadRequest, "compose path must be an absolute path")
		return
	}
	if body.ID != "" {
		existing, err := a.store.GetComposeProject(body.NodeID, body.ID)
		if err == nil && !existing.Managed {
			writeError(w, http.StatusForbidden, "scanned compose projects are read-only; create a panel-managed compose project to edit from the panel")
			return
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	existingByPath, err := a.store.GetComposeProjectByPath(body.NodeID, body.Path)
	if err == nil && existingByPath.ID != body.ID && !existingByPath.Managed {
		writeError(w, http.StatusForbidden, "this compose path belongs to a scanned host project and cannot be taken over from the panel")
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	project, err := a.store.SaveComposeProject(body.NodeID, body.ID, body.Name, body.Path, body.Content, CurrentUser(r).Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.DeployNow {
		if err := a.ensureNodeCapability(body.NodeID, "compose_deploy"); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		args := map[string]string{"path": project.Path, "content": project.Content, "name": project.Name}
		payload, _ := json.Marshal(args)
		task, err := a.store.CreateTask(Task{
			NodeID:      body.NodeID,
			Kind:        "compose_deploy",
			TargetType:  "compose",
			TargetID:    project.ID,
			RequestedBy: CurrentUser(r).Username,
			Payload:     string(payload),
		})
		if err == nil {
			_ = a.hub.EnqueueTask(task)
		}
	}
	writeJSON(w, http.StatusOK, project)
}

func (a *App) handleImportCompose(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID  string `json:"node_id"`
		ID      string `json:"id"`
		Mode    string `json:"mode"`
		Content string `json:"content"`
		Confirm bool   `json:"confirm"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	body.Mode = strings.TrimSpace(body.Mode)
	if body.NodeID == "" || body.ID == "" {
		writeError(w, http.StatusBadRequest, "node_id and id are required")
		return
	}
	project, err := a.store.GetComposeProject(body.NodeID, body.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "compose project not found")
		return
	}
	if project.Managed {
		writeError(w, http.StatusBadRequest, "this compose project is already panel-managed")
		return
	}
	switch body.Mode {
	case "", "read_only":
		project, err = a.store.ImportComposeProjectReadOnly(body.NodeID, body.ID)
	case "managed":
		if !body.Confirm {
			writeError(w, http.StatusBadRequest, "转为托管前需要二次确认")
			return
		}
		project, err = a.store.ImportComposeProjectManaged(body.NodeID, body.ID, body.Content, CurrentUser(r).Username)
	default:
		writeError(w, http.StatusBadRequest, "unsupported import mode")
		return
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "compose project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (a *App) handleListTasks(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tasks, err := a.store.ListTasks(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range tasks {
		tasks[i].Payload = redactTaskPayload(tasks[i].Payload)
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (a *App) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID     string            `json:"node_id"`
		Kind       string            `json:"kind"`
		TargetType string            `json:"target_type"`
		TargetID   string            `json:"target_id"`
		Args       map[string]string `json:"args"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.NodeID == "" || body.Kind == "" {
		writeError(w, http.StatusBadRequest, "node_id and kind are required")
		return
	}
	if !allowedTaskKind(body.Kind) {
		writeError(w, http.StatusBadRequest, "unsupported task kind")
		return
	}
	if err := a.ensureNodeCapability(body.NodeID, body.Kind); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	body.Args = a.prepareTaskArgs(r, body.Kind, body.Args)
	if body.Kind == "agent_update" {
		body.TargetType = nonEmpty(body.TargetType, "node")
		body.TargetID = nonEmpty(body.TargetID, body.NodeID)
	}
	payload, _ := json.Marshal(body.Args)
	task, err := a.store.CreateTask(Task{
		NodeID:      body.NodeID,
		Kind:        body.Kind,
		TargetType:  body.TargetType,
		TargetID:    body.TargetID,
		RequestedBy: CurrentUser(r).Username,
		Payload:     string(payload),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.hub.EnqueueTask(task); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func allowedTaskKind(kind string) bool {
	switch kind {
	case "detect_updates", "agent_update", "compose_update", "restart_container", "prune_images":
		return true
	default:
		return false
	}
}

func (a *App) ensureNodeCapability(nodeID, kind string) error {
	required := requiredCapability(kind)
	if required == "" {
		return nil
	}
	node, err := a.store.GetNode(nodeID)
	if err != nil {
		return err
	}
	if nodeHasCapability(node.Capabilities, required) {
		return nil
	}
	return fmt.Errorf("节点端未开启此操作能力：%s。请在节点 Agent 环境变量中开启对应 DOCKPILOT_AGENT_ALLOW_* 开关后重启 Agent", required)
}

func requiredCapability(kind string) string {
	switch kind {
	case "detect_updates":
		return "detect_updates"
	case "agent_update":
		return "agent_update"
	case "compose_update":
		return "compose_update"
	case "compose_deploy":
		return "compose_deploy"
	case "restart_container":
		return "restart_container"
	case "prune_images":
		return "prune_images"
	default:
		return ""
	}
}

func nodeHasCapability(raw, capability string) bool {
	if capability == "detect_updates" {
		return true
	}
	values := map[string]bool{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return false
	}
	return values[capability]
}

func (a *App) prepareTaskArgs(r *http.Request, kind string, args map[string]string) map[string]string {
	if args == nil {
		args = map[string]string{}
	}
	if kind != "agent_update" {
		return args
	}
	if strings.TrimSpace(args["version"]) == "" {
		settings := a.runtimeSettings()
		args["version"] = nonEmpty(settings.AgentAutoUpdateVersion, "latest")
	}
	if strings.TrimSpace(args["repo"]) == "" {
		args["repo"] = a.cfg.ReleaseRepo
	}
	if strings.TrimSpace(args["server_url"]) == "" {
		args["server_url"] = a.publicURL(r)
	}
	return args
}

func (a *App) handleTaskLogs(w http.ResponseWriter, r *http.Request, taskID string) {
	logs, err := a.store.TaskLogs(taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *App) handleClearTasks(w http.ResponseWriter, r *http.Request) {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	deleted, err := a.store.ClearTasks(scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (a *App) handleUpdateRecords(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	records, err := a.store.ListUpdateRecords(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (a *App) handleCancelTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if err := a.store.FinishTask(taskID, TaskCanceled, `{"message":"canceled by user"}`); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": TaskCanceled})
}

func (a *App) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := a.store.ListPolicies()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

func (a *App) handleUpsertPolicy(w http.ResponseWriter, r *http.Request) {
	var policy Policy
	if err := readJSON(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if policy.Scope == "" {
		policy.Scope = "global"
	}
	if policy.Mode == "" {
		policy.Mode = DefaultPolicyMode
	}
	if policy.Schedule == "" {
		policy.Schedule = DefaultPolicySchedule
	}
	saved, err := a.store.UpsertPolicy(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	notifications, err := a.store.ListNotifications()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

func (a *App) handleUpsertNotification(w http.ResponseWriter, r *http.Request) {
	var notification Notification
	if err := readJSON(r, &notification); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := a.store.UpsertNotification(notification)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	user, err := a.store.CreateUser(body.Username, body.Password, body.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleInstallInfo(w http.ResponseWriter, r *http.Request) {
	serverURL := a.publicURL(r)
	installScript := "https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh"
	agentDocker := fmt.Sprintf("curl -fsSL %s | bash -s -- install-agent-docker --server-url %s --registration-token %s", installScript, shellArg(serverURL), shellArg(a.cfg.AgentRegistrationToken))
	agentBinary := fmt.Sprintf("curl -fsSL %s | bash -s -- install-agent-binary --server-url %s --registration-token %s", installScript, shellArg(serverURL), shellArg(a.cfg.AgentRegistrationToken))
	serverDocker := fmt.Sprintf("curl -fsSL %s | bash -s -- install-server-docker --public-url %s", installScript, shellArg(serverURL))
	serverBinary := fmt.Sprintf("curl -fsSL %s | bash -s -- install-server-binary --public-url %s", installScript, shellArg(serverURL))
	interactive := fmt.Sprintf("curl -fsSL %s | bash", installScript)
	uninstall := fmt.Sprintf("curl -fsSL %s | bash -s -- uninstall", installScript)
	uninstallAgent := fmt.Sprintf("curl -fsSL %s | bash -s -- uninstall --target agent", installScript)
	uninstallServer := fmt.Sprintf("curl -fsSL %s | bash -s -- uninstall --target server", installScript)
	uninstallAll := fmt.Sprintf("curl -fsSL %s | bash -s -- uninstall --target all", installScript)
	uninstallPurge := fmt.Sprintf("curl -fsSL %s | bash -s -- uninstall --target all --purge", installScript)
	writeJSON(w, http.StatusOK, map[string]string{
		"install_script":     installScript,
		"server_url":         serverURL,
		"registration_token": a.cfg.AgentRegistrationToken,
		"interactive":        interactive,
		"docker_command":     agentDocker,
		"binary_command":     agentBinary,
		"agent_docker":       agentDocker,
		"agent_binary":       agentBinary,
		"server_docker":      serverDocker,
		"server_binary":      serverBinary,
		"uninstall":          uninstall,
		"uninstall_agent":    uninstallAgent,
		"uninstall_server":   uninstallServer,
		"uninstall_all":      uninstallAll,
		"uninstall_purge":    uninstallPurge,
	})
}

func shellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func parseStoredBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func redactTaskPayload(payload string) string {
	if payload == "" {
		return payload
	}
	values := map[string]string{}
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		return payload
	}
	changed := false
	for key := range values {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") {
			values[key] = "******"
			changed = true
		}
	}
	if !changed {
		return payload
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return payload
	}
	return string(raw)
}

func (a *App) publicURL(r *http.Request) string {
	configured := strings.TrimRight(a.cfg.PublicURL, "/")
	if !isLoopbackURL(configured) || r.Host == "" {
		return configured
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

func isLoopbackURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func (a *App) serveStatic(w http.ResponseWriter, r *http.Request) {
	path := filepath.Clean(r.URL.Path)
	if path == "/" {
		path = "/index.html"
	}
	filePath := filepath.Join(a.cfg.WebDist, strings.TrimPrefix(path, "/"))
	if _, err := os.Stat(filePath); err == nil {
		http.ServeFile(w, r, filePath)
		return
	}
	index := filepath.Join(a.cfg.WebDist, "index.html")
	if _, err := os.Stat(index); err == nil {
		http.ServeFile(w, r, index)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<!doctype html><title>DockPilot</title><main style=\"font-family:system-ui;padding:32px\"><h1>DockPilot server is running</h1><p>Build the Vue panel with <code>cd web && npm run build</code>.</p></main>"))
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
