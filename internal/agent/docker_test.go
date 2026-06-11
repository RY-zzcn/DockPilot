package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

func TestAgentURL(t *testing.T) {
	got, err := agentURL("https://example.com/panel")
	if err != nil {
		t.Fatalf("agent url: %v", err)
	}
	want := "wss://example.com/panel/api/agent/ws"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestParseLabels(t *testing.T) {
	labels := parseLabels("com.docker.compose.project=blog,env=prod")
	if labels["com.docker.compose.project"] != "blog" || labels["env"] != "prod" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
}

func TestAgentInstallModeUsesConfiguredValue(t *testing.T) {
	if got := agentInstallMode("docker"); got != "docker" {
		t.Fatalf("docker install mode = %q", got)
	}
	if got := agentInstallMode(" BINARY "); got != "binary" {
		t.Fatalf("binary install mode = %q", got)
	}
}

func TestConfigCapabilitiesDefaultDangerousOperationsOff(t *testing.T) {
	capabilities := Config{}.Capabilities()
	if !capabilities["detect_updates"] || !capabilities["docker_snapshot"] || !capabilities["metrics"] {
		t.Fatalf("read-only capabilities should be enabled: %#v", capabilities)
	}
	for _, key := range []string{"agent_update", "compose_update", "compose_deploy", "restart_container", "prune_images"} {
		if capabilities[key] {
			t.Fatalf("dangerous capability %s should default to disabled: %#v", key, capabilities)
		}
	}
}

func TestTaskExecutorRejectsDisabledDangerousTask(t *testing.T) {
	result := TaskExecutor{}.Execute(context.Background(), protocolTask("compose_update"), func(string) {})
	if result.Status != "failed" || !strings.Contains(result.Message, "DOCKPILOT_AGENT_ALLOW_COMPOSE_UPDATE") {
		t.Fatalf("compose_update should be locally gated, got %#v", result)
	}
}

func TestFriendlyDetectionFailureExplains1PanelEnvIssue(t *testing.T) {
	reason, advice := friendlyDetectionFailure("variable WEBSITE_DIR is not set and mount becomes :/www")
	if !strings.Contains(reason, "环境变量") || !strings.Contains(advice, "1Panel") {
		t.Fatalf("expected 1Panel/env friendly explanation, got %q / %q", reason, advice)
	}
}

func TestComposeFileArgsUseProjectDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "compose.yml")
	if err := os.WriteFile(file, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := composeFileArgs(file)
	want := []string{"--project-directory", root, "-f", file}
	if len(args) != len(want) {
		t.Fatalf("composeFileArgs length = %d, want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("composeFileArgs[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func protocolTask(kind string) protocol.TaskPayload {
	return protocol.TaskPayload{
		ID:   "task-1",
		Kind: kind,
		Args: map[string]string{"path": "/opt/app/compose.yml"},
	}
}

func TestComposeFilePathAcceptsDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "docker-compose.yml")
	if err := os.WriteFile(file, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := composeFilePath(root); got != file {
		t.Fatalf("composeFilePath(%q) = %q, want %q", root, got, file)
	}
}

func TestParseComposeImagesIgnoresWarnings(t *testing.T) {
	out := `time="2026-06-10T22:49:05+08:00" level=warning msg="/opt/app/docker-compose.yml: the attribute version is obsolete"
nginx:stable
ghcr.io/example/api:latest
level=warning
nginx:stable
redis
`
	images := parseComposeImages(out)
	want := []string{"ghcr.io/example/api:latest", "nginx:stable", "redis"}
	if len(images) != len(want) {
		t.Fatalf("parseComposeImages length = %d, want %d: %#v", len(images), len(want), images)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("parseComposeImages[%d] = %q, want %q", i, images[i], want[i])
		}
	}
}

func TestParseComposeConfigImages(t *testing.T) {
	out := `{
  "services": {
    "api": {"image": "ghcr.io/example/api:latest"},
    "worker": {"image": "nginx:stable"},
    "duplicate": {"image": "nginx:stable"},
    "build_only": {"build": "."}
  }
}`
	images, ok := parseComposeConfigImages(out)
	if !ok {
		t.Fatalf("compose config json should parse")
	}
	want := []string{"ghcr.io/example/api:latest", "nginx:stable"}
	if len(images) != len(want) {
		t.Fatalf("parseComposeConfigImages length = %d, want %d: %#v", len(images), len(want), images)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("parseComposeConfigImages[%d] = %q, want %q", i, images[i], want[i])
		}
	}
	if _, ok := parseComposeConfigImages("not json"); ok {
		t.Fatalf("invalid json should not parse")
	}
}

func TestMergeImageReferencesDedupesAndFilters(t *testing.T) {
	images := mergeImageReferences(
		[]string{"nginx:stable", "level=warning", "nginx:stable"},
		[]string{"ghcr.io/example/api:latest", "https://example.invalid/image"},
	)
	want := []string{"ghcr.io/example/api:latest", "nginx:stable"}
	if len(images) != len(want) {
		t.Fatalf("mergeImageReferences length = %d, want %d: %#v", len(images), len(want), images)
	}
	for i := range want {
		if images[i] != want[i] {
			t.Fatalf("mergeImageReferences[%d] = %q, want %q", i, images[i], want[i])
		}
	}
}

func TestComposeProjectsFromDirsSkipsNoisyDirs(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noisyDir := filepath.Join(root, "node_modules", "ignored")
	if err := os.MkdirAll(noisyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noisyDir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects := DockerClient{ComposeDirs: []string{root}}.composeProjectsFromDirs(context.Background())
	if len(projects) != 1 {
		t.Fatalf("composeProjectsFromDirs length = %d, want 1: %#v", len(projects), projects)
	}
	if projects[0].Name != "app" || projects[0].Path != filepath.Join(appDir, "compose.yml") {
		t.Fatalf("unexpected project: %#v", projects[0])
	}
}

func TestComposeProjectsFromDirsSkips1PanelResourceTemplates(t *testing.T) {
	root := t.TempDir()
	activeDir := filepath.Join(root, "1panel", "apps", "openresty", "openresty")
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	templateDir := filepath.Join(root, "1panel", "resource", "apps", "remote", "openresty", "1.29.2.5-0-noble")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml"), []byte("services:\n  app:\n    volumes:\n      - ${WEBSITE_DIR}:/www\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects := DockerClient{ComposeDirs: []string{root}}.composeProjectsFromDirs(context.Background())
	if len(projects) != 1 {
		t.Fatalf("composeProjectsFromDirs length = %d, want 1: %#v", len(projects), projects)
	}
	if projects[0].Path != filepath.Join(activeDir, "docker-compose.yml") {
		t.Fatalf("unexpected project path: %s", projects[0].Path)
	}
}

func TestComposeProjectDoesNotExposeScannedContent(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "compose.yml")
	raw := []byte("services:\n  watch-room-server:\n    image: cyc233/watch-room-server:latest\n    container_name: watch-room-server\n    restart: unless-stopped\n    environment:\n      PASSWORD: secret\n")
	if err := os.WriteFile(file, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	project := composeProject("app", file, false)
	if project.Content != "" {
		t.Fatalf("scanned compose content should be hidden, got %q", project.Content)
	}
	if project.ContentHash != composeFileHash(file) || project.ContentHash == "" {
		t.Fatalf("scanned compose hash should be reported without content, got %q", project.ContentHash)
	}
	if !strings.Contains(project.ContentPreview, "image: cyc233/watch-room-server:latest") || !strings.Contains(project.ContentPreview, "restart: unless-stopped") {
		t.Fatalf("scanned compose preview should include safe service fields, got %q", project.ContentPreview)
	}
	if strings.Contains(project.ContentPreview, "PASSWORD") || strings.Contains(project.ContentPreview, "secret") || strings.Contains(project.ContentPreview, "environment") {
		t.Fatalf("scanned compose preview leaked sensitive fields: %q", project.ContentPreview)
	}
}

func TestParseContainerManifestDigests(t *testing.T) {
	arrayOutput := `[{"ImageManifestDescriptor":{"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]`
	objectOutput := `{"ImageManifestDescriptor":{"digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
	if got := parseContainerManifestDigests(arrayOutput); len(got) != 1 || got[0] != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected array manifest digests: %#v", got)
	}
	if got := parseContainerManifestDigests(objectOutput); len(got) != 1 || got[0] != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("unexpected object manifest digests: %#v", got)
	}
}

func TestDigestFromInspectOutput(t *testing.T) {
	out := "Name: nginx:stable\nDigest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"
	got := digestFromInspectOutput(out)
	want := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
	if digestOnly("not-a-digest") != "" {
		t.Fatalf("non-digest text should not be accepted")
	}
}

func TestUpdateAvailable(t *testing.T) {
	local := localImageInfo{ConfigDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	remote := remoteImageInfo{ConfigDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if updateAvailable(local, remote) {
		t.Fatalf("same digest should not need update")
	}
	remote.ConfigDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if !updateAvailable(local, remote) {
		t.Fatalf("different digest should need update")
	}
	if !updateAvailable(localImageInfo{Missing: true}, remote) {
		t.Fatalf("missing local digest should be treated as pull needed")
	}
}

func TestUpdateAvailableUsesManifestDigestBeforeConfig(t *testing.T) {
	local := localImageInfo{
		ConfigDigest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ManifestDigests: []string{"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	remote := remoteImageInfo{
		ConfigDigest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ManifestDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}
	if updateAvailable(local, remote) {
		t.Fatalf("matching manifest digest should not need update even when config digests differ")
	}
}

func TestRawManifestPlatformMatch(t *testing.T) {
	raw := `{
  "schemaVersion": 2,
  "manifests": [
    {"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","platform":{"os":"linux","architecture":"amd64"}},
    {"digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","platform":{"os":"linux","architecture":"arm64"}}
  ]
}`
	info := parseRawManifestInfo(raw, platformSpec{OS: "linux", Architecture: "arm64"})
	if info.ManifestDigest != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("unexpected manifest digest: %#v", info)
	}
}

func TestPinnedImageDetectionDoesNotCallRegistry(t *testing.T) {
	called := false
	detector := NewUpdateDetector(time.Minute)
	detector.command = func(context.Context, string, ...string) (string, error) {
		return "", errors.New("missing")
	}
	detector.registry = func(context.Context, string, platformSpec) (remoteImageInfo, error) {
		called = true
		return remoteImageInfo{}, nil
	}
	result := detector.Detect(context.Background(), "nginx@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if called {
		t.Fatal("registry should not be called for pinned image")
	}
	if !result.Pinned || result.UpdateAvailable {
		t.Fatalf("unexpected pinned result: %#v", result)
	}
}

func TestDetectorUsesRegistryConfigDigest(t *testing.T) {
	detector := NewUpdateDetector(time.Minute)
	detector.command = func(ctx context.Context, name string, args ...string) (string, error) {
		return `[{"Id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","RepoDigests":["nginx@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"],"Os":"linux","Architecture":"amd64"}]`, nil
	}
	detector.registry = func(context.Context, string, platformSpec) (remoteImageInfo, error) {
		return remoteImageInfo{
			ConfigDigest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ManifestDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			Method:         "registry",
		}, nil
	}
	result := detector.Detect(context.Background(), "nginx:stable")
	if !result.UpdateAvailable || result.Method != "registry" {
		t.Fatalf("unexpected registry result: %#v", result)
	}
}

func TestDetectorUsesContainerManifestDigest(t *testing.T) {
	detector := NewUpdateDetector(time.Minute)
	detector.command = func(ctx context.Context, name string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "image" && args[1] == "inspect" {
			return `[{"Id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","RepoDigests":["nginx@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"],"Os":"linux","Architecture":"amd64"}]`, nil
		}
		if len(args) >= 2 && args[0] == "ps" {
			return "0123456789ab\n", nil
		}
		if len(args) >= 3 && args[0] == "container" && args[1] == "inspect" {
			return `[{"ImageManifestDescriptor":{"digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}}]`, nil
		}
		return "", errors.New("unexpected command")
	}
	detector.registry = func(context.Context, string, platformSpec) (remoteImageInfo, error) {
		return remoteImageInfo{
			ConfigDigest:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ManifestDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			Method:         "registry",
		}, nil
	}
	result := detector.Detect(context.Background(), "nginx:stable")
	if result.UpdateAvailable || result.LocalManifestDigest != "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd" {
		t.Fatalf("unexpected manifest digest result: %#v", result)
	}
}

func TestDetectorFallsBackToCLI(t *testing.T) {
	detector := NewUpdateDetector(time.Minute)
	detector.command = func(ctx context.Context, name string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "image" && args[1] == "inspect" {
			return `[{"Id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","RepoDigests":["nginx@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"],"Os":"linux","Architecture":"amd64"}]`, nil
		}
		return `{"schemaVersion":2,"manifests":[{"digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","platform":{"os":"linux","architecture":"amd64"}}]}`, nil
	}
	detector.registry = func(context.Context, string, platformSpec) (remoteImageInfo, error) {
		return remoteImageInfo{}, errors.New("registry unavailable")
	}
	result := detector.Detect(context.Background(), "nginx:stable")
	if !result.UpdateAvailable || result.Method != "cli" {
		t.Fatalf("unexpected cli fallback result: %#v", result)
	}
}
