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
	ServerURL          string
	NodeID             string
	NodeToken          string
	RegistrationToken  string
	Name               string
	StatePath          string
	ComposeDirs        []string
	MetricsInterval    time.Duration
	SnapshotInterval   time.Duration
	UpdateCacheTTL     time.Duration
	InstallMode        string
	ReleaseRepo        string
	AgentImage         string
	SelfUpdate         bool
	SelfUpdateInterval time.Duration
	AllowAgentUpdate   bool
	AllowComposeUpdate bool
	AllowDeploy        bool
	AllowRestart       bool
	AllowImagePrune    bool
}

type State struct {
	NodeID    string `json:"node_id"`
	NodeToken string `json:"node_token"`
}

func LoadConfig() Config {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	cfg := Config{
		ServerURL:          env("DOCKPILOT_SERVER_URL", "http://127.0.0.1:8080"),
		NodeID:             env("DOCKPILOT_NODE_ID", ""),
		NodeToken:          env("DOCKPILOT_NODE_TOKEN", ""),
		RegistrationToken:  env("DOCKPILOT_REGISTRATION_TOKEN", ""),
		Name:               env("DOCKPILOT_NODE_NAME", hostname),
		StatePath:          env("DOCKPILOT_STATE_PATH", filepath.Join(home, ".dockpilot-agent.json")),
		ComposeDirs:        splitCSV(env("DOCKPILOT_COMPOSE_DIRS", "/opt,/srv,/var/www")),
		MetricsInterval:    time.Duration(envInt("DOCKPILOT_METRICS_INTERVAL_SECONDS", 5)) * time.Second,
		SnapshotInterval:   time.Duration(envInt("DOCKPILOT_SNAPSHOT_INTERVAL_SECONDS", 60)) * time.Second,
		UpdateCacheTTL:     time.Duration(envInt("DOCKPILOT_UPDATE_CACHE_SECONDS", 900)) * time.Second,
		InstallMode:        env("DOCKPILOT_INSTALL_MODE", ""),
		ReleaseRepo:        env("DOCKPILOT_RELEASE_REPO", "RY-zzcn/DockPilot"),
		AgentImage:         env("DOCKPILOT_AGENT_IMAGE", "ghcr.io/ry-zzcn/dockpilot-agent"),
		SelfUpdate:         envBool("DOCKPILOT_AGENT_SELF_UPDATE", true),
		SelfUpdateInterval: time.Duration(envInt("DOCKPILOT_AGENT_SELF_UPDATE_INTERVAL_SECONDS", 3600)) * time.Second,
		AllowAgentUpdate:   envBool("DOCKPILOT_AGENT_ALLOW_AGENT_UPDATE", false),
		AllowComposeUpdate: envBool("DOCKPILOT_AGENT_ALLOW_COMPOSE_UPDATE", false),
		AllowDeploy:        envBool("DOCKPILOT_AGENT_ALLOW_DEPLOY", false),
		AllowRestart:       envBool("DOCKPILOT_AGENT_ALLOW_CONTAINER_RESTART", false),
		AllowImagePrune:    envBool("DOCKPILOT_AGENT_ALLOW_IMAGE_PRUNE", false),
	}
	flag.StringVar(&cfg.ServerURL, "server", cfg.ServerURL, "DockPilot server URL")
	flag.StringVar(&cfg.RegistrationToken, "registration-token", cfg.RegistrationToken, "one-time registration token")
	flag.StringVar(&cfg.NodeID, "node-id", cfg.NodeID, "existing node id")
	flag.StringVar(&cfg.NodeToken, "node-token", cfg.NodeToken, "existing node token")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "node display name")
	flag.StringVar(&cfg.StatePath, "state", cfg.StatePath, "agent state file")
	flag.StringVar(&cfg.InstallMode, "install-mode", cfg.InstallMode, "agent install mode: binary or docker")
	flag.StringVar(&cfg.ReleaseRepo, "release-repo", cfg.ReleaseRepo, "GitHub repo for DockPilot releases")
	flag.StringVar(&cfg.AgentImage, "agent-image", cfg.AgentImage, "Docker image used for agent self-update")
	flag.BoolVar(&cfg.SelfUpdate, "self-update", cfg.SelfUpdate, "automatically update agent when a newer DockPilot release exists")
	flag.BoolVar(&cfg.AllowAgentUpdate, "allow-agent-update", cfg.AllowAgentUpdate, "allow panel-triggered agent update tasks")
	flag.BoolVar(&cfg.AllowComposeUpdate, "allow-compose-update", cfg.AllowComposeUpdate, "allow compose update tasks from the server")
	flag.BoolVar(&cfg.AllowDeploy, "allow-deploy", cfg.AllowDeploy, "allow compose deploy tasks from the server")
	flag.BoolVar(&cfg.AllowRestart, "allow-container-restart", cfg.AllowRestart, "allow container restart tasks from the server")
	flag.BoolVar(&cfg.AllowImagePrune, "allow-image-prune", cfg.AllowImagePrune, "allow image prune tasks from the server")
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

func (c Config) Capabilities() map[string]bool {
	return map[string]bool{
		"detect_updates":    true,
		"docker_snapshot":   true,
		"metrics":           true,
		"agent_update":      c.AllowAgentUpdate,
		"compose_update":    c.AllowComposeUpdate,
		"compose_deploy":    c.AllowDeploy,
		"container_restart": c.AllowRestart,
		"restart_container": c.AllowRestart,
		"image_prune":       c.AllowImagePrune,
		"prune_images":      c.AllowImagePrune,
		"compose_read":      true,
		"compose_write":     c.AllowDeploy,
	}
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

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	default:
		return fallback
	}
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
