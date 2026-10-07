package mapping

import (
	"dcsmanager/internal/panel"
	"path/filepath"
	"testing"
)

func TestFlapsReturnToCentreDoesNotUndoPosition(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	for _, iface := range []string{"set_state", "fixed_step", "variable_step", "action"} {
		if err := s.SetProfile(Profile{Aircraft: "A", Bindings: []Binding{{Model: panel.PZ70, Control: "FLAPS_DOWN", Command: "FLAPS", Interface: iface}}}); err != nil {
			t.Fatal(err)
		}
		ev := panel.Event{Model: panel.PZ70, Control: panel.Control{ID: "FLAPS_DOWN", Kind: panel.Button}, Active: true}
		if len(s.Simulate("A", ev, nil)) != 1 {
			t.Fatalf("%s must act on press", iface)
		}
		ev.Active = false
		got := s.Simulate("A", ev, nil)
		if iface == "action" {
			if len(got) != 1 || got[0] != "FLAPS 0\n" {
				t.Fatalf("release: %v", got)
			}
		} else if len(got) != 0 {
			t.Fatalf("centre undoes %s: %v", iface, got)
		}
	}
}

func TestTrimInversionAndAutoThrottle(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	if err := s.SetProfile(Profile{Aircraft: "A", Bindings: []Binding{
		{Model: panel.PZ70, Control: "PITCH_TRIM", Command: "TRIM", Interface: "variable_step", Invert: true},
		{Model: panel.PZ70, Control: "AUTO_THROTTLE", Command: "AT", Interface: "set_state"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, direction := range []bool{true, false} {
		got := s.Simulate("A", panel.Event{Model: panel.PZ70, Control: panel.Control{ID: "PITCH_TRIM", Kind: panel.EncoderPulse, Clockwise: direction}, Active: true}, nil)
		want := "TRIM 1\n"
		if direction {
			want = "TRIM -1\n"
		}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("direction %v: %v", direction, got)
		}
	}
	for _, active := range []bool{true, false} {
		got := s.Simulate("A", panel.Event{Model: panel.PZ70, Control: panel.Control{ID: "AUTO_THROTTLE", Kind: panel.Toggle}, Active: active}, nil)
		want := "AT 0\n"
		if active {
			want = "AT 1\n"
		}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("ON/OFF: %v", got)
		}
	}
}

func TestGearOldPositionDoesNotSend(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	if err := s.SetProfile(Profile{Aircraft: "A", Bindings: []Binding{{Model: panel.PZ55, Control: "GEAR_UP", Command: "GEAR", Interface: "set_state", Invert: true}}}); err != nil {
		t.Fatal(err)
	}
	got := s.Simulate("A", panel.Event{Model: panel.PZ55, Control: panel.Control{ID: "GEAR_UP", Kind: panel.Toggle}, Active: false}, nil)
	if len(got) != 0 {
		t.Fatalf("old gear position sent %v", got)
	}
}
