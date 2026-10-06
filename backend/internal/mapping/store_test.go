package mapping

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/panel"
)

// recorder captures the commands the store sends.
type recorder struct {
	lines []string
	fail  bool
}

func (r *recorder) send(line string) error {
	if r.fail {
		return os.ErrInvalid
	}
	r.lines = append(r.lines, line)
	return nil
}

// TestOutputsSwitchIsIndependent checks the outputs switch is separate from the
// command-sending switch: it needs no transport and does not require sending.
func TestOutputsSwitchIsIndependent(t *testing.T) {
	// No transport at all: outputs must still be switchable.
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	if s.OutputsEnabled() {
		t.Fatal("outputs should start off")
	}
	s.SetOutputsEnabled(true)
	if !s.OutputsEnabled() {
		t.Fatal("outputs should be on after SetOutputsEnabled(true)")
	}
	if s.Enabled() {
		t.Fatal("turning outputs on must not arm command sending")
	}

	// Persistence: a profile can carry outputs and displays without sending.
	p := Profile{
		Aircraft: "F-16C_50",
		Outputs:  []OutputBinding{{Model: panel.PZ55, Target: panel.TargetGearLeft, Command: "LIGHT_GEAR_L", Color: "green"}},
		Displays: []DisplayBinding{{Model: panel.PZ70, Mode: "ALT", Line: "upper", Command: "ALT_SEL", Export: 0}},
	}
	if err := s.SetProfile(p); err != nil {
		t.Fatal(err)
	}
	got := s.Profile("F-16C_50")
	if len(got.Outputs) != 1 || len(got.Displays) != 1 {
		t.Fatalf("profile lost outputs/displays: %+v", got)
	}
}

// TestDisplayValidation checks malformed LCD bindings are refused: a bad mode,
// line, duplicate, negative export index and non-finite conversion.
func TestDisplayValidation(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
	base := func(dp DisplayBinding) error {
		return s.SetProfile(Profile{Aircraft: "A", Displays: []DisplayBinding{dp}})
	}
	ok := DisplayBinding{Model: panel.PZ70, Mode: "ALT", Line: "upper", Command: "ALT_SEL", Export: 0}
	if err := base(ok); err != nil {
		t.Fatalf("a valid display was refused: %v", err)
	}
	bad := []DisplayBinding{
		{Model: panel.PZ70, Mode: "NOPE", Line: "upper", Command: "X", Export: 0},
		{Model: panel.PZ70, Mode: "ALT", Line: "middle", Command: "X", Export: 0},
		{Model: panel.PZ70, Mode: "ALT", Line: "upper", Command: "", Export: 0},
		{Model: panel.PZ70, Mode: "ALT", Line: "upper", Command: "X", Export: -1},
		{Model: panel.PZ55, Mode: "ALT", Line: "upper", Command: "X", Export: 0},
	}
	for i, dp := range bad {
		if err := base(dp); err == nil {
			t.Errorf("display %d should have been refused: %+v", i, dp)
		}
	}

	// Two displays on the same mode+line collide.
	dup := []DisplayBinding{ok, ok}
	if err := s.SetProfile(Profile{Aircraft: "A", Displays: dup}); err == nil {
		t.Error("two displays on the same line should be refused")
	}
}

// apControl finds the AP button in the PZ70 definitions, so the tests use the real
// control rather than a guessed id.
func apControl(t *testing.T) panel.Control {
	t.Helper()
	for _, c := range panel.Controls(panel.PZ70) {
		if c.ID == "AP_BUTTON" {
			return c
		}
	}
	t.Fatal("AP_BUTTON should be defined")
	return panel.Control{}
}

// gearControl finds the PZ55 gear-down switch.
func gearControl(t *testing.T) panel.Control {
	t.Helper()
	for _, c := range panel.Controls(panel.PZ55) {
		if c.ID == "GEAR_DOWN" {
			return c
		}
	}
	t.Fatal("GEAR_DOWN should be defined")
	return panel.Control{}
}

