package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
)

func TestHealthReportsDatabaseFailure(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "dcsmanager.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := New(config.Config{Mode: "server"}, nil, nil, database, nil, nil, nil, nil)
	srv.SetMode("server")
	srv.SetAuth(database)

	rec := httptest.NewRecorder()
	srv.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"db":true`) {
		t.Fatalf("health = %d %s", rec.Code, rec.Body.String())
	}

	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	srv.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Fatalf("closed health = %d %s", rec.Code, rec.Body.String())
	}
}
