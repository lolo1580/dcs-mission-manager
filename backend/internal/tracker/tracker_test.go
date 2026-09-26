package tracker

import (
	"path/filepath"
	"testing"
	"time"

	"dcsmm/internal/db"
	"dcsmm/internal/model"
	"dcsmm/internal/state"
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
	tr, database, _ := setup(t)
	// A zero grace means a unit missing from this tick is reported immediately.
	tr.grace = 0

	// Previous state: unit 1 was seen a while ago, unit 2 just now.
	tr.seen["1"] = sampleAt("1", time.Now().Add(-time.Second).UnixMilli())
	tr.seen["2"] = sampleAt("2", time.Now().UnixMilli())

	// This tick: no unit is present, so unit 1 (older than grace) is lost and
	// unit 2 is not (it was seen within the grace window of this instant).
	tr.Tick()

	points, err := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected at least one loss")
	}
	if tr.lostIDs["1"] != true {
		t.Fatal("unit 1 should be marked lost")
	}

	// A second tick must not report the same loss twice.
	before := len(points)
	tr.Tick()
	after, _ := database.Heatmap(tr.missionID, "losses", 0.05, 100)
	if len(after) != before {
		t.Fatalf("loss reported twice: %d -> %d", before, len(after))
	}
}

func TestReappearingUnitClearsLoss(t *testing.T) {
	tr, _, store := setup(t)
	tr.grace = 0

	tr.seen["1"] = sampleAt("1", time.Now().Add(-time.Second).UnixMilli())
	tr.lostIDs["1"] = true

	// The unit reappears.
	store.Update(&state.Unit{ID: "1", Type: "F-16C_50", Category: "plane", Lat: 42, Lng: 41})
	tr.Tick()

	if tr.lostIDs["1"] {
		t.Fatal("a unit that reappears should no longer be considered lost")
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
