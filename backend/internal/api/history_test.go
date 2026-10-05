package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"dcsmanager/internal/db"
	"dcsmanager/internal/model"
)

func serverWithDB(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &Server{db: database, localOnly: true}, database
}

func seedEvents(t *testing.T, database *db.DB, n int) {
	t.Helper()
	id, err := database.EnsureMissionTagged("Flight", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatalf("mission: %v", err)
	}
	for i := 0; i < n; i++ {
		if err := database.SaveEvent(id, model.Event{Event: "kill", Args: []any{i}, RealTS: int64(i)}); err != nil {
			t.Fatalf("save event: %v", err)
		}
	}
}

type eventsResponse struct {
	Count       int           `json:"count"`
	Events      []model.Event `json:"events"`
	NextSinceID int64         `json:"nextSinceId"`
}

// TestHistoryEventsIncremental locks the cursor contract: sinceId returns only
// newer events, oldest first, and reports the next cursor.
func TestHistoryEventsIncremental(t *testing.T) {
	s, database := serverWithDB(t)
	seedEvents(t, database, 3)

	// First pass: everything from 0.
	rec := httptest.NewRecorder()
	s.handleHistoryEvents(rec, httptest.NewRequest(http.MethodGet, "/api/history/events?sinceId=0", nil))
	var first eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if first.Count != 3 {
		t.Fatalf("first pass count = %d, want 3", first.Count)
	}
	if first.NextSinceID != first.Events[len(first.Events)-1].ID {
		t.Fatalf("nextSinceId = %d, want last id %d", first.NextSinceID, first.Events[len(first.Events)-1].ID)
	}
	// Oldest first.
	if first.Events[0].ID >= first.Events[len(first.Events)-1].ID {
		t.Fatalf("events not ordered oldest first: %+v", first.Events)
	}

	// Second pass: only newer events, none here.
	cursor := first.NextSinceID
	rec = httptest.NewRecorder()
	s.handleHistoryEvents(rec, httptest.NewRequest(http.MethodGet, "/api/history/events?sinceId=2", nil))
	var second eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second.Count != 1 {
		t.Fatalf("events after id 2 = %d, want 1", second.Count)
	}
	if second.Events[0].ID <= 2 {
		t.Fatalf("returned an event not newer than the cursor: %+v", second.Events)
	}
	_ = cursor

	// A stale cursor (>= max) yields an empty batch and a 0 next cursor.
	rec = httptest.NewRecorder()
	s.handleHistoryEvents(rec, httptest.NewRequest(http.MethodGet, "/api/history/events?sinceId=999", nil))
	var empty eventsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &empty)
	if empty.Count != 0 || empty.NextSinceID != 0 {
		t.Fatalf("stale cursor = %+v, want empty with nextSinceId 0", empty)
	}
}

// TestHistoryEventsWithoutSinceKeepsRecentBehaviour ensures the change is
// backward compatible: no sinceId still means "the most recent, newest first".
func TestHistoryEventsWithoutSinceKeepsRecentBehaviour(t *testing.T) {
	s, database := serverWithDB(t)
	seedEvents(t, database, 3)

	rec := httptest.NewRecorder()
	s.handleHistoryEvents(rec, httptest.NewRequest(http.MethodGet, "/api/history/events", nil))
	var got eventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 3 {
		t.Fatalf("count = %d, want 3", got.Count)
	}
	// Newest first in the legacy shape.
	if got.Events[0].ID <= got.Events[len(got.Events)-1].ID {
		t.Fatalf("legacy shape should be newest first: %+v", got.Events)
	}
}

// TestHistoryEventsBadSinceIsTolerant: a non-numeric sinceId must not 500 or
// drop data; it falls back to 0 (the whole history).
func TestHistoryEventsBadSinceIsTolerant(t *testing.T) {
	s, database := serverWithDB(t)
	seedEvents(t, database, 2)

	rec := httptest.NewRecorder()
	s.handleHistoryEvents(rec, httptest.NewRequest(http.MethodGet, "/api/history/events?sinceId=abc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("bad sinceId should still be 200, got %d", rec.Code)
	}
	var got eventsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Count != 2 {
		t.Fatalf("bad sinceId should return the whole history, got %d", got.Count)
	}
}

type missionsResponse struct {
	Count       int             `json:"count"`
	Missions    []model.Mission `json:"missions"`
	NextSinceID int64           `json:"nextSinceId"`
}

// TestHistoryMissionsIncremental locks the cursor contract for missions.
func TestHistoryMissionsIncremental(t *testing.T) {
	s, database := serverWithDB(t)
	for i := 0; i < 3; i++ {
		if _, err := database.StartMission("M"+strconv.Itoa(i), "Caucasus", db.SourceLive); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	s.handleHistoryMissions(rec, httptest.NewRequest(http.MethodGet, "/api/history/missions?sinceId=0", nil))
	var first missionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if first.Count != 3 {
		t.Fatalf("count = %d, want 3", first.Count)
	}
	if first.NextSinceID != first.Missions[len(first.Missions)-1].ID {
		t.Fatalf("nextSinceId = %d, want last id", first.NextSinceID)
	}

	// Incremental: only the newer mission.
	rec = httptest.NewRecorder()
	s.handleHistoryMissions(rec, httptest.NewRequest(http.MethodGet, "/api/history/missions?sinceId=1", nil))
	var second missionsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second.Count != 2 {
		t.Fatalf("missions after id 1 = %d, want 2", second.Count)
	}
}
