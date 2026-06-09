package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	app := &App{
		cfg:      cfg,
		store:    store,
		auth:     NewAuthService(cfg.AuthSecret),
		notifier: notifier,
		mux:      http.NewServeMux(),
	}
	app.hub = NewAgentHub(cfg, store, notifier)
	app.scheduler = NewScheduler(store, app.hub, notifier, cfg.HeartbeatTimeout)
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
	a.mux.Handle("/api/", a.auth.Middleware(http.HandlerFunc(a.handleAPI)))
	a.mux.HandleFunc("/", a.serveStatic)
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
	case path == "/nodes" && r.Method == http.MethodGet:
		a.handleListNodes(w, r)
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodGet:
		a.handleGetNode(w, r, strings.TrimPrefix(path, "/nodes/"))
	case path == "/docker/state" && r.Method == http.MethodGet:
		a.handleDockerState(w, r)
	case path == "/docker/compose" && r.Method == http.MethodPost:
		RequireAdmin(a.handleSaveCompose)(w, r)
	case path == "/tasks" && r.Method == http.MethodGet:
		a.handleListTasks(w, r)
	case path == "/tasks" && r.Method == http.MethodPost:
		RequireAdmin(a.handleCreateTask)(w, r)
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
		a.handleListNotifications(w, r)
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
	writeJSON(w, http.StatusOK, map[string]string{
		"version":     info.Version,
		"commit":      info.Commit,
		"build_date":  info.BuildDate,
		"time_zone":   a.cfg.TimeZone,
		"server_time": time.Now().In(time.Local).Format(time.RFC3339),
	})
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
	if body.NodeID == "" || body.Name == "" || body.Path == "" {
		writeError(w, http.StatusBadRequest, "node_id, name and path are required")
		return
	}
	project, err := a.store.SaveComposeProject(body.NodeID, body.ID, body.Name, body.Path, body.Content, CurrentUser(r).Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.DeployNow {
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

func (a *App) handleListTasks(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tasks, err := a.store.ListTasks(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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

func (a *App) handleTaskLogs(w http.ResponseWriter, r *http.Request, taskID string) {
	logs, err := a.store.TaskLogs(taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
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
		policy.Mode = PolicyManual
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
	serverURL := strings.TrimRight(a.cfg.PublicURL, "/")
	docker := fmt.Sprintf("docker run -d --name dockpilot-agent --restart unless-stopped -e TZ=Asia/Shanghai -e DOCKPILOT_SERVER_URL=%s -e DOCKPILOT_REGISTRATION_TOKEN=%s -v /var/run/docker.sock:/var/run/docker.sock -v /opt:/opt ghcr.io/ry-zzcn/dockpilot-agent:latest", serverURL, a.cfg.AgentRegistrationToken)
	systemd := fmt.Sprintf("DOCKPILOT_SERVER_URL=%s DOCKPILOT_REGISTRATION_TOKEN=%s ./dockpilot-agent", serverURL, a.cfg.AgentRegistrationToken)
	writeJSON(w, http.StatusOK, map[string]string{
		"server_url":         serverURL,
		"registration_token": a.cfg.AgentRegistrationToken,
		"docker_command":     docker,
		"binary_command":     systemd,
	})
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
