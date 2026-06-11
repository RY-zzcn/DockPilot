package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

type TaskExecutor struct {
	Docker             DockerClient
	Detector           *UpdateDetector
	ServerURL          string
	RegistrationToken  string
	NodeName           string
	ComposeDirs        []string
	MetricsInterval    time.Duration
	SnapshotInterval   time.Duration
	UpdateCacheTTL     time.Duration
	InstallMode        string
	ReleaseRepo        string
	AgentImage         string
	AllowAgentUpdate   bool
	AllowComposeUpdate bool
	AllowDeploy        bool
	AllowRestart       bool
	AllowImagePrune    bool
}

func (t TaskExecutor) Execute(ctx context.Context, task protocol.TaskPayload, logLine func(string)) protocol.TaskResultPayload {
	taskCtx, cancel := context.WithTimeout(ctx, commandTimeout())
	defer cancel()
	status := protocol.TaskResultPayload{
		TaskID: task.ID,
		Status: "success",
	}
	var err error
	restartAgent := false
	if err = t.requireCapability(task.Kind); err != nil {
		status.Status = "failed"
		status.Message = err.Error()
		status.ExitCode = 1
		return status
	}
	switch task.Kind {
	case "detect_updates":
		var updates []protocol.UpdateDetection
		updates, err = t.detectUpdates(taskCtx, task, logLine)
		status.Updates = updates
	case "agent_update":
		restartAgent, err = t.agentUpdate(taskCtx, task, logLine)
	case "compose_update":
		var changes []protocol.ImageChange
		changes, err = t.composeUpdate(taskCtx, task, logLine)
		status.ImageChanges = changes
	case "compose_deploy":
		var changes []protocol.ImageChange
		changes, err = t.composeDeploy(taskCtx, task, logLine)
		status.ImageChanges = changes
	case "restart_container":
		err = runLogged(taskCtx, logLine, "docker", "restart", task.TargetID)
	case "prune_images":
		err = runLogged(taskCtx, logLine, "docker", "image", "prune", "-f")
	default:
		err = fmt.Errorf("unsupported agent task: %s", task.Kind)
	}
	if err != nil {
		status.Status = "failed"
		status.Message = err.Error()
		status.ExitCode = 1
		return status
	}
	status.RestartAgent = restartAgent
	status.Message = "completed"
	return status
}

func (t TaskExecutor) requireCapability(kind string) error {
	allowed := true
	var hint string
	switch kind {
	case "detect_updates":
		allowed = true
	case "agent_update":
		allowed = t.AllowAgentUpdate
		hint = "set DOCKPILOT_AGENT_ALLOW_AGENT_UPDATE=true on the node to allow panel-triggered agent updates"
	case "compose_update":
		allowed = t.AllowComposeUpdate
		hint = "set DOCKPILOT_AGENT_ALLOW_COMPOSE_UPDATE=true on the node to allow server-initiated Compose updates"
	case "compose_deploy":
		allowed = t.AllowDeploy
		hint = "set DOCKPILOT_AGENT_ALLOW_DEPLOY=true on the node to allow server-initiated Compose deploys"
	case "restart_container":
		allowed = t.AllowRestart
		hint = "set DOCKPILOT_AGENT_ALLOW_CONTAINER_RESTART=true on the node to allow container restarts"
	case "prune_images":
		allowed = t.AllowImagePrune
		hint = "set DOCKPILOT_AGENT_ALLOW_IMAGE_PRUNE=true on the node to allow Docker image pruning"
	}
	if allowed {
		return nil
	}
	return fmt.Errorf("agent capability %q is disabled locally; %s", kind, hint)
}

