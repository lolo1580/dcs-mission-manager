// Package config loads backend configuration from DCSMM_* environment variables.
//
// The exact same configuration mechanism is used by the Windows .exe and by the
// Docker image, so both deployments behave identically.
package config

import (
	"os"
	"strings"
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
	// Theatre is the default DCS theatre used for map projection.
	Theatre string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads the configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		HTTPAddr: env("DCSMM_HTTP_ADDR", "0.0.0.0:8080"),
		UDPAddr:  env("DCSMM_UDP_ADDR", "127.0.0.1:7778"),
		TCPAddr:  env("DCSMM_TCP_ADDR", "127.0.0.1:7779"),
		DBPath:   env("DCSMM_DB_PATH", "./data/dcsmm.db"),
		Theatre:  env("DCSMM_THEATRE", "Caucasus"),
		LogLevel: strings.ToLower(env("DCSMM_LOG_LEVEL", "info")),
	}
}
