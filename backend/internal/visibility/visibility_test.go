package visibility

import (
	"testing"

	"dcsmm/internal/state"
)

func TestFromDCSOption(t *testing.T) {
	cases := map[string]Mode{
		"optview_onlymap":    MapOnly,
		"optview_myaircraft": MyAircraft,
		"optview_allies":     Allies,
		"optview_onlyallies": OnlyAllies,
		"optview_all":        All,
		"":                   Unknown,
		"garbage":            Unknown,
		"OPTVIEW_ALL":        All, // case-insensitive
	}
	for in, want := range cases {
		if got := FromDCSOption(in); got != want {
			t.Errorf("FromDCSOption(%q) = %q, want %q", in, got, want)
		}
	}
}

func units() []state.Unit {
	return []state.Unit{
		{ID: "ownship", Coalition: "blue", Ownship: true},
		{ID: "blue-f16", Coalition: "blue"},
		{ID: "blue-tank", Coalition: "blue"},
		{ID: "red-su27", Coalition: "red"},
		{ID: "neutral-ship", Coalition: "neutral"},
	}
}

func TestFilterRestrictive(t *testing.T) {
	cases := []struct {
		mode Mode
		want int
	}{
		{MapOnly, 1},    // ownship only
		{MyAircraft, 1}, // ownship only
		{Allies, 3},     // ownship + 2 blue
		{OnlyAllies, 3}, // ownship + 2 blue
		{All, 5},        // everything
		{Unknown, 3},    // restrictive fallback: ownside only
	}
	for _, c := range cases {
		p := New(false)
		p.SetMode(c.mode)
		got := p.Filter(units())
		if len(got) != c.want {
			t.Errorf("mode %s: want %d units, got %d", c.mode, c.want, len(got))
		}
		// The ownship must always survive, so the map stays usable.
		if c.mode != MapOnly && c.mode != MyAircraft {
			found := false
			for _, u := range got {
				if u.Ownship {
					found = true
				}
			}
			if !found {
				t.Errorf("mode %s: ownship must always be visible", c.mode)
			}
		}
	}
}

func TestFilterNeverLeaksEnemies(t *testing.T) {
	// The whole point: enemy and neutral units must never appear in the
	// restrictive modes, whatever the configuration.
	for _, mode := range []Mode{MapOnly, MyAircraft, Allies, OnlyAllies, Unknown} {
		p := New(false)
		p.SetMode(mode)
		for _, u := range p.Filter(units()) {
			if u.Coalition == "red" {
				t.Errorf("mode %s leaked an enemy unit %s", mode, u.ID)
			}
			if u.Coalition == "neutral" {
				t.Errorf("mode %s leaked a neutral unit %s", mode, u.ID)
			}
		}
	}
}

func TestOverrideRevealsAll(t *testing.T) {
	p := New(true)
	p.SetMode(MapOnly)
	if got := p.Filter(units()); len(got) != 5 {
		t.Fatalf("override should reveal everything, got %d", len(got))
	}
	d := p.Describe()
	if !d.Override || d.Note == "" {
		t.Fatalf("description should flag the override: %+v", d)
	}
}

func TestFilterWithoutOwnship(t *testing.T) {
	// Before the ownship appears, no coalition is known: nothing but the
	// ownship itself may be shown. This avoids guessing a side.
	p := New(false)
	p.SetMode(Allies)

	anonymous := []state.Unit{
		{ID: "blue-f16", Coalition: "blue"},
		{ID: "red-su27", Coalition: "red"},
	}
	if got := p.Filter(anonymous); len(got) != 0 {
		t.Fatalf("without an ownship nothing should be shown, got %d", len(got))
	}
}

func TestDescribe(t *testing.T) {
	p := New(false)
	p.SetMode(Allies)
	d := p.Describe()
	if d.Mode != Allies || d.Label == "" {
		t.Fatalf("unexpected description: %+v", d)
	}
	if d.Note == "" {
		t.Fatal("the fog-of-war approximation should be documented in the note")
	}
}