func (t TaskExecutor) detectUpdates(ctx context.Context, task protocol.TaskPayload, logLine func(string)) ([]protocol.UpdateDetection, error) {
	path := task.Args["path"]
	if path == "" {
		projects := t.Docker.composeProjects(ctx)
		if len(projects) == 0 {
			logLine("No compose projects found; running docker image list as a lightweight visibility pass.")
			return nil, runLogged(ctx, logLine, "docker", "images")
		}
		var detections []protocol.UpdateDetection
		for _, project := range projects {
			logLine("Checking compose project: " + project.Name)
			detection, err := t.detectComposeProject(ctx, project.ID, project.Name, project.Path, logLine)
			detections = append(detections, detection)
			if err != nil {
				logLine("Compose project check failed: " + err.Error())
				continue
			}
		}
		return detections, nil
	}
	name := task.Args["name"]
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}
	detection, err := t.detectComposeProject(ctx, task.TargetID, name, path, logLine)
	if err != nil {
		detection.Error = err.Error()
	}
	return []protocol.UpdateDetection{detection}, err
}

func (t TaskExecutor) detectComposeProject(ctx context.Context, projectID, name, path string, logLine func(string)) (protocol.UpdateDetection, error) {
	detection := protocol.UpdateDetection{
		TargetType:  "compose",
		TargetID:    projectID,
		ProjectName: name,
		Path:        path,
		Images:      []protocol.ImageUpdateDetection{},
	}
	if !t.pathAllowed(path) {
		err := fmt.Errorf("compose file is outside agent allowed directories: %s", path)
		detection.Error = err.Error()
		detection.Reason = "Compose 文件不在 Agent 允许扫描的目录内"
		detection.Advice = "请在节点端调整 DOCKPILOT_COMPOSE_DIRS，或把项目放到允许目录后再检测"
		return detection, err
	}
	images, err := composeImages(ctx, path)
	if err != nil {
		runtimeImages := composeRuntimeImages(ctx, name)
		if len(runtimeImages) == 0 {
			detection.Error = err.Error()
			detection.Reason, detection.Advice = friendlyDetectionFailure(err.Error())
			return detection, err
		}
		detection.Error = err.Error()
		detection.Reason, detection.Advice = friendlyDetectionFailure(err.Error())
		images = runtimeImages
		logLine("Compose config check failed; falling back to images from existing project containers: " + err.Error())
	} else {
		images = mergeImageReferences(images, composeRuntimeImages(ctx, name))
	}
	if len(images) == 0 {
		logLine("No image references found in compose config: " + path)
		return detection, nil
	}
	logLine(fmt.Sprintf("Checking %d image reference(s) for compose project: %s", len(images), path))
	detector := t.Detector
	if detector == nil {
		detector = NewUpdateDetector(0)
	}
	failedImages := 0
	for _, image := range images {
		item := detector.Detect(ctx, image)
		item.Status, item.Reason, item.Advice = imageDetectionStatus(item)
		detection.Images = append(detection.Images, item)
		switch {
		case item.Pinned:
			logLine(fmt.Sprintf("Image %s is digest-pinned for %s: %s", image, item.Platform, shortDigest(item.RemoteManifestDigest)))
		case item.Error != "":
			failedImages++
			logLine(fmt.Sprintf("Image %s check failed via %s for %s: %s", image, nonEmpty(item.Method, "unknown"), item.Platform, item.Error))
		case item.UpdateAvailable:
			logLine(fmt.Sprintf("Image %s has update via %s for %s: local %s remote %s", image, item.Method, item.Platform, shortDigest(item.LocalDigest), shortDigest(item.RemoteDigest)))
		default:
			logLine(fmt.Sprintf("Image %s is current via %s for %s: %s", image, item.Method, item.Platform, shortDigest(nonEmpty(item.LocalDigest, item.RemoteDigest))))
		}
	}
	if failedImages > 0 {
		imageError := fmt.Sprintf("%d image update checks failed", failedImages)
		if detection.Error != "" {
			detection.Error += "; " + imageError
		} else {
			detection.Error = imageError
		}
		if detection.Reason == "" {
			detection.Reason = "部分镜像无法完成更新检测"
			detection.Advice = "请查看失败镜像的原因；常见原因包括私有仓库未登录、网络超时、镜像标签不存在或 registry 权限不足"
		}
	}
	return detection, nil
}

