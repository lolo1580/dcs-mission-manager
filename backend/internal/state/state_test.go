package state

import (
	"testing"
	"time"
)

func TestUpdateAndSnapshot(t *testing.T) {
	s := New(time.Minute, 0)
	s.Update(&Unit{ID: "1", Type: "F-16C_50", Lat: 41.5, Lng: 41.8})

	got := s.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 unit, got %d", len(got))
	}
	if got[0].ID != "1" || got[0].Type != "F-16C_50" {
		t.Fatalf("unexpected unit: %+v", got[0])
	}
}

func TestUpdateRejectsEmptyID(t *testing.T) {
	s := New(time.Minute, 0)
	if s.Update(&Unit{ID: ""}) {
		t.Fatal("empty id should be rejected")
	}
	if s.Update(nil) {
		t.Fatal("nil unit should be rejected")
	}
	if n := s.Count(); n != 0 {
		t.Fatalf("want 0 units, got %d", n)
	}
}

func TestUpdateReplacesByID(t *testing.T) {
	s := New(time.Minute, 0)
	s.Update(&Unit{ID: "1", Lat: 1})
	s.Update(&Unit{ID: "1", Lat: 2})

	got := s.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 unit after replace, got %d", len(got))
	}
	if got[0].Lat != 2 {
		t.Fatalf("want latest Lat=2, got %v", got[0].Lat)
	}
}

func TestExpiredUnitsAreDropped(t *testing.T) {
	s := New(time.Nanosecond, 0)
	s.Update(&Unit{ID: "1"})
	time.Sleep(time.Millisecond)

	if n := s.Count(); n != 0 {
		t.Fatalf("want expired unit dropped, got %d", n)
	}
}

func TestMaxUnitsCap(t *testing.T) {
	s := New(time.Minute, 2)
	if !s.Update(&Unit{ID: "1"}) || !s.Update(&Unit{ID: "2"}) {
		t.Fatal("first units should be accepted")
	}
	if s.Update(&Unit{ID: "3"}) {
		t.Fatal("unit beyond the cap should be rejected")
	}
	// Updating a known unit must still work when full.
	if !s.Update(&Unit{ID: "1", Lat: 9}) {
		t.Fatal("updating a known unit should be accepted")
	}
	if n := s.Count(); n != 2 {
		t.Fatalf("want 2 units, got %d", n)
	}
}

func TestSnapshotFillsAge(t *testing.T) {
	s := New(time.Minute, 0)
	s.Update(&Unit{ID: "1"})
	time.Sleep(5 * time.Millisecond)

	got := s.Snapshot()
	if got[0].AgeMs < 5 {
		t.Fatalf("want age >= 5ms, got %d", got[0].AgeMs)
	}
}

func TestRemove(t *testing.T) {
	s := New(time.Minute, 0)
	s.Update(&Unit{ID: "1"})
	s.Remove("1")
	if n := s.Count(); n != 0 {
		t.Fatalf("want 0 units after remove, got %d", n)
	}
}

func TestHeartbeatTracksExportWithoutCreatingUnit(t *testing.T) {
	s := New(time.Minute, 0)
	if s.FeedStopped() {
		t.Fatal("a feed that never started cannot be reported as stopped")
	}
	s.Touch()
	if s.FeedStopped() || s.Count() != 0 {
		t.Fatal("heartbeat must keep an empty export feed active without creating a unit")
	}
	s.SimulateSilence(2 * time.Minute)
	if !s.FeedStopped() {
		t.Fatal("a previously active feed should be reported as stopped after its TTL")
	}
	s.Touch()
	if s.FeedStopped() {
		t.Fatal("a new heartbeat should restore the feed")
	}
}
