package mapping

import (
	"dcsmanager/internal/panel"
	"path/filepath"
	"testing"
)

func TestWheelContextOverridesFallback(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	profile := Profile{Aircraft: "A", Bindings: []Binding{
		{Model: panel.PZ70, Control: "LCD_WHEEL", Command: "DEFAULT", Interface: "variable_step"},
		{Model: panel.PZ70, Control: "LCD_WHEEL", Mode: "ALT", Command: "ALTITUDE", Interface: "variable_step"},
		{Model: panel.PZ70, Control: "LCD_WHEEL", Mode: "HDG", Command: "HEADING", Interface: "variable_step"},
	}}
	if err := store.SetProfile(profile); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ mode, command string }{{"ALT", "ALTITUDE"}, {"HDG", "HEADING"}, {"VS", "DEFAULT"}, {"", "DEFAULT"}} {
		got := store.Simulate("A", panel.Event{Model: panel.PZ70, Mode: test.mode, Control: panel.Control{ID: "LCD_WHEEL", Kind: panel.EncoderPulse, Clockwise: true}, Active: true}, nil)
		if len(got) != 1 || got[0] != test.command+" 1\n" {
			t.Fatalf("mode %s: %v", test.mode, got)
		}
	}
	reloaded := NewStore(filepath.Join(filepath.Dir(store.path), "mappings.json"), nil)
	if len(reloaded.Profile("A").Bindings) != 3 {
		t.Fatal("contexts not persisted")
	}
	profile.Bindings[1].Mode = "invalid"
	if store.SetProfile(profile) == nil {
		t.Fatal("invalid context accepted")
	}
}

func TestLEDInvalidRulesRejected(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	for _, rule := range []OutputRule{{Command: "VALUE", Operator: "invalid", Color: "green"}, {Command: "VALUE", Operator: "eq", Export: -1, Color: "green"}, {Command: "VALUE", Operator: "eq", Color: "rgb"}} {
		if store.SetProfile(Profile{Aircraft: "A", Outputs: []OutputBinding{{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "VALUE", Rules: []OutputRule{rule}}}}) == nil {
			t.Fatalf("invalid rule accepted: %+v", rule)
		}
	}
}
