package api

import (
	"net/http/httptest"
	"testing"

	"dcsmm/internal/state"
)

func sample() []state.Unit {
	return []state.Unit{
		{ID: "1", Type: "F-16C_50", Category: "plane", Coalition: "blue", Ownship: true, Label: "Player"},
		{ID: "2", Type: "T-72B", Category: "ground", Coalition: "red"},
		{ID: "3", Type: "USS_Arleigh_Burke", Category: "ship", Coalition: "blue", Country: "USA"},
	}
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

func TestValidTilePart(t *testing.T) {
	for _, ok := range []string{"Caucasus", "3", "12", "17", "abc-1_2"} {
		if !validTilePart(ok) {
			t.Errorf("validTilePart(%q) should be true", ok)
		}
	}
	for _, bad := range []string{"", "../", "a/b", "a.b", "a b", "a\\b"} {
		if validTilePart(bad) {
			t.Errorf("validTilePart(%q) should be false", bad)
		}
	}
}
