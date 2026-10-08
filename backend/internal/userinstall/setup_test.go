package userinstall

import (
	"dcsmanager/internal/config"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationAndUpgradePreserveData(t *testing.T) {
	root, portable, sg, game := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	put(t, filepath.Join(game, "bin-mt", "DCS.exe"), "fixture")
	put(t, filepath.Join(portable, "data", "dcsmanager.db"), "original")
	put(t, filepath.Join(portable, "data", "profiles", "mapping.json"), "mapping")
	if err := os.Mkdir(filepath.Join(portable, "maps_dcs"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Configure(root, sg, portable, game); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{root, portable} {
		data, err := os.ReadFile(filepath.Join(base, "data", "dcsmanager.db"))
		if err != nil || string(data) != "original" {
			t.Fatalf("migration corrupted %s: %s %v", base, data, err)
		}
	}
	put(t, filepath.Join(root, "data", "dcsmanager.db"), "installed changes")
	if _, err := Configure(root, "", portable, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "data", "dcsmanager.db"))
	if string(data) != "installed changes" {
		t.Fatal("upgrade overwrote installed data")
	}
	data, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings config.UserSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.SavedGames != sg || settings.DCSInstall != game || settings.ChartsDir != filepath.Join(portable, "maps_dcs") {
		t.Fatalf("settings lost: %+v", settings)
	}
}

func TestInvalidConfigurationDoesNotImport(t *testing.T) {
	root, portable := t.TempDir(), t.TempDir()
	put(t, filepath.Join(root, "settings.json"), "invalid json")
	put(t, filepath.Join(portable, "data", "dcsmanager.db"), "original")
	if _, err := Configure(root, "", portable, ""); err == nil {
		t.Fatal("invalid settings accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatal("data imported before validation")
	}
}

func TestInvalidFoldersRejected(t *testing.T) {
	root := t.TempDir()
	if _, err := Configure(root, filepath.Join(root, "absent"), "", ""); err == nil {
		t.Fatal("invalid Saved Games accepted")
	}
	if _, err := Configure(root, "", "", t.TempDir()); err == nil {
		t.Fatal("invalid DCS install accepted")
	}
}
