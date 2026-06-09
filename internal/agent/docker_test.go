package agent

import "testing"

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
}

func TestUpdateAvailable(t *testing.T) {
	if updateAvailable("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("same digest should not need update")
	}
	if !updateAvailable("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") {
		t.Fatalf("different digest should need update")
	}
	if !updateAvailable("", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") {
		t.Fatalf("missing local digest should be treated as pull needed")
	}
}