func (t TaskExecutor) composeUpdate(ctx context.Context, task protocol.TaskPayload, logLine func(string)) ([]protocol.ImageChange, error) {
	path := task.Args["path"]
	if path == "" {
		path = task.TargetID
	}
	path = composeFilePath(path)
	if path == "" {
		return nil, fmt.Errorf("compose path is required")
	}
	if !t.pathAllowed(path) {
		return nil, fmt.Errorf("compose file is outside agent allowed directories: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("compose file not found: %s", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("compose file not found in directory: %s", path)
	}
	projectDir := composeProjectDir(path)
	logLine("Compose project directory: " + projectDir)
	if err := composePreflight(ctx, path); err != nil {
		return nil, err
	}
	healthURL := strings.TrimSpace(task.Args["healthcheck_url"])
	rollbackEnabled := taskBool(task.Args, "rollback_on_failure")
	if healthURL != "" {
		logLine("Running pre-update health check: " + healthURL)
		if err := checkHealthURL(ctx, healthURL); err != nil {
			return nil, fmt.Errorf("pre-update health check failed: %w", err)
		}
	}
	beforeState, beforeStateErr := composeContainerImageState(ctx, path)
	if beforeStateErr != nil {
		logLine("Unable to read current compose container image references before update: " + beforeStateErr.Error())
	}
	before, beforeErr := composeContainerImageIDs(ctx, path)
	if beforeErr != nil {
		logLine("Unable to read current compose container images before update: " + beforeErr.Error())
	}
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "pull", "--ignore-buildable")
	if _, err := runLoggedInDir(ctx, projectDir, logLine, "docker", args...); err != nil {
		logLine("Compose pull with --ignore-buildable failed; retrying without that flag.")
		args = append([]string{"compose"}, composeFileArgs(path)...)
		args = append(args, "pull")
		if _, retryErr := runLoggedInDir(ctx, projectDir, logLine, "docker", args...); retryErr != nil {
			return nil, retryErr
		}
	}
	args = append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "up", "-d", "--remove-orphans")
	out, err := runLoggedInDir(ctx, projectDir, logLine, "docker", args...)
	if err != nil && strings.Contains(out, "No such container") {
		logLine("Compose up hit a transient missing-container conflict; retrying once.")
		out, err = runLoggedInDir(ctx, projectDir, logLine, "docker", args...)
	}
	if err != nil {
		return nil, err
	}
	if healthURL != "" {
		logLine("Running post-update health check: " + healthURL)
		if err := checkHealthURL(ctx, healthURL); err != nil {
			logLine("Post-update health check failed: " + err.Error())
			if rollbackEnabled {
				logLine("Rollback requested; restoring previous image IDs where possible.")
				if rollbackErr := rollbackComposeImages(ctx, projectDir, path, beforeState, logLine); rollbackErr != nil {
					return nil, fmt.Errorf("post-update health check failed and rollback failed: %w; rollback: %v", err, rollbackErr)
				}
				return nil, fmt.Errorf("post-update health check failed; previous images were restored where possible: %w", err)
			}
			return nil, fmt.Errorf("post-update health check failed: %w", err)
		}
	}
	after, afterErr := composeContainerImageIDs(ctx, path)
	if afterErr != nil {
		logLine("Unable to read compose container images after update: " + afterErr.Error())
		return nil, nil
	}
	changes := composeImageChanges(task, before, after)
	logComposeImageChanges(logLine, changes)
	return changes, nil
}

func (t TaskExecutor) composeDeploy(ctx context.Context, task protocol.TaskPayload, logLine func(string)) ([]protocol.ImageChange, error) {
	if !t.AllowDeploy {
		return nil, fmt.Errorf("compose deploy is disabled on this agent; set DOCKPILOT_AGENT_ALLOW_DEPLOY=true on the node to allow server-initiated deploys")
	}
	path := task.Args["path"]
	content := task.Args["content"]
	if path == "" {
		path = filepath.Join("/opt", strings.TrimSpace(task.Args["name"]), "compose.yml")
	}
	if !t.pathAllowed(path) {
		return nil, fmt.Errorf("compose file is outside agent allowed directories: %s", path)
	}
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		logLine("Compose file written: " + path)
	}
	return t.composeUpdate(ctx, protocol.TaskPayload{
		ID:         task.ID,
		Kind:       "compose_update",
		TargetType: task.TargetType,
		TargetID:   task.TargetID,
		Args:       map[string]string{"path": path},
	}, logLine)
}

