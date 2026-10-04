package tracker

import (
	"path/filepath"
	"testing"
	"time"

	"dcsmanager/internal/db"
	"dcsmanager/internal/state"
)

// TestClosedMissionIsReleased locks the fix: once the mission the tracker recorded
// is ended, the tracker must stop attaching samples to it and follow the new one.
// It used to keep a stale id and record a new flight into the closed mission.
func TestClosedMissionIsReleased(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := state.New(time.Minute, 0)
	tr := New(database, store, Options{SampleEvery: time.Second, Grace: time.Millisecond})

	first, err := database.StartMission("Mission 1", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	tr.SetMissionID(first)

	// The first mission ends and a second begins.
	if err := database.EndOpenMission(""); err != nil {
		t.Fatal(err)
	}
	second, err := database.StartMission("Mission 2", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	// The tracker follows the open mission through the callback.
	tr.SetMissionIDFunc(database.OpenMissionID)

	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane",
		Coalition: "blue", Lat: 42, Lng: 41, Alt: 5000})
	tr.Tick()

	// The sample must belong to the second mission, not the closed first one.
	var n int
	if err := database.SQL().QueryRow(
		`SELECT COUNT(*) FROM track_positions WHERE mission_id = ?`, second).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("samples should belong to the new mission %d", second)
	}
	if err := database.SQL().QueryRow(
		`SELECT COUNT(*) FROM track_positions WHERE mission_id = ?`, first).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the closed mission %d must receive no sample, got %d", first, n)
	}
}
