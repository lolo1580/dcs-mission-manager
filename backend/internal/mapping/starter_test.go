package mapping

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/panel"
)

// TestStarterProfilesAreWellFormed checks the built-in profiles are usable: a
// named aircraft, at least one binding each, every control a real panel control,
// and no control bound twice.
func TestStarterProfilesAreWellFormed(t *testing.T) {
	profiles := Starter()
	if len(profiles) == 0 {
		t.Fatal("expected starter profiles")
	}
	for _, p := range profiles {
		if p.Aircraft == "" {
			t.Fatal("a starter profile has no aircraft")
		}
		if len(p.Bindings) == 0 {
			t.Errorf("%s: no bindings", p.Aircraft)
		}
		controls := map[panel.Model]map[string]bool{}
		for _, m := range []panel.Model{panel.PZ55, panel.PZ70} {
			set := map[string]bool{}
			for _, c := range panel.Controls(m) {
				set[c.ID] = true
			}
			controls[m] = set
		}
		seen := map[string]bool{}
		for _, b := range p.Bindings {
			if b.Command == "" || b.Interface == "" {
				t.Errorf("%s: %s needs a command and an interface", p.Aircraft, b.Control)
			}
			if !controls[b.Model][b.Control] {
				t.Errorf("%s: %q is not a %s control", p.Aircraft, b.Control, b.Model)
			}
			if seen[b.Key()] {
				t.Errorf("%s: %s is bound twice", p.Aircraft, b.Key())
			}
			seen[b.Key()] = true
		}

		// Outputs go the other way: an indicator id, a source command, no interface.
		seenOut := map[string]bool{}
		for _, o := range p.Outputs {
			if o.Command == "" {
				t.Errorf("%s: %s needs a source command", p.Aircraft, o.Target)
			}
			if !panel.ValidTarget(o.Model, o.Target) {
				t.Errorf("%s: %q is not a %s indicator", p.Aircraft, o.Target, o.Model)
			}
			if k := string(o.Model) + "/" + o.Target; seenOut[k] {
				t.Errorf("%s: %s is driven twice", p.Aircraft, k)
			} else {
				seenOut[k] = true
			}
		}

		// Displays: mode, line, source; no duplicate mode+line.
		seenDisp := map[string]bool{}
		for _, d := range p.Displays {
			if d.Command == "" {
				t.Errorf("%s: a display has no source", p.Aircraft)
			}
			if !panel.ValidDisplayMode(d.Mode) {
				t.Errorf("%s: %q is not a selector mode", p.Aircraft, d.Mode)
			}
			if !panel.ValidDisplayLine(d.Line) {
				t.Errorf("%s: %q is not an LCD line", p.Aircraft, d.Line)
			}
			if k := d.Mode + "/" + d.Line; seenDisp[k] {
				t.Errorf("%s: %s line is set twice for %s", p.Aircraft, d.Line, d.Mode)
			} else {
				seenDisp[k] = true
			}
		}
	}
}