func (t TaskExecutor) pathAllowed(path string) bool {
	if path == "" {
		return false
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if evaluated, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = evaluated
	}
	if len(t.ComposeDirs) == 0 {
		return true
	}
	for _, root := range t.ComposeDirs {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if evaluated, err := filepath.EvalSymlinks(absRoot); err == nil {
			absRoot = evaluated
		}
		if resolved == absRoot || strings.HasPrefix(resolved, strings.TrimRight(absRoot, string(os.PathSeparator))+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func runLogged(ctx context.Context, logLine func(string), name string, args ...string) error {
	_, err := runLoggedInDir(ctx, "", logLine, name, args...)
	return err
}

func runLoggedInDir(ctx context.Context, dir string, logLine func(string), name string, args ...string) (string, error) {
	logLine("$ " + name + " " + strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	output := string(out)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			logLine(line)
		}
	}
	return output, err
}

func composeImages(ctx context.Context, path string) ([]string, error) {
	path = composeFilePath(path)
	projectDir := composeProjectDir(path)
	jsonArgs := append([]string{"compose"}, composeFileArgs(path)...)
	jsonArgs = append(jsonArgs, "config", "--format", "json")
	jsonOut, jsonErr := commandStdoutInDir(ctx, projectDir, "docker", jsonArgs...)
	if jsonErr == nil {
		if images, ok := parseComposeConfigImages(jsonOut); ok {
			return images, nil
		}
	}

	imageArgs := append([]string{"compose"}, composeFileArgs(path)...)
	imageArgs = append(imageArgs, "config", "--images")
	out, err := commandStdoutInDir(ctx, projectDir, "docker", imageArgs...)
	if err != nil {
		if jsonErr != nil && strings.TrimSpace(jsonOut) != "" {
			return nil, fmt.Errorf("read compose config: %w: %s", jsonErr, compactOutput(jsonOut))
		}
		return nil, fmt.Errorf("read compose images: %w: %s", err, compactOutput(out))
	}
	return parseComposeImages(out), nil
}

func parseComposeConfigImages(out string) ([]string, bool) {
	var config struct {
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	var images []string
	for _, service := range config.Services {
		image := strings.TrimSpace(service.Image)
		if image == "" || seen[image] || !isComposeImageReference(image) {
			continue
		}
		seen[image] = true
		images = append(images, image)
	}
	sort.Strings(images)
	return images, true
}

func composeRuntimeImages(ctx context.Context, projectName string) []string {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil
	}
	out, err := commandStdout(ctx, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+projectName, "--format", "{{.Image}}")
	if err != nil {
		return nil
	}
	return parseComposeImages(out)
}

func mergeImageReferences(groups ...[]string) []string {
	seen := map[string]bool{}
	var images []string
	for _, group := range groups {
		for _, image := range group {
			image = strings.TrimSpace(image)
			if image == "" || seen[image] || !isComposeImageReference(image) {
				continue
			}
			seen[image] = true
			images = append(images, image)
		}
	}
	sort.Strings(images)
	return images
}

func parseComposeImages(out string) []string {
	return mergeImageReferences(strings.Split(out, "\n"))
}

func isComposeImageReference(value string) bool {
	if strings.ContainsAny(value, " \t\r\n\"'") {
		return false
	}
	if strings.HasPrefix(value, "-") || strings.Contains(value, "://") {
		return false
	}
	return strings.Contains(value, "/") || strings.Contains(value, ":") || strings.Contains(value, "@") || !strings.Contains(value, "=")
}

func composePreflight(ctx context.Context, path string) error {
	projectDir := composeProjectDir(path)
	quietArgs := append([]string{"compose"}, composeFileArgs(path)...)
	quietArgs = append(quietArgs, "config", "--quiet")
	out, err := commandCombinedInDir(ctx, projectDir, "docker", quietArgs...)
	if err == nil {
		return nil
	}
	configArgs := append([]string{"compose"}, composeFileArgs(path)...)
	configArgs = append(configArgs, "config")
	fallbackOut, fallbackErr := commandCombinedInDir(ctx, projectDir, "docker", configArgs...)
	if fallbackErr == nil {
		return nil
	}
	if strings.TrimSpace(fallbackOut) != "" {
		out = fallbackOut
		err = fallbackErr
	}
	return fmt.Errorf("compose preflight failed for %s: %w: %s", path, err, compactOutput(out))
}

func commandCombined(ctx context.Context, name string, args ...string) (string, error) {
	return commandCombinedInDir(ctx, "", name, args...)
}

func commandStdout(ctx context.Context, name string, args ...string) (string, error) {
	return commandStdoutInDir(ctx, "", name, args...)
}

func commandStdoutInDir(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(string(out) + "\n" + stderr.String()), err
	}
	return string(out), nil
}

func commandCombinedInDir(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type composeContainerImage struct {
	Name    string
	Ref     string
	ImageID string
	Service string
	Project string
}

func composeContainerImageState(ctx context.Context, path string) (map[string]composeContainerImage, error) {
	path = composeFilePath(path)
	projectDir := composeProjectDir(path)
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "ps", "-q")
	out, err := commandStdoutInDir(ctx, projectDir, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w: %s", err, compactOutput(out))
	}
	ids := strings.Fields(out)
	if len(ids) == 0 {
		return map[string]composeContainerImage{}, nil
	}
	inspectArgs := append([]string{"inspect", "--format", "{{.Name}}={{.Config.Image}}={{.Image}}={{index .Config.Labels \"com.docker.compose.service\"}}={{index .Config.Labels \"com.docker.compose.project\"}}"}, ids...)
	out, err = commandStdout(ctx, "docker", inspectArgs...)
	if err != nil {
		return nil, fmt.Errorf("container inspect: %w: %s", err, compactOutput(out))
	}
	images := map[string]composeContainerImage{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 5)
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimPrefix(parts[0], "/")
		item := composeContainerImage{Name: name, Ref: parts[1], ImageID: parts[2]}
		if len(parts) > 3 {
			item.Service = parts[3]
		}
		if len(parts) > 4 {
			item.Project = parts[4]
		}
		images[name] = item
	}
	return images, nil
}