// TestStoreIsDisabledByDefault locks the safety property: a fresh store sends
// nothing, even with a valid binding. Sending into a live cockpit must be an
// explicit choice.
func TestStoreIsDisabledByDefault(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)

	if s.Enabled() {
		t.Fatal("a new store must be disabled")
	}
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	}); err != nil {
		t.Fatal(err)
	}

	ap := apControl(t)
	ev := panel.Event{Model: panel.PZ70, Control: ap, Active: true}
	if sent := s.Apply("F-16C_50", ev, nil); len(sent) != 0 {
		t.Fatalf("nothing should be sent while disabled, got %v", sent)
	}
	if len(rec.lines) != 0 {
		t.Fatalf("the transport should not have been called, got %v", rec.lines)
	}
}

// TestApplyWhenEnabled checks the happy path: a bound control sends the right line.
func TestApplyWhenEnabled(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	if err := s.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	}); err != nil {
		t.Fatal(err)
	}

	ap := apControl(t)
	// Press then release.
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: false}, nil)

	if len(rec.lines) != 2 {
		t.Fatalf("want a press and a release, got %v", rec.lines)
	}
	if rec.lines[0] != "AP_BTN_Hdg 1\n" {
		t.Errorf("press = %q, want AP_BTN_Hdg 1", rec.lines[0])
	}
	if rec.lines[1] != "AP_BTN_Hdg 0\n" {
		t.Errorf("release = %q, want AP_BTN_Hdg 0", rec.lines[1])
	}
}

// TestSimulateDoesNotSendAndWorksDisabled checks the mapping test: Simulate returns
// what Apply would send without sending anything, and works while sending is off —
// the whole point is to check a mapping without arming it.
func TestSimulateDoesNotSendAndWorksDisabled(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	}); err != nil {
		t.Fatal(err)
	}
	// Sending is deliberately left off.
	if s.Enabled() {
		t.Fatal("the store should start disabled")
	}

	ap := apControl(t)
	got := s.Simulate("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	if len(got) != 1 || got[0] != "AP_BTN_Hdg 1\n" {
		t.Fatalf("Simulate = %v, want [AP_BTN_Hdg 1]", got)
	}
	if len(rec.lines) != 0 {
		t.Fatalf("Simulate must not send anything, sent %v", rec.lines)
	}

	// An unbound control resolves to nothing.
	gear := gearControl(t)
	if got := s.Simulate("F-16C_50", panel.Event{Model: panel.PZ55, Control: gear, Active: true}, nil); len(got) != 0 {
		t.Fatalf("an unbound control should resolve to nothing, got %v", got)
	}
}

// TestApplyForcedSendsWhileDisabled checks the mapping test's override: it sends
// even when the store is disabled, which is how the cockpit is checked live.
func TestApplyForcedSendsWhileDisabled(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	}); err != nil {
		t.Fatal(err)
	}
	ap := apControl(t)

	// The normal path sends nothing while disabled.
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	if len(rec.lines) != 0 {
		t.Fatalf("Apply must respect the switch, sent %v", rec.lines)
	}

	// The forced path sends anyway.
	got := s.ApplyForced("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	if len(got) != 1 || len(rec.lines) != 1 {
		t.Fatalf("ApplyForced should have sent one line, got %v (rec %v)", got, rec.lines)
	}
}

// TestEncoderDirectionSelectsTheSign locks the fix for the wheel bug: both
// directions of an encoder report the same "active" edge, so the sign must come
// from Clockwise, not from Active.
func TestEncoderDirectionSelectsTheSign(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "PITCH_TRIM", Command: "PITCH_TRIM", Interface: "variable_step"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Find the two encoder directions in the real PZ70 definitions.
	var cw, ccw panel.Control
	for _, c := range panel.Controls(panel.PZ70) {
		if c.ID != "PITCH_TRIM" {
			continue
		}
		if c.Clockwise {
			cw = c
		} else {
			ccw = c
		}
	}
	if cw.ID == "" || ccw.ID == "" {
		t.Fatal("PITCH_TRIM should have both directions")
	}

	// A catalogue giving the wheel a suggested step of 3200.
	cat := catalogWithVariableStep(t, "F-16C_50", "PITCH_TRIM", 3200)

	// Both directions arrive Active; only the direction differs.
	got := s.Simulate("F-16C_50", panel.Event{Model: panel.PZ70, Control: cw, Active: true}, cat)
	if len(got) != 1 || got[0] != "PITCH_TRIM 3200\n" {
		t.Fatalf("clockwise = %v, want [+3200]", got)
	}
	got = s.Simulate("F-16C_50", panel.Event{Model: panel.PZ70, Control: ccw, Active: true}, cat)
	if len(got) != 1 || got[0] != "PITCH_TRIM -3200\n" {
		t.Fatalf("counter-clockwise = %v, want [-3200]", got)
	}
}

