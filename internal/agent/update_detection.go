package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

var digestPattern = regexp.MustCompile(`(?i)sha256:[a-f0-9]{32,}`)

func detectImageUpdate(ctx context.Context, image string) protocol.ImageUpdateDetection {
	result := protocol.ImageUpdateDetection{Image: image}
	localDigest, _ := localImageDigest(ctx, image)
	remoteDigest, remoteErr := remoteImageDigest(ctx, image)
	result.LocalDigest = localDigest
	result.RemoteDigest = remoteDigest
	if remoteErr != nil {
		result.Error = remoteErr.Error()
		return result
	}
	result.UpdateAvailable = updateAvailable(localDigest, remoteDigest)
	return result
}

func localImageDigest(ctx context.Context, image string) (string, error) {
	if digest := digestFromReference(image); digest != "" {
		return digest, nil
	}
	out, err := commandCombined(ctx, "docker", "image", "inspect", "--format", "{{json .RepoDigests}}", image)
	if err != nil {
		return "", fmt.Errorf("local image is missing or unavailable")
	}
	var repoDigests []string
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &repoDigests) == nil {
		for _, value := range repoDigests {
			if digest := digestFromReference(value); digest != "" {
				return digest, nil
			}
		}
	}
	if digest := digestPattern.FindString(out); digest != "" {
		return digest, nil
	}
	return "", fmt.Errorf("local image has no repo digest")
}

func remoteImageDigest(ctx context.Context, image string) (string, error) {
	out, err := commandCombined(ctx, "docker", "buildx", "imagetools", "inspect", image)
	if err == nil {
		if digest := digestFromInspectOutput(out); digest != "" {
			return digest, nil
		}
	}
	firstErr := strings.TrimSpace(out)
	out, err = commandCombined(ctx, "docker", "manifest", "inspect", "--verbose", image)
	if err == nil {
		if digest := digestFromInspectOutput(out); digest != "" {
			return digest, nil
		}
	}
	message := strings.TrimSpace(out)
	if message == "" {
		message = firstErr
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return "", fmt.Errorf("remote digest unavailable: %s", message)
}

func digestFromInspectOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "digest:") {
			if digest := digestPattern.FindString(trimmed); digest != "" {
				return digest
			}
		}
	}
	return digestPattern.FindString(output)
}

func digestFromReference(value string) string {
	if at := strings.LastIndex(value, "@"); at >= 0 {
		return digestOnly(value[at+1:])
	}
	return ""
}

func digestOnly(value string) string {
	if digest := digestPattern.FindString(value); digest != "" {
		return strings.ToLower(digest)
	}
	return strings.TrimSpace(strings.ToLower(value))
}

func updateAvailable(localDigest, remoteDigest string) bool {
	remote := digestOnly(remoteDigest)
	if remote == "" {
		return false
	}
	local := digestOnly(localDigest)
	if local == "" {
		return true
	}
	return local != remote
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
