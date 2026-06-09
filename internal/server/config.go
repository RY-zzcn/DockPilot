package server

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr                   string
	DataDir                string
	DatabasePath           string
	WebDist                string
	PublicURL              string
	TimeZone               string
	AgentRegistrationToken string
	AuthSecret             string
	AdminUsername          string
	AdminPassword          string
	HeartbeatTimeout       time.Duration
}

func LoadConfig() Config {
	dataDir := env("DOCKPILOT_DATA_DIR", "data")
	return Config{
		Addr:                   env("DOCKPILOT_ADDR", ":8080"),
		DataDir:                dataDir,
		DatabasePath:           env("DOCKPILOT_DB", filepath.Join(dataDir, "dockpilot.db")),
		WebDist:                env("DOCKPILOT_WEB_DIST", "web/dist"),
		PublicURL:              env("DOCKPILOT_PUBLIC_URL", "http://127.0.0.1:8080"),
		TimeZone:               env("DOCKPILOT_TIMEZONE", "Asia/Shanghai"),
		AgentRegistrationToken: env("DOCKPILOT_AGENT_REGISTRATION_TOKEN", "change-me-registration-token"),
		AuthSecret:             env("DOCKPILOT_AUTH_SECRET", "change-me-auth-secret"),
		AdminUsername:          env("DOCKPILOT_ADMIN_USER", "admin"),
		AdminPassword:          env("DOCKPILOT_ADMIN_PASSWORD", "admin"),
		HeartbeatTimeout:       time.Duration(envInt("DOCKPILOT_HEARTBEAT_TIMEOUT_SECONDS", 90)) * time.Second,
	}
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
