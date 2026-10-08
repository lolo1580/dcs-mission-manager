package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"dcsmanager/internal/live"
	"dcsmanager/internal/state"
)

func sample() []state.Unit {
	return []state.Unit{
		{ID: "1", Type: "F-16C_50", Category: "plane", Coalition: "blue", Ownship: true, Label: "Player"},
		{ID: "2", Type: "T-72B", Category: "ground", Coalition: "red"},
		{ID: "3", Type: "USS_Arleigh_Burke", Category: "ship", Coalition: "blue", Country: "USA"},
	}
}

func TestSessionReportsExportStateWithoutClaimingPause(t *testing.T) {
	store := state.New(time.Second, 0)
	s := &Server{store: store, live: live.New(5, 5)}
	check := func(wantSeen, wantStopped bool) {
		t.Helper()
		data, err := s.sessionJSON()
		if err != nil {
			t.Fatal(err)
		}
		var frame struct {
			FeedSeen    bool `json:"feedSeen"`
			FeedStopped bool `json:"feedStopped"`
		}
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.FeedSeen != wantSeen || frame.FeedStopped != wantStopped {
			t.Fatalf("unexpected export state: %+v", frame)
		}
	}
	check(false, false)
	store.Touch()
	check(true, false)
	store.SimulateSilence(2 * time.Second)
	check(true, true)
}

func TestFilterNoParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state", nil)
	if got := filter(sample(), r); len(got) != 3 {
		t.Fatalf("want 3 units, got %d", len(got))
	}
}

func TestFilterByCategory(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?category=ground", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestFilterByCoalition(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?coalition=blue", nil)
	if got := filter(sample(), r); len(got) != 2 {
		t.Fatalf("want 2 blue units, got %d", len(got))
	}
}

func TestFilterOwnship(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?ownship=true", nil)
	got := filter(sample(), r)
	if len(got) != 1 || !got[0].Ownship {
		t.Fatalf("want only ownship, got %+v", got)
	}
}

func TestFilterSearch(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?q=burke", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("search should match type, got %+v", got)
	}
}

func TestFilterSearchCaseInsensitive(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?q=PLAYER", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("search should be case-insensitive and match label, got %+v", got)
	}
}

func TestSummarise(t *testing.T) {
	s := summarise(sample())
	if s.ByCategory["plane"] != 1 || s.ByCoalition["blue"] != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	// Missing category/coalition default to other/neutral.
	s2 := summarise([]state.Unit{{ID: "x"}})
	if s2.ByCategory["other"] != 1 || s2.ByCoalition["neutral"] != 1 {
		t.Fatalf("unexpected default summary: %+v", s2)
	}
}
