package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledDefaultsAndEnvironmentPriority(t *testing.T) {
	for _, key := range []string{"DCSMANAGER_DB_PATH", "DCSMANAGER_CHARTS_DIR", "DCSMANAGER_CATEGORIES", "DCSMANAGER_SAVED_GAMES", "DCSMANAGER_DCS_INSTALL"} {
		t.Setenv(key, "")
	}
	program, root := t.TempDir(), t.TempDir()
	exe := filepath.Join(program, "dcsmanager.exe")
	cfg := Config{DBPath: "portable.db"}
	applyInstalledDefaults(&cfg, exe, root)
	if cfg.DBPath != "portable.db" {
		t.Fatal("portable defaults changed")
	}
	if err := os.WriteFile(filepath.Join(program, InstalledMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"savedGames":"custom-sg","dcsInstall":"custom-game","chartsDir":"custom-charts"}`), 0600); err != nil {
		t.Fatal(err)
	}
	applyInstalledDefaults(&cfg, exe, root)
	if cfg.DBPath != filepath.Join(root, "data", "dcsmanager.db") || cfg.SavedGames != "custom-sg" || cfg.DCSInstall != "custom-game" || cfg.ChartsDir != "custom-charts" {
		t.Fatalf("unexpected installed settings: %+v", cfg)
	}
	t.Setenv("DCSMANAGER_DB_PATH", "explicit.db")
	t.Setenv("DCSMANAGER_SAVED_GAMES", "explicit-sg")
	cfg.DBPath, cfg.SavedGames = "explicit.db", "explicit-sg"
	applyInstalledDefaults(&cfg, exe, root)
	if cfg.DBPath != "explicit.db" || cfg.SavedGames != "explicit-sg" {
		t.Fatal("environment overrides lost")
	}
}
