package agent

import "testing"

func TestDockerImageRef(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		version string
		want    string
	}{
		{name: "default latest", image: "", version: "latest", want: "ghcr.io/ry-zzcn/dockpilot-agent:latest"},
		{name: "version tag", image: "ghcr.io/ry-zzcn/dockpilot-agent", version: "0.2.2", want: "ghcr.io/ry-zzcn/dockpilot-agent:v0.2.2"},
		{name: "strip existing tag", image: "registry.example.com/dockpilot-agent:old", version: "v0.2.2", want: "registry.example.com/dockpilot-agent:v0.2.2"},
		{name: "strip digest", image: "registry.example.com/dockpilot-agent@sha256:abc", version: "latest", want: "registry.example.com/dockpilot-agent:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dockerImageRef(tt.image, tt.version); got != tt.want {
				t.Fatalf("dockerImageRef(%q, %q) = %q, want %q", tt.image, tt.version, got, tt.want)
			}
		})
	}
}

func TestUniqueFields(t *testing.T) {
	got := uniqueFields("deploy_default bridge deploy_default\ncustom")
	want := []string{"deploy_default", "bridge", "custom"}
	if len(got) != len(want) {
		t.Fatalf("uniqueFields length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueFields[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
