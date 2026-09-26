package tracker

import (
	"path/filepath"
	"testing"
	"time"

	"dcsmm/internal/db"
	"dcsmm/internal/state"
)

// TestIdleTickCreatesNoMission guards against phantom sessions: an idle backend
// must not create a "mission" at every start. Without this, the UI shows an
// empty session with no data alongside the real ones.
func TestIdleTickCreatesNoMission(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := state.New(time.Minute, 0)
	tr := New(database, store, Options{SampleEvery: time.Second, Grace: time.Millisecond})

	// Several idle ticks, exactly as an idle server would produce.
	for i := 0; i < 3; i++ {
		tr.Tick()
	}

	missions, err := database.Missions(10)
	if err != nil {
		t.Fatalf("missions: %v", err)
	}
	if len(missions) != 0 {
		t.Fatalf("an idle tracker must not create a mission, got %d: %+v", len(missions), missions)
	}
	if n, _ := database.CountMissions(db.SourceLive); n != 0 {
		t.Fatalf("no live mission should exist, got %d", n)
	}

	// As soon as a unit appears, a mission is opened and sampled.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane",
		Coalition: "blue", Lat: 42, Lng: 41, Alt: 5000})
	tr.Tick()

	if id := tr.currentMissionID(); id == 0 {
		t.Fatal("a mission should be opened once a unit is present")
	}
	if n, _ := database.CountMissions(db.SourceLive); n != 1 {
		t.Fatalf("want exactly 1 mission after a sample, got %d", n)
	}

	// The mission must not be duplicated by further ticks.
	tr.Tick()
	if n, _ := database.CountMissions(db.SourceLive); n != 1 {
		t.Fatalf("the mission must not be duplicated, got %d", n)
	}
}
