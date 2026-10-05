package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the plugin's runtime configuration, read from environment
// variables. Nothing here is required: the defaults assume Postgres and the
// manager both run on the local machine.
type Config struct {
	// ListenAddr is where the plugin's own dashboard is served.
	ListenAddr string
	// ManagerURL is the base URL of the dcsmanager REST API.
	ManagerURL string
	// DatabaseURL is the PostgreSQL connection string (pgx/pgxpool format).
	DatabaseURL string
	// SyncInterval is how often the API is polled.
	SyncInterval time.Duration
	// RequestTimeout bounds a single API call.
	RequestTimeout time.Duration
	// IncludeTest asks the manager to include simulated ("test") sessions.
	// Off by default, matching the manager's own policy.
	IncludeTest bool
	// Scopes are the stat scopes to snapshot: "career" and/or "mission".
	Scopes []string
	// Mirror enables the incremental mirror of events and chat, using the
	// manager's ?sinceId= endpoints.
	Mirror bool
	// MirrorBatch caps how many rows are fetched per incremental request.
	MirrorBatch int
	// Name identifies this manager instance in the shared database, so several
	// managers can feed the same plugin. Defaults to "local".
	Name string
	// AuthToken, when set, is required (as a Bearer token or ?token=) to reach
	// the plugin's own API and dashboard. Empty leaves it open, which is fine
	// on 127.0.0.1.
	AuthToken string
}

// LoadConfig reads the configuration from the environment, applying defaults.
func LoadConfig() Config {
	cfg := Config{
		ListenAddr:     env("PLUGIN_LISTEN_ADDR", ":8090"),
		ManagerURL:     strings.TrimRight(env("DCSMANAGER_URL", "http://127.0.0.1:8080"), "/"),
		DatabaseURL:    env("DATABASE_URL", "postgres://dcs:dcs@localhost:5432/stats?sslmode=disable"),
		SyncInterval:   envDuration("SYNC_INTERVAL", 5*time.Minute),
		RequestTimeout: envDuration("SYNC_TIMEOUT", 10*time.Second),
		IncludeTest:    envBool("INCLUDE_TEST", false),
		Scopes:         splitList(env("SYNC_SCOPES", "career")),
		Mirror:         envBool("MIRROR_EVENTS", true),
		MirrorBatch:    envInt("MIRROR_BATCH", 1000),
		Name:           env("MANAGER_NAME", "local"),
		AuthToken:      env("PLUGIN_AUTH_TOKEN", ""),
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"career"}
	}
	if cfg.MirrorBatch <= 0 || cfg.MirrorBatch > 5000 {
		cfg.MirrorBatch = 1000
	}
	if cfg.Name == "" {
		cfg.Name = "local"
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

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
