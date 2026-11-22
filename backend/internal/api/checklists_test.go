package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/checklist"
	"dcsmanager/internal/db"
)

func checklistBody(t *testing.T, c checklist.Checklist) string {
	t.Helper()
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestChecklistAPICRUD(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "checklists.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := &Server{db: database}

	c := checklist.Checklist{
		Name:  "Before start",
		Phase: "prestart",
		Items: []checklist.Item{{Text: "Battery", Response: "ON"}},
	}
	rec := httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodPost, "/api/checklists", strings.NewReader(checklistBody(t, c))))
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var saved checklist.Checklist
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.UID == "" || saved.Items[0].UID == "" {
		t.Fatalf("saved = %+v", saved)
	}

	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodGet, "/api/checklists", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Before start") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodGet, "/api/checklists/"+saved.UID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}

	saved.Name = "Renamed"
	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodPut, "/api/checklists/"+saved.UID, strings.NewReader(checklistBody(t, saved))))
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	var updated checklist.Checklist
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" || updated.Revision != 2 || updated.UID != saved.UID {
		t.Fatalf("updated = %+v", updated)
	}

	invalid := saved
	invalid.Phase = "aerobatics"
	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodPost, "/api/checklists", strings.NewReader(checklistBody(t, invalid))))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodDelete, "/api/checklists/"+saved.UID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodGet, "/api/checklists/"+saved.UID, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted get: %d %s", rec.Code, rec.Body.String())
	}
}

func TestChecklistAPIWithoutDatabaseReportsDisabled(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodGet, "/api/checklists", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChecklistUnknownUIDReturns404(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "unknown-checklist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := &Server{db: database}
	rec := httptest.NewRecorder()
	s.handleChecklists(rec, httptest.NewRequest(http.MethodGet, "/api/checklists/cl_missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
