package mapping

import (
	"dcsmanager/internal/panel"
	"path/filepath"
	"testing"
)

func TestExplicitPositionsAndRealStepArguments(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	p := Profile{Aircraft: "A", Bindings: []Binding{
		{Model: panel.PZ55, Control: "MASTER_BAT", Command: "BATTERY", Interface: "set_state", StateOn: stateValue(0), StateOff: stateValue(1)},
		{Model: panel.PZ70, Control: "FLAPS_UP", Command: "FLAPS", Interface: "fixed_step", Invert: true},
		{Model: panel.PZ70, Control: "FLAPS_DOWN", Command: "FLAPS", Interface: "fixed_step"},
	}}
	if err := s.SetProfile(p); err != nil {
		t.Fatal(err)
	}
	cat := catalogWithSetState(t, "A", map[string]bool{"BATTERY": true}, 2)
	for _, tc := range []struct {
		model  panel.Model
		id     string
		active bool
		want   string
	}{
		{panel.PZ55, "MASTER_BAT", true, "BATTERY 0\n"}, {panel.PZ55, "MASTER_BAT", false, "BATTERY 1\n"},
		{panel.PZ70, "FLAPS_UP", true, "FLAPS DEC\n"}, {panel.PZ70, "FLAPS_DOWN", true, "FLAPS INC\n"},
		{panel.PZ70, "FLAPS_UP", false, ""},
	} {
		var metadata = cat
		if tc.model == panel.PZ70 {
			metadata = nil
		}
		got := s.Simulate("A", panel.Event{Model: tc.model, Control: panel.Control{ID: tc.id, Kind: panel.Toggle}, Active: tc.active}, metadata)
		if tc.want == "" {
			if len(got) != 0 {
				t.Fatalf("release: %v", got)
			}
			continue
		}
		if len(got) != 1 || got[0] != tc.want {
			t.Fatalf("%s/%v: %v, want %q", tc.id, tc.active, got, tc.want)
		}
	}
	reloaded := NewStore(s.path, nil)
	if *reloaded.Profile("A").Bindings[0].StateOn != 0 {
		t.Fatal("explicit zero lost on reload")
	}
	p.Bindings[0].StateOn = stateValue(3)
	if err := s.SetProfile(p); err != nil {
		t.Fatal(err)
	}
	if got := s.Simulate("A", panel.Event{Model: panel.PZ55, Control: panel.Control{ID: "MASTER_BAT"}, Active: true}, cat); len(got) != 0 {
		t.Fatalf("out-of-range position sent: %v", got)
	}
	p.Bindings[0].StateOn = stateValue(-1)
	if s.SetProfile(p) == nil {
		t.Fatal("negative position accepted")
	}
}

func TestAllStarterProfilesPassStoreValidation(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "profiles.json"), nil)
	if len(Starter()) != 7 {
		t.Fatalf("want 7 built-in profiles, got %d", len(Starter()))
	}
	for _, p := range Starter() {
		if err := s.SetProfile(p); err != nil {
			t.Fatalf("%s: %v", p.Aircraft, err)
		}
	}
}