func composeContainerImageIDs(ctx context.Context, path string) (map[string]string, error) {
	path = composeFilePath(path)
	projectDir := composeProjectDir(path)
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "ps", "-q")
	out, err := commandStdoutInDir(ctx, projectDir, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w: %s", err, compactOutput(out))
	}
	ids := strings.Fields(out)
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	inspectArgs := append([]string{"inspect", "--format", "{{.Name}}={{.Image}}"}, ids...)
	out, err = commandStdout(ctx, "docker", inspectArgs...)
	if err != nil {
		return nil, fmt.Errorf("container inspect: %w: %s", err, compactOutput(out))
	}
	images := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimPrefix(parts[0], "/")
		images[name] = parts[1]
	}
	return images, nil
}

func checkHealthURL(ctx context.Context, rawURL string) error {
	client := http.Client{Timeout: 15 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= 6; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return lastErr
}

func rollbackComposeImages(ctx context.Context, projectDir, path string, before map[string]composeContainerImage, logLine func(string)) error {
	if len(before) == 0 {
		return fmt.Errorf("no previous compose containers were available for rollback")
	}
	tagged := 0
	for _, item := range before {
		ref := strings.TrimSpace(item.Ref)
		imageID := strings.TrimSpace(item.ImageID)
		if ref == "" || imageID == "" || strings.Contains(ref, "@sha256:") {
			continue
		}
		if err := runLogged(ctx, logLine, "docker", "tag", imageID, ref); err != nil {
			return fmt.Errorf("retag %s as %s: %w", shortContainerImageID(imageID), ref, err)
		}
		tagged++
	}
	if tagged == 0 {
		return fmt.Errorf("previous image references could not be safely retagged")
	}
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "up", "-d", "--remove-orphans", "--force-recreate")
	if _, err := runLoggedInDir(ctx, projectDir, logLine, "docker", args...); err != nil {
		return err
	}
	return nil
}

