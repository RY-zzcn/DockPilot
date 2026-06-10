package agent

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ServerURL         string
	NodeID            string
	NodeToken         string
	RegistrationToken string
	Name              string
	StatePath         string
	ComposeDirs       []string
	MetricsInterval   time.Duration
	SnapshotInterval  time.Duration
	UpdateCacheTTL    time.Duration
	InstallMode       string
	ReleaseRepo       string
}

type State struct {
	NodeID    string `json:"node_id"`
	NodeToken string `json:"node_token"`
}

func LoadConfig() Config {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	cfg := Config{
		ServerURL:         env("DOCKPILOT_SERVER_URL", "http://127.0.0.1:8080"),
		NodeID:            env("DOCKPILOT_NODE_ID", ""),
		NodeToken:         env("DOCKPILOT_NODE_TOKEN", ""),
		RegistrationToken: env("DOCKPILOT_REGISTRATION_TOKEN", ""),
		Name:              env("DOCKPILOT_NODE_NAME", hostname),
		StatePath:         env("DOCKPILOT_STATE_PATH", filepath.Join(home, ".dockpilot-agent.json")),
		ComposeDirs:       splitCSV(env("DOCKPILOT_COMPOSE_DIRS", "/opt,/srv,/var/www")),
		MetricsInterval:   time.Duration(envInt("DOCKPILOT_METRICS_INTERVAL_SECONDS", 15)) * time.Second,
		SnapshotInterval:  time.Duration(envInt("DOCKPILOT_SNAPSHOT_INTERVAL_SECONDS", 60)) * time.Second,
		UpdateCacheTTL:    time.Duration(envInt("DOCKPILOT_UPDATE_CACHE_SECONDS", 900)) * time.Second,
		InstallMode:       env("DOCKPILOT_INSTALL_MODE", ""),
		ReleaseRepo:       env("DOCKPILOT_RELEASE_REPO", "RY-zzcn/DockPilot"),
	}
	flag.StringVar(&cfg.ServerURL, "server", cfg.ServerURL, "DockPilot server URL")
	flag.StringVar(&cfg.RegistrationToken, "registration-token", cfg.RegistrationToken, "one-time registration token")
	flag.StringVar(&cfg.NodeID, "node-id", cfg.NodeID, "existing node id")
	flag.StringVar(&cfg.NodeToken, "node-token", cfg.NodeToken, "existing node token")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "node display name")
	flag.StringVar(&cfg.StatePath, "state", cfg.StatePath, "agent state file")
	flag.StringVar(&cfg.InstallMode, "install-mode", cfg.InstallMode, "agent install mode: binary or docker")
	flag.StringVar(&cfg.ReleaseRepo, "release-repo", cfg.ReleaseRepo, "GitHub repo for DockPilot releases")
	composeDirs := strings.Join(cfg.ComposeDirs, ",")
	flag.StringVar(&composeDirs, "compose-dirs", composeDirs, "comma-separated compose scan directories")
	flag.Parse()
	cfg.ComposeDirs = splitCSV(composeDirs)

	if state, err := LoadState(cfg.StatePath); err == nil {
		if cfg.NodeID == "" {
			cfg.NodeID = state.NodeID
		}
		if cfg.NodeToken == "" {
			cfg.NodeToken = state.NodeToken
		}
	}
	return cfg
}

func LoadState(path string) (State, error) {
	var state State
	raw, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}

func SaveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