// catalogWithVariableStep builds a one-control catalogue offering a variable_step
// input with a suggested step.
func catalogWithVariableStep(t *testing.T, module, command string, step int) *biosmeta.Catalog {
	t.Helper()
	sg := t.TempDir()
	dir := biosmeta.JSONDir(sg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"Starter":{"` + command + `":{"category":"Starter","control_type":"dial","identifier":"` +
		command + `","inputs":[{"interface":"variable_step","suggested_step":` + itoa(step) + `}]}}}`
	if err := os.WriteFile(filepath.Join(dir, module+".json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := biosmeta.LoadModule(sg, module)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// TestApplyIgnoresOtherAircraftAndControls checks a binding only fires for its own
// aircraft, panel and control.
func TestApplyIgnoresOtherAircraftAndControls(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	s.SetEnabled(true)
	s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	})

	ap := apControl(t)
	// Wrong aircraft.
	s.Apply("A-10C", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	// Wrong panel model.
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ55, Control: ap, Active: true}, nil)
	// Wrong control.
	gear := gearControl(t)
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: gear, Active: true}, nil)

	if len(rec.lines) != 0 {
		t.Fatalf("nothing should have matched, got %v", rec.lines)
	}
}

// TestSetEnabledWithoutTransport checks turning sending on with no transport is
// refused: the UI must not promise something that cannot happen.
func TestSetEnabledWithoutTransport(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	if err := s.SetEnabled(true); err == nil {
		t.Fatal("enabling without a transport should fail")
	}
	if s.Enabled() {
		t.Fatal("the store should still be disabled after a refused enable")
	}
}

// TestSetProfileRejectsDuplicateControl checks two bindings on one control are
// refused: the second would silently shadow the first.
func TestSetProfileRejectsDuplicateControl(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), (&recorder{}).send)
	err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{
			{Model: panel.PZ70, Control: "AP_BUTTON", Command: "A", Interface: "action"},
			{Model: panel.PZ70, Control: "AP_BUTTON", Command: "B", Interface: "action"},
		},
	})
	if err == nil {
		t.Fatal("a duplicate control should be refused")
	}
}

// TestSetProfileRejectsIncompleteBinding checks a binding missing half of itself
// is refused rather than stored and silently ignored.
func TestSetProfileRejectsIncompleteBinding(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), (&recorder{}).send)
	if err := s.SetProfile(Profile{Aircraft: "X", Bindings: []Binding{{Control: "AP_BUTTON"}}}); err == nil {
		t.Error("a binding without a command should be refused")
	}
	if err := s.SetProfile(Profile{Aircraft: "X", Bindings: []Binding{{Command: "AP_BTN_Hdg"}}}); err == nil {
		t.Error("a binding without a control should be refused")
	}
	if err := s.SetProfile(Profile{Bindings: []Binding{{Control: "A", Command: "B"}}}); err == nil {
		t.Error("a profile without an aircraft should be refused")
	}
}

// TestPersistsAcrossReload checks the bindings survive a restart, and that the
// enabled flag does NOT: sending must be re-armed deliberately.
func TestPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")

	rec := &recorder{}
	s := NewStore(path, rec.send)
	s.SetEnabled(true)
	if err := s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action", Invert: true}},
	}); err != nil {
		t.Fatal(err)
	}

	// A second store reading the same file sees the profile...
	rec2 := &recorder{}
	s2 := NewStore(path, rec2.send)
	got := s2.Profile("F-16C_50")
	if len(got.Bindings) != 1 || got.Bindings[0].Command != "AP_BTN_Hdg" || !got.Bindings[0].Invert {
		t.Fatalf("profile did not persist: %+v", got)
	}
	// ...but starts disabled again.
	if s2.Enabled() {
		t.Fatal("enabling must not survive a restart")
	}
}

// TestInvertFlipsTheState checks a binding marked inverted sends the opposite.
func TestInvertFlipsTheState(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	s.SetEnabled(true)
	s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action", Invert: true}},
	})

	ap := apControl(t)
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil)
	if len(rec.lines) != 1 || rec.lines[0] != "AP_BTN_Hdg 0\n" {
		t.Fatalf("an inverted binding should send 0 on press, got %v", rec.lines)
	}
}

