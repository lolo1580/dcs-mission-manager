package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dcsmanager/internal/db"
)

// TestPurgeRejectsAmbiguousScope locks the safeguard: a request naming more than
// one scope must be refused, never silently resolved to the most destructive one.
// `all=1&source=test` used to delete every live mission because `all` won.
func TestPurgeRejectsAmbiguousScope(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// A real mission exists and must survive the refused request.
	id, err := database.EnsureMissionTagged("Real flight", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatalf("ensure mission: %v", err)
	}

	s := &Server{db: database, localOnly: true}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/purge?all=1&source=test", nil)
	s.handlePurge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ambiguous purge should be 400, got %d", rec.Code)
	}
	// The real mission is still there.
	missions, err := database.MissionsWithSource(db.SourceLive, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(missions) != 1 || missions[0].ID != id {
		t.Fatalf("the live mission must not have been purged, got %+v", missions)
	}
}

// TestPurgeSingleScopeStillWorks checks the guard does not break the valid,
// single-scope requests.
func TestPurgeSingleScopeStillWorks(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.EnsureMissionTagged("Real", "Caucasus", db.SourceLive); err != nil {
		t.Fatal(err)
	}
	// Close it so the next open gets its own row, then tag the test session.
	if err := database.EndOpenMission(""); err != nil {
		t.Fatal(err)
	}
	if _, err := database.EnsureMissionTagged("Fake", "Caucasus", db.SourceTest); err != nil {
		t.Fatal(err)
	}

	s := &Server{db: database, localOnly: true}
	rec := httptest.NewRecorder()
	s.handlePurge(rec, httptest.NewRequest(http.MethodDelete, "/api/maintenance/purge?source=test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("single-scope purge should succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	live, _ := database.CountMissions(db.SourceLive)
	test, _ := database.CountMissions(db.SourceTest)
	if live != 1 || test != 0 {
		t.Fatalf("live=%d test=%d, want live=1 test=0", live, test)
	}
}
