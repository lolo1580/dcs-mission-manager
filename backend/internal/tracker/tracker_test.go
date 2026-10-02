package tracker

import (
	"path/filepath"
	"testing"
	"time"

	"dcsmanager/internal/db"
	"dcsmanager/internal/model"
	"dcsmanager/internal/state"
)

func setup(t *testing.T) (*Tracker, *db.DB, *state.Store) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := state.New(time.Minute, 0)
	tr := New(database, store, Options{SampleEvery: time.Second, Grace: time.Millisecond})
	missionID, _ := database.EnsureMission("Track Test", "Caucasus")
	tr.SetMissionID(missionID)
	return tr, database, store
}

// TestTickPromotesMissionSourceWhenDetectedLate covers the regression found by
// end-to-end testing: the tracker opens the mission at its first tick, which can
// happen *before* the UDP listener recognises the test tools. The mission would
// then stay tagged "live" for the whole session unless a later tick promotes it.
func TestTickPromotesMissionSourceWhenDetectedLate(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := state.New(time.Minute, 0)
	tr := New(database, store, Options{SampleEvery: time.Second, Grace: time.Millisecond})

	// The session starts looking real: the tracker opens a default mission.
	source := db.SourceLive
	tr.SetMissionSource(func() string { return source })
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane",
		Coalition: "blue", Lat: 42, Lng: 41, Alt: 5000})
	tr.Tick()

	id := tr.currentMissionID()
	if id == 0 {
		t.Fatal("the tracker should have opened a mission")
	}
	if got := missionSourceOf(t, database, id); got != db.SourceLive {
		t.Fatalf("mission should start as %q, got %q", db.SourceLive, got)
	}

	// The test tools are then recognised, and a later tick must promote it.
	source = db.SourceTest
	tr.Tick()
	if got := missionSourceOf(t, database, id); got != db.SourceTest {
		t.Fatalf("a late detection must promote the mission to %q, got %q", db.SourceTest, got)
	}

	// And it must never fall back to live.
	source = db.SourceLive
	tr.Tick()
	if got := missionSourceOf(t, database, id); got != db.SourceTest {
		t.Fatalf("the promotion must not be undone, got %q", got)
	}
}

func missionSourceOf(t *testing.T, d *db.DB, id int64) string {
	t.Helper()
	var source string
	if err := d.SQL().QueryRow(`SELECT source FROM missions WHERE id = ?`, id).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	return source
}

func TestTickSamplesPositions(t *testing.T) {
	tr, database, store := setup(t)

	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane",
		Coalition: "blue", Lat: 42, Lng: 41, Alt: 5000, Heading: 90, Speed: 250, G: 1.5})

	tr.Tick()

	trails, err := database.Trails(tr.missionID, 10, 100)
	if err != nil {
		t.Fatalf("trails: %v", err)
	}
	pts, ok := trails["1"]
	if !ok || len(pts) != 1 {
		t.Fatalf("want 1 trail point for unit 1, got %#v", trails)
	}
	if pts[0].Lat != 42 || pts[0].Speed != 250 {
		t.Fatalf("unexpected point: %+v", pts[0])
	}
}

func TestTickDetectsLoss(t *testing.T) {
	tr, database, store := setup(t)
	// A zero grace means any unit missing from this tick is reported.
	tr.grace = 0

	// A real sequence: the unit is reported, sampled, then disappears.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42, Lng: 41})
	tr.Tick()
	if _, tracked := tr.seen["1"]; !tracked {
		t.Fatal("the unit should have been sampled")
	}

	store.Remove("1")
	tr.Tick()

	points, err := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected at least one loss")
	}
	// The unit is forgotten once its loss is recorded, which is what keeps the
	// tracking maps from growing without bound.
	if _, tracked := tr.seen["1"]; tracked {
		t.Error("a unit reported lost should no longer be tracked")
	}

	// A second tick must not report the same loss twice.
	before := len(points)
	tr.Tick()
	after, _ := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if len(after) != before {
		t.Fatalf("loss reported twice: %d -> %d", before, len(after))
	}
}

// TestPausedFeedDoesNotReportLosses covers the pause bug: DCS stops calling the
// export script while the simulation is paused, so the feed goes silent. That
// silence used to be read as "every unit vanished", recording a batch of losses
// at every pause.
func TestPausedFeedDoesNotReportLosses(t *testing.T) {
	tr, database, store := setup(t)
	tr.grace = 0

	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42, Lng: 41})
	tr.Tick()

	// The simulator pauses: no telemetry arrives, and the store expires the unit.
	// Simulate the expiry by rewinding the feed, then removing the unit.
	store.Remove("1")
	store.SimulateSilence(time.Minute + time.Second)

	tr.Tick()

	points, err := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("a paused simulator must not produce losses, got %d", len(points))
	}
	// The tracked state is kept, so the map resumes where it left off.
	if _, tracked := tr.seen["1"]; !tracked {
		t.Error("a pause must not discard the tracked units")
	}

	// When telemetry returns, tracking resumes normally.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42.1, Lng: 41.1})
	tr.Tick()
	if _, tracked := tr.seen["1"]; !tracked {
		t.Error("tracking should resume once telemetry returns")
	}
}

