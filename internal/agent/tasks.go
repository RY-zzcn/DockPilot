package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

type TaskExecutor struct {
	Docker            DockerClient
	Detector          *UpdateDetector
	ServerURL         string
	RegistrationToken string
	NodeName          string
	ComposeDirs       []string
	MetricsInterval   time.Duration
	SnapshotInterval  time.Duration
	UpdateCacheTTL    time.Duration
	InstallMode       string
	ReleaseRepo       string
	AgentImage        string
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
	switch task.Kind {
	case "detect_updates":
		var updates []protocol.UpdateDetection
		updates, err = t.detectUpdates(taskCtx, task, logLine)
		status.Updates = updates
	case "agent_update":
		restartAgent, err = t.agentUpdate(taskCtx, task, logLine)
	case "compose_update":
		err = t.composeUpdate(taskCtx, task, logLine)
	case "compose_deploy":
		err = t.composeDeploy(taskCtx, task, logLine)
	case "restart_container":
		err = runLogged(taskCtx, logLine, "docker", "restart", task.TargetID)
	case "prune_images":
		err = runLogged(taskCtx, logLine, "docker", "image", "prune", "-f")
	default:
		err = runLogged(taskCtx, logLine, "docker", task.Kind)
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
	images, err := composeImages(ctx, path)
	if err != nil {
		detection.Error = err.Error()
		return detection, err
	}
	if len(images) == 0 {
		logLine("No image references found in compose config: " + path)
		return detection, nil
	}
	detector := t.Detector
	if detector == nil {
		detector = NewUpdateDetector(0)
	}
	failedImages := 0
	for _, image := range images {
		item := detector.Detect(ctx, image)
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
		detection.Error = fmt.Sprintf("%d image update checks failed", failedImages)
	}
	return detection, nil
}

func (t TaskExecutor) composeUpdate(ctx context.Context, task protocol.TaskPayload, logLine func(string)) error {
	path := task.Args["path"]
	if path == "" {
		path = task.TargetID
	}
	path = composeFilePath(path)
	if path == "" {
		return fmt.Errorf("compose path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("compose file not found: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("compose file not found in directory: %s", path)
	}
	projectDir := composeProjectDir(path)
	logLine("Compose project directory: " + projectDir)
	if err := composePreflight(ctx, path); err != nil {
		return err
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
			return retryErr
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
		return err
	}
	after, afterErr := composeContainerImageIDs(ctx, path)
	if afterErr != nil {
		logLine("Unable to read compose container images after update: " + afterErr.Error())
		return nil
	}
	logComposeImageChanges(logLine, before, after)
	return nil
}

func (t TaskExecutor) composeDeploy(ctx context.Context, task protocol.TaskPayload, logLine func(string)) error {
	path := task.Args["path"]
	content := task.Args["content"]
	if path == "" {
		path = filepath.Join("/opt", strings.TrimSpace(task.Args["name"]), "compose.yml")
	}
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
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
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "config", "--images")
	out, err := commandStdoutInDir(ctx, composeProjectDir(path), "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("read compose images: %w: %s", err, strings.TrimSpace(out))
	}
	return parseComposeImages(out), nil
}

func parseComposeImages(out string) []string {
	seen := map[string]bool{}
	var images []string
	for _, line := range strings.Split(out, "\n") {
		image := strings.TrimSpace(line)
		if image == "" || seen[image] || !isComposeImageReference(image) {
			continue
		}
		seen[image] = true
		images = append(images, image)
	}
	sort.Strings(images)
	return images
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

func logComposeImageChanges(logLine func(string), before, after map[string]string) {
	if len(after) == 0 {
		logLine("Compose update finished, but no running containers were found for this project.")
		return
	}
	changed := 0
	for name, afterID := range after {
		beforeID := before[name]
		switch {
		case beforeID == "":
			changed++
			logLine(fmt.Sprintf("Container %s is now running image %s", name, shortContainerImageID(afterID)))
		case beforeID != afterID:
			changed++
			logLine(fmt.Sprintf("Container %s image changed: %s -> %s", name, shortContainerImageID(beforeID), shortContainerImageID(afterID)))
		}
	}
	if changed == 0 {
		logLine("Compose update completed; container image IDs did not change. The project may already be current, use build-only images, or use fixed tags/digests.")
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