// TestStarterProfilesMatchDCSBIOS checks every built-in command exists for its
// aircraft in the machine's real DCS-BIOS metadata, and accepts the interface the
// binding uses. It is skipped when DCS-BIOS is not installed, so it never fails a
// machine without it, but on a cockpit it is the test that catches a typo in a
// shipped profile — the one mistake that would make a panel silently do nothing.
func TestStarterProfilesMatchDCSBIOS(t *testing.T) {
	jsonDir := filepath.Join(os.Getenv("USERPROFILE"), "Saved Games", "DCS", "Scripts", "DCS-BIOS", "doc", "json")
	if _, err := os.Stat(jsonDir); err != nil {
		t.Skipf("no DCS-BIOS metadata at %s", jsonDir)
	}
	savedGames := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(jsonDir))))

	for _, p := range Starter() {
		cat, err := biosmeta.LoadModule(savedGames, p.Aircraft)
		if err != nil {
			t.Errorf("%s: no metadata: %v", p.Aircraft, err)
			continue
		}
		cat = biosmeta.WithPanelPlugin(cat)
		for _, b := range p.Bindings {
			ctl, ok := cat.ByID(b.Command)
			if !ok {
				t.Errorf("%s: %s is not a control of the aircraft", p.Aircraft, b.Command)
				continue
			}
			iface := biosmeta.Interface(b.Interface)
			if err := biosmeta.Validate(ctl, iface); err != nil {
				t.Errorf("%s/%s: %v", p.Aircraft, b.Command, err)
				continue
			}
			if _, ok := biosmeta.ArgForInput(ctl, iface, true); !ok {
				t.Errorf("%s/%s: no argument for %s", p.Aircraft, b.Command, iface)
			}
		}
		// An output binding reads a control's exported value, so the control must
		// exist and carry an output to read.
		for _, o := range p.Outputs {
			for _, rule := range o.Rules {
				ctl, ok := cat.ByID(rule.Command)
				if !ok || rule.Export < 0 || rule.Export >= len(ctl.Outputs) || ctl.Outputs[rule.Export].Type != "integer" {
					t.Errorf("%s: invalid LED rule source %s", p.Aircraft, rule.Command)
				}
			}
			ctl, ok := cat.ByID(o.Command)
			if !ok {
				t.Errorf("%s: %s is not a control of the aircraft", p.Aircraft, o.Command)
				continue
			}
			if len(ctl.Outputs) == 0 {
				t.Errorf("%s: %s exports no value to read", p.Aircraft, o.Command)
			}
		}
		// A display reads one exported output by index; it must exist.
		for _, d := range p.Displays {
			ctl, ok := cat.ByID(d.Command)
			if !ok {
				t.Errorf("%s: display source %s is not a control of the aircraft", p.Aircraft, d.Command)
				continue
			}
			if d.Export < 0 || d.Export >= len(ctl.Outputs) {
				t.Errorf("%s: display source %s has no output %d", p.Aircraft, d.Command, d.Export)
			}
		}
	}
}

// TestNewStoreSeedsStarterProfilesOnFreshFile locks the out-of-the-box behaviour:
// a first run with no binding file seeds the ready-made profiles.
func TestNewStoreSeedsStarterProfilesOnFreshFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	s := NewStore(path, (&recorder{}).send)

	for _, p := range Starter() {
		got := s.Profile(p.Aircraft)
		if len(got.Bindings) != len(p.Bindings) {
			t.Errorf("%s: seeded %d bindings, want %d", p.Aircraft, len(got.Bindings), len(p.Bindings))
		}
	}

	// The file was written, so the profiles survive a restart.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the seeded file should exist: %v", err)
	}

	// Seeding must not arm sending: driving a live cockpit stays a deliberate act.
	if s.Enabled() {
		t.Fatal("seeding the profiles must not enable sending")
	}
}

// TestNewStoreDoesNotSeedAnExistingFile checks an operator's file — even one with
// no profiles at all — is never overwritten with the starters.
func TestNewStoreDoesNotSeedAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path, (&recorder{}).send)
	for _, p := range Starter() {
		if got := s.Profile(p.Aircraft); len(got.Bindings) != 0 {
			t.Errorf("%s should not have been seeded into an existing file", p.Aircraft)
		}
	}
}

// TestNewStorePreservesAnUnreadableFile locks the rule that a corrupt binding
// file is never replaced by the starters: its bytes may still hold the operator's
// bindings, and a silent overwrite would destroy them.
func TestNewStorePreservesAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	// Invalid JSON, as a hand-edit or a truncated write would leave.
	const broken = `{"profiles":[{"aircraft":"CUSTOM","bindings":`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path, (&recorder{}).send)

	// The file is byte-for-byte unchanged.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file should still exist: %v", err)
	}
	if string(got) != broken {
		t.Fatalf("a corrupt file must be preserved, got %q", got)
	}
	// Nothing was seeded into memory either.
	for _, p := range Starter() {
		if prof := s.Profile(p.Aircraft); len(prof.Bindings) != 0 {
			t.Errorf("%s should not be seeded over an unreadable file", p.Aircraft)
		}
	}
}