// TestUnitWithinGraceIsNotLost checks the grace period actually holds a unit
// back: with a long grace, a unit that stops being reported must not be declared
// lost immediately.
func TestUnitWithinGraceIsNotLost(t *testing.T) {
	tr, database, _ := setup(t)
	tr.grace = time.Hour

	tr.seen["1"] = sampleAt("1", time.Now().UnixMilli())

	tr.Tick()

	if _, tracked := tr.seen["1"]; !tracked {
		t.Fatal("a unit within its grace window should still be tracked")
	}
	points, err := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("no loss should be recorded within the grace window, got %d", len(points))
	}
}

func TestReappearingUnitIsNotLost(t *testing.T) {
	tr, database, store := setup(t)
	tr.grace = 0

	tr.seen["1"] = sampleAt("1", time.Now().Add(-time.Second).UnixMilli())

	// The unit reappears before the tick.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42, Lng: 41})
	tr.Tick()

	// It must still be tracked, and no loss must have been recorded.
	if _, tracked := tr.seen["1"]; !tracked {
		t.Fatal("a unit that reappears should still be tracked")
	}
	points, err := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("a unit that reappears must not be reported lost, got %d loss(es)", len(points))
	}
}

// TestTrackingMapsStayBounded covers the leak: units that are gone for good must
// be forgotten, otherwise both maps grow with every unit ever seen and every tick
// scans them all.
func TestTrackingMapsStayBounded(t *testing.T) {
	tr, _, store := setup(t)
	tr.grace = 0

	// Two units appear, then vanish for good.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42, Lng: 41})
	store.Update(&state.Unit{ID: "2", Type: "Su-27", Category: "plane", Lat: 42, Lng: 41})
	tr.Tick()
	if n := len(tr.seen); n != 2 {
		t.Fatalf("want 2 tracked units, got %d", n)
	}

	// They are gone: one tick within the grace period, then one past it.
	store.Remove("1")
	store.Remove("2")
	tr.grace = 0
	tr.Tick()
	tr.Tick()

	if n := len(tr.seen); n != 0 {
		t.Errorf("units lost for good should be forgotten, %d still tracked", n)
	}
	if n := len(tr.lostIDs); n != 0 {
		t.Errorf("lostIDs should be empty once losses are recorded, got %d", n)
	}
}

func TestHaversine(t *testing.T) {
	// One degree of latitude is about 111 km.
	d := haversineKm(42, 41, 43, 41)
	if d < 110 || d > 112 {
		t.Fatalf("distance for 1 degree latitude = %.1f km, want ~111", d)
	}
	// Same point is zero.
	if got := haversineKm(42, 41, 42, 41); got != 0 {
		t.Fatalf("distance to self = %v, want 0", got)
	}
}

func TestAnalyseSorties(t *testing.T) {
	trails := map[string][]db.TrailPoint{
		"1": {
			{Lat: 42, Lng: 41, Alt: 1000, Speed: 100, G: 1, RealTS: 0},
			{Lat: 42.1, Lng: 41, Alt: 5000, Speed: 200, G: 5, RealTS: 10000},
			{Lat: 42.2, Lng: 41, Alt: 3000, Speed: 150, G: 2, RealTS: 20000},
		},
		"2": {
			// Too short to be a sortie.
			{Lat: 42, Lng: 41, RealTS: 0},
		},
	}

	stats := Analyse(trails)
	if len(stats) != 1 {
		t.Fatalf("want 1 sortie (unit 2 too short), got %d", len(stats))
	}
	s := stats[0]
	if s.UnitID != "1" {
		t.Fatalf("unexpected unit: %s", s.UnitID)
	}
	if s.MaxAlt != 5000 || s.MaxSpeed != 200 || s.MaxG != 5 {
		t.Fatalf("unexpected extremes: %+v", s)
	}
	if s.DistanceKm < 20 || s.DistanceKm > 25 {
		t.Fatalf("distance = %.1f km, want ~22", s.DistanceKm)
	}
	if s.DurationSec != 20 {
		t.Fatalf("duration = %v, want 20", s.DurationSec)
	}
}

func sampleAt(id string, ts int64) model.Sample {
	return model.Sample{UnitID: id, Type: "F-16C_50", Category: "plane", RealTS: ts}
}
