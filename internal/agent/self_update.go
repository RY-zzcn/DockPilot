package agent

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	"github.com/dockpilot/dockpilot/internal/version"
)

const defaultInstallScript = "https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh"

func (t TaskExecutor) agentUpdate(ctx context.Context, task protocol.TaskPayload, logLine func(string)) (bool, error) {
	repo := nonEmpty(task.Args["repo"], t.ReleaseRepo)
	repo = nonEmpty(repo, "RY-zzcn/DockPilot")
	targetVersion := nonEmpty(task.Args["version"], "latest")
	installMode := strings.ToLower(nonEmpty(task.Args["install_mode"], t.InstallMode))
	if installMode == "" {
		installMode = detectInstallMode()
	}
	logLine(fmt.Sprintf("Preparing agent update: current=%s target=%s mode=%s", version.Version, targetVersion, installMode))
	if installMode == "docker" {
		return false, t.scheduleInstallerUpdate(ctx, task, repo, targetVersion, "install-agent-docker", logLine)
	}
	restart, err := t.updateBinaryAgent(ctx, repo, targetVersion, logLine)
	if err != nil {
		logLine("Binary self-update failed; falling back to installer.")
		if fallbackErr := t.scheduleInstallerUpdate(ctx, task, repo, targetVersion, "install-agent-binary", logLine); fallbackErr != nil {
			return false, fmt.Errorf("%w; installer fallback failed: %v", err, fallbackErr)
		}
		return false, nil
	}
	return restart, nil
}

func (t TaskExecutor) updateBinaryAgent(ctx context.Context, repo, targetVersion string, logLine func(string)) (bool, error) {
	clean, err := resolveReleaseVersion(ctx, repo, targetVersion)
	if err != nil {
		return false, err
	}
	if version.Compare(version.Version, clean) >= 0 {
		logLine("Agent binary is already up to date.")
		return false, nil
	}
	suffix := releaseSuffix()
	asset := fmt.Sprintf("dockpilot-agent_%s_%s.tar.gz", clean, suffix)
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, clean, asset)
	logLine("Downloading " + url)
	binary, err := downloadAgentBinary(ctx, url)
	if err != nil {
		return false, err
	}
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	target, err := filepath.EvalSymlinks(executable)
	if err != nil {
		target = executable
	}
	temp := target + ".new"
	if err := os.WriteFile(temp, binary, 0o755); err != nil {
		return false, err
	}
	if err := os.Chmod(temp, 0o755); err != nil {
		_ = os.Remove(temp)
		return false, err
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return false, err
	}
	logLine(fmt.Sprintf("Agent binary updated to %s. Restarting agent process.", clean))
	return true, nil
}

func (t TaskExecutor) scheduleInstallerUpdate(ctx context.Context, task protocol.TaskPayload, repo, targetVersion, action string, logLine func(string)) error {
	serverURL := nonEmpty(task.Args["server_url"], t.ServerURL)
	registrationToken := nonEmpty(task.Args["registration_token"], t.RegistrationToken)
	nodeName := nonEmpty(task.Args["node_name"], t.NodeName)
	installScript := nonEmpty(task.Args["install_script"], defaultInstallScript)
	if serverURL == "" {
		return fmt.Errorf("server_url is required for installer update")
	}
	if registrationToken == "" {
		return fmt.Errorf("registration_token is required for installer update")
	}
	args := []string{
		"sleep 3",
		"curl -fsSL " + shellArg(installScript) +
			" | DOCKPILOT_YES=1 DOCKPILOT_REPO=" + shellArg(repo) +
			" bash -s -- " + shellArg(action) +
			" --server-url " + shellArg(serverURL) +
			" --registration-token " + shellArg(registrationToken) +
			" --node-name " + shellArg(nodeName),
	}
	if targetVersion != "" && targetVersion != "latest" {
		args[1] += " --version " + shellArg(version.EnsureVPrefix(targetVersion))
	}
	command := strings.Join(args, "; ") + " >> /tmp/dockpilot-agent-update.log 2>&1"
	logLine("Scheduling installer-based agent update in background.")
	cmd := exec.CommandContext(ctx, "sh", "-c", "nohup sh -c "+shellArg(command)+" >/dev/null 2>&1 &")
	return cmd.Run()
}

func resolveReleaseVersion(ctx context.Context, repo, targetVersion string) (string, error) {
	clean := cleanVersion(targetVersion)
	if clean != "" && clean != "latest" {
		return clean, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "DockPilot-Agent")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("release lookup failed: %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	clean = cleanVersion(body.TagName)
	if clean == "" {
		return "", fmt.Errorf("latest release has no tag")
	}
	return clean, nil
}

func downloadAgentBinary(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DockPilot-Agent")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "dockpilot-agent" {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, 128*1024*1024))
	}
	return nil, fmt.Errorf("dockpilot-agent binary not found in release asset")
}

func releaseSuffix() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS + "_" + runtime.GOARCH
	}
	switch runtime.GOARCH {
	case "amd64":
		return "linux_amd64"
	case "arm64":
		return "linux_arm64"
	case "386":
		return "linux_386"
	case "riscv64":
		return "linux_riscv64"
	case "arm":
		switch unameMachine() {
		case "armv6l", "armv6":
			return "linux_armv6"
		default:
			return "linux_armv7"
		}
	default:
		return "linux_" + runtime.GOARCH
	}
}

func unameMachine() string {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectInstallMode() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return "docker"
	}
	return "binary"
}

func cleanVersion(value string) string {
	return version.Clean(value)
}

func shellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