// TestNewStoreBackfillsOutputsIntoAnOlderFile locks the upgrade path from a file
// written before LED outputs existed: its profiles have no `outputs` key, and the
// starter outputs of the same aircraft must be filled in so the LEDs work without
// the operator redoing their bindings. An explicit empty list stays empty.
func TestNewStoreBackfillsOutputsIntoAnOlderFile(t *testing.T) {
	// A profile as an older build would have written it: no outputs key at all.
	old := `{"profiles":[{"aircraft":"F-16C_50","bindings":[]}]}`
	path := filepath.Join(t.TempDir(), "mappings.json")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path, (&recorder{}).send)
	if got := len(s.Profile("F-16C_50").Outputs); got == 0 {
		t.Fatal("an older file should have been backfilled with the starter outputs")
	}

	// An explicit empty list is a deliberate "no LED" and is left alone.
	cleared := `{"profiles":[{"aircraft":"F-16C_50","bindings":[],"outputs":[]}]}`
	path2 := filepath.Join(t.TempDir(), "mappings.json")
	if err := os.WriteFile(path2, []byte(cleared), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := NewStore(path2, (&recorder{}).send)
	if got := len(s2.Profile("F-16C_50").Outputs); got != 0 {
		t.Fatalf("an explicit empty outputs list was overwritten (%d)", got)
	}
}

// TestStarterGearLeverDirections locks the one direction that is easy to get
// backwards and expensive to debug in the air: the gear lever's UP position must
// send the command's 0 (gear up), and DOWN its maximum (gear down).
func TestStarterGearLeverDirections(t *testing.T) {
	rec := &recorder{}
	path := filepath.Join(t.TempDir(), "mappings.json")
	s := NewStore(path, rec.send)
	if err := s.SetEnabled(true); err != nil {
		t.Fatal(err)
	}

	up := panel.Control{ID: "GEAR_UP", Kind: panel.Toggle}
	down := panel.Control{ID: "GEAR_DOWN", Kind: panel.Toggle}

	for _, p := range Starter() {
		commands := map[string]bool{}
		for _, b := range p.Bindings {
			if b.Control == "GEAR_UP" || b.Control == "GEAR_DOWN" {
				commands[b.Command] = true
			}
		}
		if len(commands) == 0 {
			continue
		}
		cat := catalogWithSetState(t, p.Aircraft, commands, 1)

		rec.lines = nil
		s.Apply(p.Aircraft, panel.Event{Model: panel.PZ55, Control: up, Active: true}, cat)
		if len(rec.lines) != 1 {
			t.Fatalf("%s: gear up sent %v, want one command", p.Aircraft, rec.lines)
		}
		upSuffix := " 0\n"
		if p.Aircraft == "A-10C" {
			upSuffix = " 1\n"
		}
		if !strings.HasSuffix(rec.lines[0], upSuffix) {
			t.Errorf("%s: gear up sent %q, want position 0", p.Aircraft, rec.lines[0])
		}

		rec.lines = nil
		s.Apply(p.Aircraft, panel.Event{Model: panel.PZ55, Control: down, Active: true}, cat)
		if len(rec.lines) != 1 {
			t.Fatalf("%s: gear down sent %v, want one command", p.Aircraft, rec.lines)
		}
		downSuffix := " 1\n"
		if p.Aircraft == "A-10C" {
			downSuffix = " 0\n"
		}
		if !strings.HasSuffix(rec.lines[0], downSuffix) {
			t.Errorf("%s: gear down sent %q, want position 1", p.Aircraft, rec.lines[0])
		}
	}
}

// catalogWithSetState builds a one-file catalogue offering a set_state input for
// each command, so a starter profile can be applied without a DCS-BIOS install.
func catalogWithSetState(t *testing.T, module string, commands map[string]bool, maxValue int) *biosmeta.Catalog {
	t.Helper()
	sg := t.TempDir()
	dir := biosmeta.JSONDir(sg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"Starter":{`
	first := true
	for cmd := range commands {
		if !first {
			raw += ","
		}
		first = false
		raw += `"` + cmd + `":{"category":"Starter","control_type":"selector","identifier":"` +
			cmd + `","inputs":[{"interface":"set_state","max_value":` +
			itoa(maxValue) + `}]}`
	}
	raw += `}}`
	path := filepath.Join(dir, module+".json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := biosmeta.LoadModule(sg, module)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
