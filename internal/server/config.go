package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const insecureDefaultAuthSecret = "change-me-auth-secret"

type Config struct {
	Addr                    string
	DataDir                 string
	DatabasePath            string
	WebDist                 string
	PublicURL               string
	TimeZone                string
	ReleaseRepo             string
	ReleaseCacheTTL         time.Duration
	AgentRegistrationToken  string
	AgentAutoUpdate         bool
	AgentAutoUpdateVersion  string
	AgentAutoUpdateInterval time.Duration
	AuthSecret              string
	AdminUsername           string
	AdminPassword           string
	HeartbeatTimeout        time.Duration
}

func LoadConfig() Config {
	dataDir := env("DOCKPILOT_DATA_DIR", "data")
	authSecret := env("DOCKPILOT_AUTH_SECRET", "")
	if isInsecureAuthSecret(authSecret) {
		authSecret = RandomToken("auth_")
	}
	return Config{
		Addr:                    env("DOCKPILOT_ADDR", ":8080"),
		DataDir:                 dataDir,
		DatabasePath:            env("DOCKPILOT_DB", filepath.Join(dataDir, "dockpilot.db")),
		WebDist:                 env("DOCKPILOT_WEB_DIST", "web/dist"),
		PublicURL:               env("DOCKPILOT_PUBLIC_URL", "http://127.0.0.1:8080"),
		TimeZone:                env("DOCKPILOT_TIMEZONE", "Asia/Shanghai"),
		ReleaseRepo:             env("DOCKPILOT_RELEASE_REPO", "RY-zzcn/DockPilot"),
		ReleaseCacheTTL:         time.Duration(envInt("DOCKPILOT_RELEASE_CACHE_SECONDS", 900)) * time.Second,
		AgentRegistrationToken:  env("DOCKPILOT_AGENT_REGISTRATION_TOKEN", "change-me-registration-token"),
		AgentAutoUpdate:         envBool("DOCKPILOT_AGENT_AUTO_UPDATE", false),
		AgentAutoUpdateVersion:  env("DOCKPILOT_AGENT_AUTO_UPDATE_VERSION", "latest"),
		AgentAutoUpdateInterval: time.Duration(envInt("DOCKPILOT_AGENT_AUTO_UPDATE_INTERVAL_SECONDS", 3600)) * time.Second,
		AuthSecret:              authSecret,
		AdminUsername:           env("DOCKPILOT_ADMIN_USER", "admin"),
		AdminPassword:           env("DOCKPILOT_ADMIN_PASSWORD", "admin"),
		HeartbeatTimeout:        time.Duration(envInt("DOCKPILOT_HEARTBEAT_TIMEOUT_SECONDS", 90)) * time.Second,
	}
}

func isInsecureAuthSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == insecureDefaultAuthSecret
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
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
