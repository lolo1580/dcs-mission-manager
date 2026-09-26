// Package config loads backend configuration from DCSMM_* environment variables.
//
// The exact same configuration mechanism is used by the Windows .exe and by the
// Docker image, so both deployments behave identically.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration of the backend.
type Config struct {
	// HTTPAddr is the listen address of the web UI (HTTP + SSE).
	HTTPAddr string
	// UDPAddr is the listen address for telemetry coming from DCS.
	UDPAddr string
	// TCPAddr is the listen address for events and downstream commands (Phase 2+).
	TCPAddr string
	// DBPath is the path of the SQLite database.
	DBPath string
	// DBEnabled toggles persistence (SQLite). Disabled keeps everything in memory.
	DBEnabled bool
	// Theatre is the default DCS theatre.
	Theatre string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// UnitTTL is how long a unit is kept after its last update.
	UnitTTL time.Duration
	// TilesDir is the directory holding DCS map tiles (per theatre).
	TilesDir string
	// Basemap is the default basemap id (satellite, topo, osm, dark).
	Basemap string
	// BasemapURL is an optional custom basemap tile template ({z}/{x}/{y}).
	BasemapURL string
	// CategoriesFile is an optional JSON file overriding unit type categories.
	CategoriesFile string
	// MaxUnits caps how many units are kept in the store.
	MaxUnits int
	// TrackInterval is how often unit positions are sampled for history.
	TrackInterval time.Duration
	// TrackGrace is how long a unit must be missing before being counted as lost.
	TrackGrace time.Duration
	// TrackRetention is how long tracking history is kept.
	TrackRetention time.Duration
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
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

// envDuration reads a duration expressed in seconds (integer or decimal).
func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(f * float64(time.Second))
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

// Load reads the configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		HTTPAddr:       env("DCSMM_HTTP_ADDR", "0.0.0.0:8080"),
		UDPAddr:        env("DCSMM_UDP_ADDR", "127.0.0.1:7778"),
		TCPAddr:        env("DCSMM_TCP_ADDR", "127.0.0.1:7779"),
		DBPath:         env("DCSMM_DB_PATH", "./data/dcsmm.db"),
		DBEnabled:      envBool("DCSMM_DB_ENABLED", true),
		Theatre:        env("DCSMM_THEATRE", "Caucasus"),
		LogLevel:       strings.ToLower(env("DCSMM_LOG_LEVEL", "info")),
		UnitTTL:        envDuration("DCSMM_UNIT_TTL", 5*time.Second),
		TilesDir:       env("DCSMM_TILES_DIR", "./tiles"),
		Basemap:        strings.ToLower(env("DCSMM_BASEMAP", "satellite")),
		BasemapURL:     env("DCSMM_BASEMAP_URL", ""),
		CategoriesFile: env("DCSMM_CATEGORIES", "./categories.json"),
		MaxUnits:       envInt("DCSMM_MAX_UNITS", 5000),
		TrackInterval:  envDuration("DCSMM_TRACK_INTERVAL", 3*time.Second),
		TrackGrace:     envDuration("DCSMM_TRACK_GRACE", 15*time.Second),
		TrackRetention: envDuration("DCSMM_TRACK_RETENTION", 24*time.Hour),
	}
}
