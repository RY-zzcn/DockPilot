package agent

import (
	"context"
	"errors"
	"testing"
	"time"
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
