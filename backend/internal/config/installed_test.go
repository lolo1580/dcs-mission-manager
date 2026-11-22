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
	if cfg.DBPath != filepath.Join(root, "data", DatabaseFile) || cfg.SavedGames != "custom-sg" || cfg.DCSInstall != "custom-game" || cfg.ChartsDir != "custom-charts" {
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

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyDataCopiesDatabaseAndSettings(t *testing.T) {
	legacy, root := t.TempDir(), t.TempDir()
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile), "database")
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile+"-wal"), "wal")
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile+"-shm"), "shm")
	put(t, filepath.Join(legacy, "data", "rejected", "broken.log"), "log")
	put(t, filepath.Join(legacy, "data", "mappings.json"), "panel bindings")
	put(t, filepath.Join(legacy, "data", "panel-settings.json"), "panel switches")
	put(t, filepath.Join(legacy, "data", "profiles", "FA-18C.json"), "panel profile")
	put(t, filepath.Join(legacy, "settings.json"), `{"dcsInstall":"legacy-game","chartsDir":"legacy-charts"}`)
	put(t, filepath.Join(legacy, "categories.json"), "{}")
	// The setup wizard may already have written its own settings before the
	// first launch; those paths win, and only the empty fields are filled.
	put(t, filepath.Join(root, "settings.json"), `{"savedGames":"wizard-sg"}`)

	if err := migrateLegacyData(legacy, root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "data", DatabaseFile))
	if err != nil || string(data) != "database" {
		t.Fatalf("database not migrated: %q %v", data, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		got, err := os.ReadFile(filepath.Join(root, "data", DatabaseFile+suffix))
		if err != nil || string(got) != suffix[1:] {
			t.Fatalf("%s not migrated: %q %v", suffix, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "data", LegacyDatabaseFile)); !os.IsNotExist(err) {
		t.Fatal("legacy database name left behind")
	}
	if got, err := os.ReadFile(filepath.Join(root, "data", "rejected", "broken.log")); err != nil || string(got) != "log" {
		t.Fatal("nested data not migrated")
	}
	for _, panelArtifact := range []string{"mappings.json", "panel-settings.json", filepath.Join("profiles", "FA-18C.json")} {
		if _, err := os.Stat(filepath.Join(root, "data", panelArtifact)); !os.IsNotExist(err) {
			t.Fatalf("panel artifact migrated: %s", panelArtifact)
		}
	}
	settings, err := ReadUserSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings.SavedGames != "wizard-sg" || settings.DCSInstall != "legacy-game" || settings.ChartsDir != "legacy-charts" {
		t.Fatalf("settings not merged: %+v", settings)
	}
	if _, err := os.Stat(filepath.Join(root, "categories.json")); err != nil {
		t.Fatal("categories not migrated")
	}
	if _, err := os.Stat(filepath.Join(root, "migrated-from-flight-sim-manager.txt")); err != nil {
		t.Fatal("migration note missing")
	}
	// The original folder is evidence and must survive untouched.
	if got, err := os.ReadFile(filepath.Join(legacy, "data", LegacyDatabaseFile)); err != nil || string(got) != "database" {
		t.Fatal("legacy folder modified")
	}
}

func TestMigrateLegacyDataIgnoresAnExistingLogFile(t *testing.T) {
	legacy, root := t.TempDir(), t.TempDir()
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile), "legacy")
	put(t, filepath.Join(root, "data", "dcsmanager.log"), "startup")

	if err := migrateLegacyData(legacy, root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "data", DatabaseFile))
	if err != nil || string(got) != "legacy" {
		t.Fatalf("log file suppressed the migration: %q %v", got, err)
	}
}

func TestMigrateLegacyDataKeepsExistingDestination(t *testing.T) {
	legacy, root := t.TempDir(), t.TempDir()
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile), "legacy")
	put(t, filepath.Join(root, "data", DatabaseFile), "current")

	if err := migrateLegacyData(legacy, root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "data", DatabaseFile))
	if string(got) != "current" {
		t.Fatal("existing data overwritten")
	}
	if _, err := os.Stat(filepath.Join(root, "data", LegacyDatabaseFile)); !os.IsNotExist(err) {
		t.Fatal("legacy file imported into a populated folder")
	}
}

func TestMigrateLegacyDataIsIdempotent(t *testing.T) {
	legacy, root := t.TempDir(), t.TempDir()
	put(t, filepath.Join(legacy, "data", LegacyDatabaseFile), "database")
	if err := migrateLegacyData(legacy, root); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "data", DatabaseFile), "changed")
	if err := migrateLegacyData(legacy, root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "data", DatabaseFile))
	if string(got) != "changed" {
		t.Fatal("second migration overwrote the live database")
	}
}

func TestMigrateLegacyDataWithoutSourceIsNoOp(t *testing.T) {
	root := t.TempDir()
	if err := migrateLegacyData(filepath.Join(t.TempDir(), "absent"), root); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("no-op created files: %v %v", entries, err)
	}
}
