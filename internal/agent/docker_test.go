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
