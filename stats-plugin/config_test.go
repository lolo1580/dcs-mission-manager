package main

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range []string{"MANAGER_DATABASE_URL", "PLUGIN_LISTEN_ADDR", "QUERY_TIMEOUT", "PLUGIN_AUTH_TOKEN"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfig()
	if cfg.ListenAddr != ":8090" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.ManagerDSN != "" {
		t.Errorf("ManagerDSN = %q, want empty", cfg.ManagerDSN)
	}
	if cfg.RequestTimeout != 15*time.Second {
		t.Errorf("RequestTimeout = %s", cfg.RequestTimeout)
	}
	if cfg.IncludeTest {
		t.Error("IncludeTest should default to false")
	}
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("X", "10m")
	if got := envDuration("X", time.Second); got != 10*time.Minute {
		t.Errorf("10m -> %s", got)
	}
	t.Setenv("X", "300")
	if got := envDuration("X", time.Second); got != 300*time.Second {
		t.Errorf("300 -> %s", got)
	}
	t.Setenv("X", "garbage")
	if got := envDuration("X", 7*time.Second); got != 7*time.Second {
		t.Errorf("garbage should fall back, got %s", got)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("B", "on")
	if !envBool("B", false) {
		t.Error("on should be true")
	}
	t.Setenv("B", "0")
	if envBool("B", true) {
		t.Error("0 should be false")
	}
}
