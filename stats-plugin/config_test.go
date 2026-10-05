package main

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	// t.Setenv clears the variable afterwards; set to empty so the defaults win.
	for _, k := range []string{"DATABASE_URL", "DCSMANAGER_URL", "PLUGIN_LISTEN_ADDR", "SYNC_INTERVAL", "SYNC_SCOPES"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfig()
	if cfg.ManagerURL != "http://127.0.0.1:8080" {
		t.Errorf("ManagerURL = %q", cfg.ManagerURL)
	}
	if cfg.ListenAddr != ":8090" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.SyncInterval != 5*time.Minute {
		t.Errorf("SyncInterval = %s", cfg.SyncInterval)
	}
	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "career" {
		t.Errorf("Scopes = %v", cfg.Scopes)
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

func TestSplitList(t *testing.T) {
	got := splitList(" career , mission ,, ")
	if len(got) != 2 || got[0] != "career" || got[1] != "mission" {
		t.Errorf("splitList = %v", got)
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