func composeImageChanges(task protocol.TaskPayload, before, after map[string]string) []protocol.ImageChange {
	changes := make([]protocol.ImageChange, 0, len(after))
	for name, afterID := range after {
		beforeID := before[name]
		changes = append(changes, protocol.ImageChange{
			TargetType:      task.TargetType,
			TargetID:        task.TargetID,
			Name:            name,
			PreviousVersion: beforeID,
			CurrentVersion:  afterID,
			Changed:         beforeID == "" || beforeID != afterID,
		})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return changes
}

func logComposeImageChanges(logLine func(string), changes []protocol.ImageChange) {
	if len(changes) == 0 {
		logLine("Compose update finished, but no running containers were found for this project.")
		return
	}
	changed := 0
	for _, item := range changes {
		switch {
		case item.PreviousVersion == "":
			changed++
			logLine(fmt.Sprintf("Container %s is now running image %s", item.Name, shortContainerImageID(item.CurrentVersion)))
		case item.Changed:
			changed++
			logLine(fmt.Sprintf("Container %s image changed: %s -> %s", item.Name, shortContainerImageID(item.PreviousVersion), shortContainerImageID(item.CurrentVersion)))
		}
	}
	if changed == 0 {
		logLine("Compose update completed; container image IDs did not change. The project may already be current, use build-only images, or use fixed tags/digests.")
	}
}

func taskBool(args map[string]string, key string) bool {
	switch strings.ToLower(strings.TrimSpace(args[key])) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func imageDetectionStatus(item protocol.ImageUpdateDetection) (status, reason, advice string) {
	if item.Error != "" {
		reason, advice = friendlyDetectionFailure(item.Error)
		return "failed", reason, advice
	}
	if item.Pinned {
		return "pinned", "镜像使用 digest 固定版本", "固定 digest 的镜像不会按标签检测更新；如需自动更新，请改用可更新标签"
	}
	if item.UpdateAvailable {
		return "update_available", "远端镜像摘要已变化", "可以在维护窗口内执行更新，并保留更新记录便于回溯"
	}
	return "current", "本地镜像与远端镜像一致", "无需处理"
}

func friendlyDetectionFailure(message string) (reason, advice string) {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "timeout") || strings.Contains(lower, "i/o timeout"):
		return "节点无法连接镜像仓库或网络超时", "请检查节点 DNS、网络出口、防火墙或 registry 地址是否可访问"
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "authentication required") || strings.Contains(lower, "denied") || strings.Contains(lower, "forbidden"):
		return "镜像仓库需要登录或当前账号没有权限", "请在节点机执行 docker login，或确认镜像仓库权限和访问令牌"
	case strings.Contains(lower, "not found") || strings.Contains(lower, "manifest unknown") || strings.Contains(lower, "name unknown"):
		return "镜像或标签在仓库中不存在", "请确认 Compose 中的镜像名和 tag 是否正确；1Panel 应用模板目录可忽略"
	case strings.Contains(lower, "invalid mount config") || strings.Contains(lower, ":/www") || strings.Contains(lower, "variable is not set"):
		return "Compose 配置缺少环境变量，导致挂载或配置无效", "请在项目目录补齐 .env，或在 1Panel 中修复应用变量后再检测"
	case strings.Contains(lower, "compose preflight") || strings.Contains(lower, "read compose"):
		return "Compose 文件无法被 Docker Compose 正常解析", "请在节点机项目目录运行 docker compose config 查看具体错误"
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return "镜像仓库触发访问频率限制", "请稍后重试，或登录镜像仓库账号提高访问额度"
	default:
		return "检测过程中出现未知错误", "请查看任务日志中的原始错误；如果是私有镜像，请先确认节点机可 docker pull 该镜像"
	}
}

func shortContainerImageID(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "sha256:") && len(value) > len("sha256:")+12 {
		return value[:len("sha256:")+12]
	}
	if len(value) > 19 {
		return value[:19]
	}
	if value == "" {
		return "-"
	}
	return value
}

func compactOutput(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	text := strings.Join(fields, " ")
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}
