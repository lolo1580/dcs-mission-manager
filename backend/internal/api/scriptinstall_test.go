package api

import (
	"dcsmanager/internal/config"
	"dcsmanager/internal/install"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptAutoInstallerPreservesUserFilesAndRefreshesStatus(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	original := "-- Tacview\n-- DCS-BIOS\ndofile('other.lua')\n"
	export := filepath.Join(root, "Scripts", "Export.lua")
	os.WriteFile(export, []byte(original), 0644)
	os.MkdirAll(filepath.Join(root, "Config"), 0755)
	cfg := filepath.Join(root, "Config", "dcsmanager.cfg")
	settings := []byte("dcsmanager_host = '192.168.0.42'\n")
	os.WriteFile(cfg, settings, 0644)
	s := &Server{cfg: config.Config{SavedGames: root}, dcsRunning: func() (bool, error) { return false, nil }}
	invoke := func() map[string]any {
		t.Helper()
		r := httptest.NewRecorder()
		s.handleScriptInstall(r, httptest.NewRequest(http.MethodPost, "/api/scripts/install", nil))
		if r.Code != 200 {
			t.Fatalf("install: %d %s", r.Code, r.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	body := invoke()
	if body["restartRequired"] != true {
		t.Fatal("missing restart notice")
	}
	data, _ := os.ReadFile(export)
	if !strings.HasPrefix(string(data), original) || strings.Count(string(data), install.BeginMarker) != 1 {
		t.Fatal("other exports changed")
	}
	backups, _ := filepath.Glob(export + ".bak-*")
	if len(backups) != 1 {
		t.Fatal("missing backup")
	}
	before, _ := os.ReadFile(backups[0])
	if string(before) != original {
		t.Fatal("backup differs")
	}
	if got, _ := os.ReadFile(cfg); string(got) != string(settings) {
		t.Fatal("custom configuration overwritten")
	}
	if _, err := os.Stat(filepath.Join(root, "Scripts", "DCSManager", "PanelCommands.lua")); err != nil {
		t.Fatal(err)
	}
	invoke()
	backups, _ = filepath.Glob(export + ".bak-*")
	if len(backups) != 1 {
		t.Fatal("repeat installation not idempotent")
	}
	status := s.scriptStatusNow()
	if len(status.Managed) != 4 {
		t.Fatal("stale script status")
	}
	for _, st := range status.Managed {
		if st.State != "installed" {
			t.Fatalf("stale status: %#v", st)
		}
	}
}

func TestAutoInstallerBlocksRunningDCSMissingFolderAndGET(t *testing.T) {
	for _, tc := range []struct {
		method, root string
		running      bool
		code         int
	}{
		{http.MethodGet, t.TempDir(), false, 405},
		{http.MethodPost, "", false, 409},
		{http.MethodPost, filepath.Join(t.TempDir(), "absent"), false, 409},
		{http.MethodPost, t.TempDir(), true, 409},
	} {
		s := &Server{cfg: config.Config{SavedGames: tc.root}, dcsRunning: func() (bool, error) { return tc.running, nil }}
		r := httptest.NewRecorder()
		s.handleScriptInstall(r, httptest.NewRequest(tc.method, "/api/scripts/install", nil))
		if r.Code != tc.code {
			t.Fatalf("wanted %d: %d %s", tc.code, r.Code, r.Body.String())
		}
		if tc.root != "" {
			if _, err := os.Stat(filepath.Join(tc.root, "Scripts")); !os.IsNotExist(err) {
				t.Fatal("blocked request wrote files")
			}
		}
	}
}
