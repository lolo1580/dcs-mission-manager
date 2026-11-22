package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dcsmanager/internal/config"
)

func mappingsTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	sg := t.TempDir()
	input := filepath.Join(sg, "Config", "Input", "F-16C_50", "keyboard")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "Keyboard.diff.lua"), []byte("return"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Mode:       "desktop",
		SavedGames: sg,
		DBPath:     filepath.Join(t.TempDir(), "dcsmanager.db"),
	}
	srv := New(cfg, nil, nil, nil, nil, nil, nil, nil)
	srv.dcsRunning = func() (bool, error) { return false, nil }
	return srv, sg
}

func TestBackupMappings(t *testing.T) {
	srv, _ := mappingsTestServer(t)

	rec := httptest.NewRecorder()
	srv.handleBackupMappings(rec, httptest.NewRequest(http.MethodGet, "/api/backup/mappings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
	}
	var getBody struct {
		Mappings struct {
			Files    int `json:"files"`
			Aircraft []struct {
				Name string `json:"name"`
			} `json:"aircraft"`
		} `json:"mappings"`
		Archives []struct {
			Name string `json:"name"`
		} `json:"archives"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.Mappings.Files != 1 || len(getBody.Mappings.Aircraft) != 1 {
		t.Fatalf("mappings = %+v", getBody.Mappings)
	}

	rec = httptest.NewRecorder()
	srv.handleBackupMappings(rec, httptest.NewRequest(http.MethodPost, "/api/backup/mappings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Backup struct {
			Name       string   `json:"name"`
			Categories []string `json:"categories"`
		} `json:"backup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Backup.Name == "" || len(created.Backup.Categories) != 1 || created.Backup.Categories[0] != "input" {
		t.Fatalf("backup = %+v", created.Backup)
	}

	// A selected subset is recorded in the manifest and the archive name.
	body, _ := json.Marshal(map[string]any{"aircraft": []string{"F-16C_50"}})
	rec = httptest.NewRecorder()
	srv.handleBackupMappings(rec, httptest.NewRequest(http.MethodPost, "/api/backup/mappings", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST selected = %d %s", rec.Code, rec.Body.String())
	}
	var selected struct {
		Backup struct {
			Name     string   `json:"name"`
			Aircraft []string `json:"aircraft"`
		} `json:"backup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected.Backup.Aircraft) != 1 || selected.Backup.Aircraft[0] != "F-16C_50" {
		t.Fatalf("selected backup = %+v", selected.Backup)
	}

	body, _ = json.Marshal(map[string]any{"name": created.Backup.Name, "dryRun": true})
	rec = httptest.NewRecorder()
	srv.handleBackupMappingsRestore(rec, httptest.NewRequest(http.MethodPost, "/api/backup/mappings/restore", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("restore dry run = %d %s", rec.Code, rec.Body.String())
	}

	srv.dcsRunning = func() (bool, error) { return true, nil }
	// A dry run stays available while DCS is open; it does not write anything.
	rec = httptest.NewRecorder()
	srv.handleBackupMappingsRestore(rec, httptest.NewRequest(http.MethodPost, "/api/backup/mappings/restore", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry run with DCS running = %d %s", rec.Code, rec.Body.String())
	}
	// A real restore is refused.
	body, _ = json.Marshal(map[string]any{"name": created.Backup.Name, "dryRun": false})
	rec = httptest.NewRecorder()
	srv.handleBackupMappingsRestore(rec, httptest.NewRequest(http.MethodPost, "/api/backup/mappings/restore", bytes.NewReader(body)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("restore with DCS running = %d %s", rec.Code, rec.Body.String())
	}

	// The generic restore endpoint also refuses to overwrite mappings while DCS
	// is open, but its dry run stays available.
	body, _ = json.Marshal(map[string]any{"name": created.Backup.Name, "dryRun": false})
	rec = httptest.NewRecorder()
	srv.handleBackupRestore(rec, httptest.NewRequest(http.MethodPost, "/api/backup/restore", bytes.NewReader(body)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("generic restore with DCS running = %d %s", rec.Code, rec.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"name": created.Backup.Name, "dryRun": true})
	rec = httptest.NewRecorder()
	srv.handleBackupRestore(rec, httptest.NewRequest(http.MethodPost, "/api/backup/restore", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("generic dry run with DCS running = %d %s", rec.Code, rec.Body.String())
	}
}
