package live

import (
	"testing"

	"dcsmm/internal/model"
)

func TestAddEventAssignsIDsAndCaps(t *testing.T) {
	s := New(3, 2)
	for i := 0; i < 5; i++ {
		e := s.AddEvent(model.Event{Event: "kill"})
		if e.ID != int64(i+1) {
			t.Fatalf("event %d: want id %d, got %d", i, i+1, e.ID)
		}
	}
	ev := s.Events()
	if len(ev) != 3 {
		t.Fatalf("want 3 events kept, got %d", len(ev))
	}
	// Oldest two were dropped.
	if ev[0].ID != 3 || ev[2].ID != 5 {
		t.Fatalf("unexpected ids after trim: %+v", ev)
	}
}

func TestPlayersSortAndReplace(t *testing.T) {
	s := New(10, 10)
	s.SetPlayers([]model.Player{
		{ID: 5, Side: 0, Name: "Spec"},
		{ID: 3, Side: 2, Name: "Blue2"},
		{ID: 1, Side: 1, Name: "Red1"},
		{ID: 2, Side: 2, Name: "Blue1"},
	})

	got := s.Players()
	want := []int{1, 2, 3, 5} // red, blue, spectator
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d: want id %d, got %d (%+v)", i, id, got[i].ID, got)
		}
	}

	// Replacing the roster drops players that left.
	s.SetPlayers([]model.Player{{ID: 9, Side: 1, Name: "New"}})
	if got := s.Players(); len(got) != 1 || got[0].ID != 9 {
		t.Fatalf("roster should be replaced, got %+v", got)
	}
}

func TestChatCap(t *testing.T) {
	s := New(10, 2)
	s.AddChat(model.Chat{From: "a"})
	s.AddChat(model.Chat{From: "b"})
	s.AddChat(model.Chat{From: "c"})
	got := s.Chat()
	if len(got) != 2 || got[0].From != "b" {
		t.Fatalf("want last 2 messages starting at b, got %+v", got)
	}
}

func TestMissionLifecycle(t *testing.T) {
	s := New(10, 10)
	if _, ok := s.Mission(); ok {
		t.Fatal("no mission expected initially")
	}
	s.StartMission(model.Mission{Name: "Test", StartedAt: 100})
	s.EndMission("blue", 200)

	m, ok := s.Mission()
	if !ok {
		t.Fatal("mission expected")
	}
	if m.Name != "Test" || m.Winner != "blue" || m.EndedAt != 200 {
		t.Fatalf("unexpected mission: %+v", m)
	}
}

func TestSideName(t *testing.T) {
	cases := map[int]string{0: "spectator", 1: "red", 2: "blue", 9: "spectator"}
	for side, want := range cases {
		if got := (model.Player{Side: side}).SideName(); got != want {
			t.Errorf("side %d: want %q, got %q", side, want, got)
		}
	}
}
