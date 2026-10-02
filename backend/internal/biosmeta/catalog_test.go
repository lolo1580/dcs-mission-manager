package biosmeta

import (
	"os"
	"path/filepath"
	"testing"
)

// sampleCatalog is a hand-written module covering the four command interfaces
// and a read-only display, so the parsing and command logic are tested without a
// DCS-BIOS install.
const sampleCatalog = `{
  "Gear": {
    "GEAR_LEVER": {
      "category": "Gear",
      "control_type": "selector",
      "description": "Landing Gear Lever",
      "identifier": "GEAR_LEVER",
      "inputs": [
        {"description": "switch to previous or next state", "interface": "fixed_step"},
        {"description": "set position", "interface": "set_state", "max_value": 2}
      ],
      "outputs": [{"address": 17462, "description": "selector position", "mask": 768, "max_value": 2, "shift_by": 8, "type": "integer"}]
    }
  },
  "UFC": {
    "UFC_1": {
      "category": "UFC",
      "control_type": "momentary",
      "description": "UFC Button 1",
      "identifier": "UFC_1",
      "inputs": [{"description": "trigger", "interface": "action"}],
      "outputs": [{"address": 17500, "description": "pressed", "type": "integer"}]
    }
  },
  "ADI": {
    "ADI_PITCH_TRIM": {
      "category": "ADI",
      "control_type": "limited_dial",
      "description": "ADI Pitch Trim Knob",
      "identifier": "ADI_PITCH_TRIM",
      "inputs": [{"description": "turn the dial", "interface": "variable_step", "suggested_step": 3200}],
      "outputs": [{"address": 17518, "type": "integer"}]
    }
  },
  "Lights": {
    "MASTER_CAUTION": {
      "category": "Lights",
      "control_type": "led",
      "description": "Master Caution Light",
      "identifier": "MASTER_CAUTION",
      "inputs": [],
      "outputs": [{"address": 17520, "type": "integer"}]
    }
  }
}`

