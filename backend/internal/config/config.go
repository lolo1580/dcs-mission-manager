// Package config loads backend configuration from DCSMANAGER_* environment variables.
//
// The manager is local-only: it runs on the same Windows machine as DCS. That is
// a deliberate constraint, not a limitation to work around — it is what lets the
// backend read DCS's own terrain files (airfields, beacons) directly, and it
// removes every networking pitfall (LAN address, firewall, container ports).
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration of the backend.
type Config struct {
	// HTTPAddr is the listen address of the web UI (HTTP + SSE). It defaults to
	// the loopback interface: the manager is local, and the API includes a
	// destructive purge endpoint with no authentication. Set
	// DCSMANAGER_HTTP_ADDR=0.0.0.0:8080 deliberately to reach the UI from another
	// device (a tablet in the cockpit, for instance).
	HTTPAddr string
	// UDPAddr is the listen address for telemetry coming from DCS.
	//
	// 7776, not 7778: DCS-BIOS — which many cockpits run alongside this manager —
	// owns 7778 for its command channel. Binding it too would make one of the two
	// fail to start, and DCS-BIOS' port is the established one.
	UDPAddr string
	// TCPAddr is the listen address for events and downstream commands (Phase 2+).
	TCPAddr string
	// DBPath is the path of the SQLite database.
	DBPath string
	// DBEnabled toggles persistence (SQLite). Disabled keeps everything in memory.
	DBEnabled bool
	// DBDriver selects the persistence engine: "sqlite" (default) or "postgres".
	// The change is read at startup and requires a restart.
	DBDriver string
	// DBDSN is the PostgreSQL connection string, used when DBDriver is
	// "postgres". SQLite keeps using DBPath.
	DBDSN string
	// Theatre is the default DCS theatre.
	Theatre string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// UnitTTL is how long a unit is kept after its last update.
	UnitTTL time.Duration
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
	// SavedGames is the DCS Saved Games folder, when it could be located. It is
	// used to find the installation (through Logs\dcs.log) and to read mission
	// and debrief data. Empty means "not found".
	SavedGames string
	// ChartsDir is the folder holding aeronautical chart scans (approach plates,
	// ground plans). They are documents, never shipped: only indexed and
	// displayed. Empty disables the feature.
	ChartsDir string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// savedGamesDir locates the DCS Saved Games folder, honouring an explicit
// DCSMANAGER_SAVED_GAMES override. It is best-effort: an empty result is normal when
// DCS is not installed, and callers must cope with it rather than fail.
func savedGamesDir() string {
	if v := os.Getenv("DCSMANAGER_SAVED_GAMES"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	base := filepath.Join(home, "Saved Games")
	// OpenBeta first: a machine usually has only one, and the beta is the one
	// that is actively used.
	for _, name := range []string{"DCS.openbeta", "DCS"} {
		candidate := filepath.Join(base, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
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
		HTTPAddr:       env("DCSMANAGER_HTTP_ADDR", "127.0.0.1:8080"),
		UDPAddr:        env("DCSMANAGER_UDP_ADDR", "127.0.0.1:7776"),
		TCPAddr:        env("DCSMANAGER_TCP_ADDR", "127.0.0.1:7779"),
		DBPath:         env("DCSMANAGER_DB_PATH", "./data/dcsmanager.db"),
		DBEnabled:      envBool("DCSMANAGER_DB_ENABLED", true),
		DBDriver:       strings.ToLower(env("DCSMANAGER_DB_DRIVER", "sqlite")),
		DBDSN:          env("DCSMANAGER_DB_DSN", ""),
		Theatre:        env("DCSMANAGER_THEATRE", "Caucasus"),
		LogLevel:       strings.ToLower(env("DCSMANAGER_LOG_LEVEL", "info")),
		UnitTTL:        envDuration("DCSMANAGER_UNIT_TTL", 5*time.Second),
		CategoriesFile: env("DCSMANAGER_CATEGORIES", "./categories.json"),
		MaxUnits:       envInt("DCSMANAGER_MAX_UNITS", 5000),
		TrackInterval:  envDuration("DCSMANAGER_TRACK_INTERVAL", 3*time.Second),
		TrackGrace:     envDuration("DCSMANAGER_TRACK_GRACE", 15*time.Second),
		TrackRetention: envDuration("DCSMANAGER_TRACK_RETENTION", 24*time.Hour),
		SavedGames:     savedGamesDir(),
		ChartsDir:      env("DCSMANAGER_CHARTS_DIR", "./maps_dcs"),
	}
}
