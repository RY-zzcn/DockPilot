package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

type TaskExecutor struct {
	Docker   DockerClient
	Detector *UpdateDetector
}

func (t TaskExecutor) Execute(ctx context.Context, task protocol.TaskPayload, logLine func(string)) protocol.TaskResultPayload {
	taskCtx, cancel := context.WithTimeout(ctx, commandTimeout())
	defer cancel()
	status := protocol.TaskResultPayload{
		TaskID: task.ID,
		Status: "success",
	}
	var err error
	switch task.Kind {
	case "detect_updates":
		var updates []protocol.UpdateDetection
		updates, err = t.detectUpdates(taskCtx, task, logLine)
		status.Updates = updates
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
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "pull", "--ignore-buildable")
	if err := runLogged(ctx, logLine, "docker", args...); err != nil {
		logLine("Compose pull with --ignore-buildable failed; retrying without that flag.")
		args = append([]string{"compose"}, composeFileArgs(path)...)
		args = append(args, "pull")
		if retryErr := runLogged(ctx, logLine, "docker", args...); retryErr != nil {
			return retryErr
		}
	}
	args = append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "up", "-d", "--remove-orphans")
	return runLogged(ctx, logLine, "docker", args...)
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
	logLine("$ " + name + " " + strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			logLine(line)
		}
	}
	return err
}

func composeImages(ctx context.Context, path string) ([]string, error) {
	args := append([]string{"compose"}, composeFileArgs(path)...)
	args = append(args, "config", "--images")
	out, err := commandCombined(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("read compose images: %w: %s", err, strings.TrimSpace(out))
	}
	seen := map[string]bool{}
	var images []string
	for _, line := range strings.Split(out, "\n") {
		image := strings.TrimSpace(line)
		if image == "" || seen[image] {
			continue
		}
		seen[image] = true
		images = append(images, image)
	}
	sort.Strings(images)
	return images, nil
}

func commandCombined(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