// TestSendFailureDoesNotPanic checks a failing transport is survivable: a command
// that cannot be sent must not take the input handling down.
func TestSendFailureDoesNotPanic(t *testing.T) {
	rec := &recorder{fail: true}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	s.SetEnabled(true)
	s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	})
	ap := apControl(t)
	if sent := s.Apply("F-16C_50", panel.Event{Model: panel.PZ70, Control: ap, Active: true}, nil); len(sent) != 0 {
		t.Fatalf("a failed send should report nothing sent, got %v", sent)
	}
}

// TestApplyUsesCatalogMaximum checks a set_state binding sends the control's real
// maximum when the catalog is available, rather than assuming two positions.
func TestApplyUsesCatalogMaximum(t *testing.T) {
	rec := &recorder{}
	s := NewStore(filepath.Join(t.TempDir(), "mappings.json"), rec.send)
	s.SetEnabled(true)
	s.SetProfile(Profile{
		Aircraft: "F-16C_50",
		Bindings: []Binding{{Model: panel.PZ55, Control: "GEAR_DOWN", Command: "GEAR_LEVER", Interface: "set_state"}},
	})

	cat := &biosmeta.Catalog{}
	// The catalog is built through its loader normally; here a minimal one is
	// enough to exercise the lookup.
	var ctl = biosmeta.Control{
		Identifier:  "GEAR_LEVER",
		ControlType: "selector",
		Inputs:      []biosmeta.Input{{Interface: "set_state", MaxValue: 2}},
	}
	cat = buildCatalog(t, ctl)

	gear := gearControl(t)
	s.Apply("F-16C_50", panel.Event{Model: panel.PZ55, Control: gear, Active: true}, cat)

	if len(rec.lines) != 1 || !strings.Contains(rec.lines[0], " 2") {
		t.Fatalf("set_state should send the catalog maximum (2), got %v", rec.lines)
	}
}

// buildCatalog makes a one-control catalogue without touching the filesystem, by
// going through the same path the loader uses.
func buildCatalog(t *testing.T, ctl biosmeta.Control) *biosmeta.Catalog {
	t.Helper()
	sg := t.TempDir()
	dir := biosmeta.JSONDir(sg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"Gear": {"` + ctl.Identifier + `": {"category":"Gear","control_type":"` +
		ctl.ControlType + `","identifier":"` + ctl.Identifier + `","inputs":[{"interface":"set_state","max_value":2}]}}}`
	if err := os.WriteFile(filepath.Join(dir, "Test.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := biosmeta.LoadModule(sg, "Test")
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// TestSetProfileNotPublishedWhenSaveFails locks the fix: when the write fails, the
// in-memory profile must not change. Publishing before persisting let the UI report
// an error while the manager kept the new bindings, and a panel could act on them.
func TestSetProfileNotPublishedWhenSaveFails(t *testing.T) {
	// A path whose parent is a file, so MkdirAll/WriteFile cannot succeed.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(filepath.Join(blocker, "mappings.json"), (&recorder{}).send)

	err := s.SetProfile(Profile{
		Aircraft: "CUSTOM",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	})
	if err == nil {
		t.Fatal("the save should have failed")
	}
	if got := s.Profile("CUSTOM"); len(got.Bindings) != 0 {
		t.Fatalf("a failed save must not publish the profile, got %d bindings", len(got.Bindings))
	}
}

// TestDeleteProfileRestoredWhenSaveFails checks the same rule for deletion.
func TestDeleteProfileRestoredWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mappings.json")
	rec := &recorder{}
	s := NewStore(path, rec.send)
	if err := s.SetProfile(Profile{
		Aircraft: "CUSTOM",
		Bindings: []Binding{{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_BTN_Hdg", Interface: "action"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Make the target unwritable by turning the parent into a file.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteProfile("CUSTOM"); err == nil {
		t.Fatal("the delete should have failed")
	}
	if got := s.Profile("CUSTOM"); len(got.Bindings) != 1 {
		t.Fatalf("a failed delete must keep the profile, got %d bindings", len(got.Bindings))
	}
}