func writeSample(t *testing.T) string {
	t.Helper()
	sg := t.TempDir()
	dir := JSONDir(sg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "F-16C_50.json"), []byte(sampleCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	return sg
}

// TestLoadCatalog checks the file is parsed into controls, indexed by identifier
// and grouped by category.
func TestLoadCatalog(t *testing.T) {
	sg := writeSample(t)
	cat, err := LoadModule(sg, "F-16C_50")
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if len(cat.Controls) != 4 {
		t.Fatalf("controls = %d, want 4", len(cat.Controls))
	}
	if cat.Module != "F-16C_50" {
		t.Errorf("module = %q", cat.Module)
	}

	// Lookup is case-insensitive, since DCS-BIOS uses upper case.
	ctl, ok := cat.ByID("gear_lever")
	if !ok || ctl.Identifier != "GEAR_LEVER" {
		t.Fatalf("ByID failed: %+v", ctl)
	}
	if ctl.Category != "Gear" {
		t.Errorf("category = %q, want Gear", ctl.Category)
	}

	// Categories are sorted and complete.
	got := cat.Categories()
	want := []string{"ADI", "Gear", "Lights", "UFC"}
	if len(got) != len(want) {
		t.Fatalf("categories = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("categories = %v, want %v", got, want)
		}
	}

	// Writable filters out the read-only display.
	writable := cat.Writable()
	if len(writable) != 3 {
		t.Fatalf("writable = %d, want 3 (the LED is read-only)", len(writable))
	}
	for _, c := range writable {
		if c.Identifier == "MASTER_CAUTION" {
			t.Error("a control with no inputs must not be writable")
		}
	}
}

// TestLoadModuleMissing checks an aircraft without metadata is an ordinary
// "not found", which the caller can ignore.
func TestLoadModuleMissing(t *testing.T) {
	_, err := LoadModule(t.TempDir(), "NoSuchAircraft")
	if !os.IsNotExist(err) {
		t.Fatalf("want a not-exist error, got %v", err)
	}
}

// TestLoadModuleRejectsTraversal checks a crafted module name cannot read a file
// outside the metadata folder. The name comes from the aircraft DCS reports, but
// it is still untrusted input.
func TestLoadModuleRejectsTraversal(t *testing.T) {
	sg := t.TempDir()
	// Put a file one level above the json folder.
	if err := os.MkdirAll(JSONDir(sg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(JSONDir(sg), "..", "secret.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadModule(sg, "../../secret"); !os.IsNotExist(err) {
		t.Fatalf("traversal should be refused, got %v", err)
	}
}

// TestArgForInput covers the argument conventions of each interface, which is
// where a wrong byte silently sends the wrong command.
func TestArgForInput(t *testing.T) {
	sg := writeSample(t)
	cat, _ := LoadModule(sg, "F-16C_50")
	gear, _ := cat.ByID("GEAR_LEVER")
	ufc, _ := cat.ByID("UFC_1")
	dial, _ := cat.ByID("ADI_PITCH_TRIM")
	led, _ := cat.ByID("MASTER_CAUTION")

	// fixed_step: forward on "on", back on "off".
	if v, ok := ArgForInput(gear, FixedStep, true); !ok || v != 0 {
		t.Errorf("fixed_step on = (%d, %v), want (0, true)", v, ok)
	}
	if v, ok := ArgForInput(gear, FixedStep, false); !ok || v != 2 {
		t.Errorf("fixed_step off = (%d, %v), want (2, true)", v, ok)
	}
	// set_state: the maximum for "on", zero for "off".
	if v, ok := ArgForInput(gear, SetState, true); !ok || v != 2 {
		t.Errorf("set_state on = (%d, %v), want (2, true)", v, ok)
	}
	if v, ok := ArgForInput(gear, SetState, false); !ok || v != 0 {
		t.Errorf("set_state off = (%d, %v), want (0, true)", v, ok)
	}
	// action: press then release.
	if v, ok := ArgForInput(ufc, Action, true); !ok || v != 1 {
		t.Errorf("action press = (%d, %v), want (1, true)", v, ok)
	}
	if v, ok := ArgForInput(ufc, Action, false); !ok || v != 0 {
		t.Errorf("action release = (%d, %v), want (0, true)", v, ok)
	}
	// variable_step: the suggested step, signed.
	if v, ok := ArgForInput(dial, VariableStep, true); !ok || v != 3200 {
		t.Errorf("variable_step on = (%d, %v), want (3200, true)", v, ok)
	}
	if v, ok := ArgForInput(dial, VariableStep, false); !ok || v != -3200 {
		t.Errorf("variable_step off = (%d, %v), want (-3200, true)", v, ok)
	}

	// An interface the control does not accept is refused, not guessed.
	if _, ok := ArgForInput(gear, Action, true); ok {
		t.Error("a selector does not accept action")
	}
	if _, ok := ArgForInput(led, Action, true); ok {
		t.Error("a read-only display accepts nothing")
	}
}

// TestDefaultInterface checks the interface suggested for each kind of control.
func TestDefaultInterface(t *testing.T) {
	sg := writeSample(t)
	cat, _ := LoadModule(sg, "F-16C_50")

	cases := map[string]Interface{
		"UFC_1":          Action,   // momentary
		"GEAR_LEVER":     SetState, // selector, prefers the idempotent one
		"ADI_PITCH_TRIM": VariableStep,
	}
	for id, want := range cases {
		ctl, _ := cat.ByID(id)
		got, ok := DefaultInterface(ctl)
		if !ok || got != want {
			t.Errorf("DefaultInterface(%s) = (%q, %v), want (%q, true)", id, got, ok, want)
		}
	}
	led, _ := cat.ByID("MASTER_CAUTION")
	if _, ok := DefaultInterface(led); ok {
		t.Error("a read-only display should have no interface")
	}
}

// TestValidate checks an invalid mapping is refused with a message that says what
// the control does accept.
func TestValidate(t *testing.T) {
	sg := writeSample(t)
	cat, _ := LoadModule(sg, "F-16C_50")
	gear, _ := cat.ByID("GEAR_LEVER")
	led, _ := cat.ByID("MASTER_CAUTION")

	if err := Validate(gear, SetState); err != nil {
		t.Errorf("a supported interface should validate: %v", err)
	}
	if err := Validate(gear, Action); err == nil {
		t.Error("an unsupported interface should be refused")
	}
	if err := Validate(gear, ""); err == nil {
		t.Error("an empty interface should be refused")
	}
	if err := Validate(led, Action); err == nil {
		t.Error("a read-only control should be refused")
	}
}

// TestCommandLine checks the wire format: identifier, space, value, newline.
func TestCommandLine(t *testing.T) {
	got := Command{Identifier: "GEAR_LEVER", Value: 2}.Line()
	if got != "GEAR_LEVER 2\n" {
		t.Errorf("Line() = %q", got)
	}
}
