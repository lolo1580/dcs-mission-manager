package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the plugin's runtime configuration, read from environment
// variables.
//
// The plugin no longer talks to the manager over HTTP: PostgreSQL is the shared
// "library". The manager writes to it (it runs with
// DCSMANAGER_DB_DRIVER=postgres), and the plugin reads the same tables. So the
// only required setting is the manager's PostgreSQL DSN.
type Config struct {
	// ListenAddr is where the plugin's own dashboard is served.
	ListenAddr string
	// ManagerDSN is the manager's PostgreSQL connection string. The plugin opens
	// it READ-ONLY (see OpenStore), so it can never alter the manager's data.
	ManagerDSN string
	// RequestTimeout bounds a single query.
	RequestTimeout time.Duration
	// IncludeTest counts simulated ("test") sessions. Off by default, matching
	// the manager's own policy.
	IncludeTest bool
	// AuthToken, when set, is required (as a Bearer token or ?token=) to reach
	// the plugin's own API and dashboard. Empty leaves it open, which is fine on
	// 127.0.0.1.
	AuthToken string
}

// LoadConfig reads the configuration from the environment, applying defaults.
func LoadConfig() Config {
	cfg := Config{
		ListenAddr:     env("PLUGIN_LISTEN_ADDR", "127.0.0.1:8090"),
		ManagerDSN:     env("MANAGER_DATABASE_URL", ""),
		RequestTimeout: envDuration("QUERY_TIMEOUT", 15*time.Second),
		IncludeTest:    envBool("INCLUDE_TEST", false),
		AuthToken:      env("PLUGIN_AUTH_TOKEN", ""),
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 15 * time.Second
	}
	return cfg
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envDuration reads a duration expressed either as a Go duration ("10m", "30s")
// or as a plain number of seconds ("300").
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return time.Duration(f * float64(time.Second))
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
