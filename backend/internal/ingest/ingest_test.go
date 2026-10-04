package ingest

import (
	"path/filepath"
	"testing"

	"dcsmanager/internal/db"
	"dcsmanager/internal/live"
	"dcsmanager/internal/model"
)

func newWriter(t *testing.T) (*Writer, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database, live.New(100, 100)), database
}

// TestEventAfterPurgeIsRecorded locks the fix: a purge deletes the mission the
// writer remembered, and the next event used to fail on the foreign key and be
// lost. The writer must notice the mission is gone and open a fresh one.
func TestEventAfterPurgeIsRecorded(t *testing.T) {
	w, database := newWriter(t)

	w.Handle(model.Message{Type: "mission", Phase: "start", Name: "Live", Theatre: "Caucasus"})
	if _, err := database.PurgeAll(); err != nil {
		t.Fatalf("purge: %v", err)
	}

	w.Handle(model.Message{Type: "event", Event: "takeoff", Args: []any{"Pilot"}, T: 1})

	var n int
	if err := database.SQL().QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("the event after a purge must be recorded, got %d", n)
	}
	if database.OpenMissionID() == 0 {
		t.Fatal("a mission should have been reopened")
	}
}

// TestNewMissionStartClosesPrevious locks the transition: a start while a previous
// mission is still open must open a new mission, not merge into the stale one.
func TestNewMissionStartClosesPrevious(t *testing.T) {
	w, database := newWriter(t)

	w.Handle(model.Message{Type: "mission", Phase: "start", Name: "Previous", Theatre: "Caucasus"})
	w.Handle(model.Message{Type: "mission", Phase: "start", Name: "New flight", Theatre: "Syria"})

	var name, theatre string
	if err := database.SQL().QueryRow(
		`SELECT name, COALESCE(theatre,'') FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`,
	).Scan(&name, &theatre); err != nil {
		t.Fatal(err)
	}
	if name != "New flight" || theatre != "Syria" {
		t.Fatalf("open mission = %q/%q, want the new flight on Syria", name, theatre)
	}
	var open int
	if err := database.SQL().QueryRow(`SELECT COUNT(*) FROM missions WHERE ended_at IS NULL`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 1 {
		t.Fatalf("exactly one mission should be open, got %d", open)
	}
}
