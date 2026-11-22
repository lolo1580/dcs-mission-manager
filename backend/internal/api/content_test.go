package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/config"
	"dcsmanager/internal/content"
)

func contentTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	sg := t.TempDir()
	cfg := config.Config{Mode: "desktop", SavedGames: sg, DBPath: filepath.Join(t.TempDir(), "dcsmanager.db")}
	srv := New(cfg, nil, nil, nil, nil, nil, nil, nil)
	svc := content.New(sg, t.TempDir())
	svc.SetDCSRunning(func() (bool, error) { return false, nil })
	srv.SetContent(svc)
	return srv, sg
}

func TestContentHandlers(t *testing.T) {
	srv, sg := contentTestServer(t)
	if err := os.MkdirAll(filepath.Join(sg, "Mods", "aircraft", "A"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sg, "Mods", "aircraft", "A", "entry.lua"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	srv.handleContentStatus(rec, httptest.NewRequest(http.MethodGet, "/api/content/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "modsDir") {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.handleContentMods(rec, httptest.NewRequest(http.MethodGet, "/api/content/mods", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"A"`) {
		t.Fatalf("mods = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"action":"disable","category":"aircraft","name":"A"}`)
	srv.handleContentModAction(rec, httptest.NewRequest(http.MethodPost, "/api/content/mods/action", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("action = %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods.disabled", "aircraft", "A", "entry.lua")); err != nil {
		t.Fatal("mod was not disabled", err)
	}
}

func TestContentHandlersRequireService(t *testing.T) {
	srv := New(config.Config{Mode: "desktop"}, nil, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	srv.handleContentMods(rec, httptest.NewRequest(http.MethodGet, "/api/content/mods", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestContentImportInspectWithoutFile(t *testing.T) {
	srv, _ := contentTestServer(t)
	rec := httptest.NewRecorder()
	srv.handleContentImportInspect(rec, httptest.NewRequest(http.MethodPost, "/api/content/import/inspect", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
}

func TestContentThumbnail(t *testing.T) {
	srv, sg := contentTestServer(t)
	livery := filepath.Join(sg, "Liveries", "F-5E-3", "Test")
	if err := os.MkdirAll(livery, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(livery, "description.lua"), []byte(`name = "Test"`), 0o644); err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if err := os.WriteFile(filepath.Join(livery, "thumbnail.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	scan, err := srv.content.Liveries()
	if err != nil || len(scan.Liveries) != 1 {
		t.Fatalf("scan = %+v, %v", scan, err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/content/liveries/thumbnail?id="+url.QueryEscape(scan.Liveries[0].ID), nil)
	rec := httptest.NewRecorder()
	srv.handleContentThumbnail(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() != len(png) {
		t.Fatalf("thumbnail = %d %d bytes", rec.Code, rec.Body.Len())
	}
}
