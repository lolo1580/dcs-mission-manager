package state

import (
	"testing"
	"time"
)

func TestUpdateAndSnapshot(t *testing.T) {
	s := New(time.Minute)
	s.Update(&Unit{Name: "Player", UnitType: "F-16C_50", Lat: 41.5, Lng: 41.8})

	got := s.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 unit, got %d", len(got))
	}
	if got[0].Name != "Player" || got[0].UnitType != "F-16C_50" {
		t.Fatalf("unexpected unit: %+v", got[0])
	}
	if got[0].UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}
}

func TestUpdateIgnoresEmptyName(t *testing.T) {
	s := New(time.Minute)
	s.Update(&Unit{Name: ""})
	s.Update(nil)
	if n := s.Count(); n != 0 {
		t.Fatalf("want 0 units, got %d", n)
	}
}

func TestUpdateReplacesByName(t *testing.T) {
	s := New(time.Minute)
	s.Update(&Unit{Name: "Player", Lat: 1})
	s.Update(&Unit{Name: "Player", Lat: 2})

	got := s.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 unit after replace, got %d", len(got))
	}
	if got[0].Lat != 2 {
		t.Fatalf("want latest Lat=2, got %v", got[0].Lat)
	}
}

func TestExpiredUnitsAreDropped(t *testing.T) {
	s := New(time.Nanosecond)
	s.Update(&Unit{Name: "Player"})
	time.Sleep(time.Millisecond)

	if n := s.Count(); n != 0 {
		t.Fatalf("want expired unit dropped, got %d", n)
	}
}
