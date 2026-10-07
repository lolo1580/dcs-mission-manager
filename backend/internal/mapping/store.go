// Package mapping binds a physical panel control to a DCS-BIOS command.
//
// It is the piece that makes a cockpit work: the PZ55's gear lever drives the
// landing gear, the PZ70's autopilot buttons drive the autopilot. The bindings are
// stored as JSON next to the database, per aircraft, and — deliberately — sending
// is off unless the operator turns it on for the session.
package mapping

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/panel"
)

// Binding is one panel control bound to one DCS-BIOS command.
type Binding struct {
	// Model is the panel the control belongs to (pz55, pz70).
	Model panel.Model `json:"model"`
	// Control is the panel control id (GEAR_DOWN, AP_BUTTON…).
	Control string `json:"control"`
	// Command is the DCS-BIOS identifier to send (GEAR_LEVER, AP_BTN_Hdg…).
	Command string `json:"command"`
	// Interface is how it is sent (fixed_step, set_state, action…).
	Interface string `json:"interface"`
	// Invert flips the active state, for a switch whose "on" means the opposite of
	// the command's "on".
	Invert   bool   `json:"invert,omitempty"`
	Mode     string `json:"mode,omitempty"`
	StateOn  *int   `json:"state_on,omitempty"`
	StateOff *int   `json:"state_off,omitempty"`
	// PulseReset releases a spring-centred cockpit switch after an encoder pulse.
	PulseReset *int `json:"pulse_reset,omitempty"`
}

// Profile is the set of bindings for one aircraft.
type Profile struct {
	// Aircraft is the module name the bindings were made for (F-16C_50).
	Aircraft string    `json:"aircraft"`
	Bindings []Binding `json:"bindings"`
	// Outputs drive the panels' indicators (gear lights, autopilot button LEDs)
	// from DCS-BIOS-exported values. Empty means no indicator is driven. The key
	// is always written (no omitempty), so an empty list is distinguishable from a
	// file that predates outputs — see load.
	Outputs []OutputBinding `json:"outputs"`
	// Displays drive the PZ70 LCD. The key is always written (no omitempty), so an
	// empty list is distinguishable from a file that predates displays — see load.
	Displays []DisplayBinding `json:"displays"`
}

// DisplayBinding shows an exported DCS-BIOS value on one PZ70 LCD line, for one
// position of the ALT/VS/IAS/HDG/CRS selector.
type DisplayBinding struct {
	// Model is the panel, always pz70 for now.
	Model panel.Model `json:"model"`
	// Mode is the selector position this binding answers to (ALT, VS, IAS, HDG,
	// CRS), case-insensitive.
	Mode string `json:"mode"`
	// Line is "upper" or "lower".
	Line string `json:"line"`
	// Command is the DCS-BIOS control whose exported value is shown.
	Command string `json:"command"`
	// Export is the index of the output to read within that control.
	Export int `json:"export"`
	// Scale and Offset turn the raw value into the displayed number
	// (displayed = round(raw*Scale + Offset)). Scale 0 is treated as 1.
	Scale  float64 `json:"scale,omitempty"`
	Offset float64 `json:"offset,omitempty"`
	// Unit is a descriptive label (feet, knots, degrees) shown in the UI only; it
	// is never printed on the numeric LCD.
	Unit string `json:"unit,omitempty"`
}

// OutputBinding lights one panel indicator from one exported DCS-BIOS control.
// LED only: the PZ55's landing-gear lights and the PZ70's autopilot button lights.
type OutputBinding struct {
	// Model is the panel whose indicator is lit (pz55, pz70).
	Model panel.Model `json:"model"`
	// Target is the indicator id (LIGHT_GEAR_UPPER, LIGHT_AP…).
	Target string `json:"target"`
	// Command is the DCS-BIOS control whose exported value drives it (LIGHT_GEAR_N).
	Command string `json:"command"`
	// Color is the colour shown when the exported value is non-zero (green, red,
	// yellow). The PZ55's indicators are bicolour; the PZ70's are single-colour.
	Color string       `json:"color,omitempty"`
	Rules []OutputRule `json:"rules,omitempty"`
}

// OutputRule is evaluated in order; the first match determines the light colour.
type OutputRule struct {
	Command  string `json:"command"`
	Export   int    `json:"export"`
	Operator string `json:"operator"`
	Value    int    `json:"value"`
	Color    string `json:"color"`
}

// Key identifies the control a binding is attached to.
func (b Binding) Key() string {
	key := string(b.Model) + "/" + b.Control
	if b.Mode != "" {
		key += "/" + strings.ToUpper(b.Mode)
	}
	return key
}

// Store holds the profiles and applies them. It is safe for concurrent use.
type Store struct {
	path string

	mu sync.RWMutex
	// profiles is keyed by aircraft.
	profiles map[string]*Profile
	// enabled gates sending. Off until the operator turns it on, and off again
	// whenever the manager restarts: sending commands into a live cockpit is not
	// something that should survive a restart unnoticed.
	enabled bool
	// outputsEnabled gates the panels' outputs (LEDs, and later the LCD). It is a
	// separate switch from `enabled`: driving a cockpit from DCS-BIOS is a read-only
	// action, and must not require the far more dangerous power to send the pilot's
	// inputs into the aircraft. Off until the operator turns it on, off on restart.
	outputsEnabled bool
	// send is the transport, injected so the store can be tested without DCS-BIOS.
	send func(command string) error
}

// NewStore creates a store backed by a JSON file. send is the function that
// actually transmits a command line; it may be nil, in which case enabling the
// store is refused (there is nothing to send with).
//
// Built-in starter profiles are seeded only when the file does not exist yet, so
// a new cockpit has working bindings out of the box. An existing file is never
// overwritten or seeded over, even if it is empty or unreadable: a corrupt
// binding file may still hold the operator's bindings, and silently replacing it
// with the defaults would destroy them.
func NewStore(path string, send func(command string) error) *Store {
	s := &Store{
		path:     path,
		profiles: map[string]*Profile{},
		send:     send,
	}
	switch s.load() {
	case fileAbsent:
		for _, p := range Starter() {
			p := p
			s.profiles[p.Aircraft] = &p
		}
		// A failure to seed is not fatal: the profiles are a convenience, and the
		// in-memory set is already usable even if the file cannot be written.
		_ = s.saveLocked()
	case filePresent:
		// The file exists: respect it, do not seed over the operator's edits.
	case fileUnreadable:
		// A file that exists but cannot be read (invalid JSON, I/O error) must be
		// preserved, not replaced. Leave the store empty; the operator can fix or
		// rebuild it in the UI.
	}
	return s
}

// Enabled reports whether commands are currently sent.
func (s *Store) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// SetEnabled turns sending on or off. Turning it on without a transport fails,
// so the UI cannot promise something that cannot happen.
func (s *Store) SetEnabled(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on && s.send == nil {
		return fmt.Errorf("mapping: no DCS-BIOS transport configured")
	}
	s.enabled = on
	return nil
}

// Aircraft returns the names that have a profile, sorted.
func (s *Store) Aircraft() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.profiles))
	for name := range s.profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// OutputsEnabled reports whether the panels' outputs (LEDs, LCD) are driven.
func (s *Store) OutputsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outputsEnabled
}

// SetOutputsEnabled turns the panel outputs (LEDs, LCD) on or off. Unlike
// SetEnabled it needs no transport: outputs only read DCS-BIOS and write the
// panels, they never command the aircraft.
func (s *Store) SetOutputsEnabled(on bool) {
	s.mu.Lock()
	s.outputsEnabled = on
	s.mu.Unlock()
}

// Profile returns a copy of an aircraft's profile, empty when none exists.
func (s *Store) Profile(aircraft string) Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.profiles[aircraft]
	if p == nil {
		return Profile{Aircraft: aircraft, Bindings: []Binding{}}
	}
	out := Profile{
		Aircraft: aircraft,
		Bindings: make([]Binding, len(p.Bindings)),
		Outputs:  make([]OutputBinding, len(p.Outputs)),
		Displays: make([]DisplayBinding, len(p.Displays)),
	}
	copy(out.Bindings, p.Bindings)
	copy(out.Outputs, p.Outputs)
	for i := range out.Outputs {
		out.Outputs[i].Rules = append([]OutputRule(nil), p.Outputs[i].Rules...)
	}
	copy(out.Displays, p.Displays)
	return out
}

// SetProfile replaces an aircraft's bindings and persists them.
func (s *Store) SetProfile(p Profile) error {
	if p.Aircraft == "" {
		return fmt.Errorf("mapping: a profile needs an aircraft")
	}
	if !validToken(p.Aircraft, 64) {
		return fmt.Errorf("mapping: invalid aircraft name")
	}
	// Reject two bindings on the same control: the second would silently shadow
	// the first, and the panel would look broken.
	seen := map[string]bool{}
	for i, b := range p.Bindings {
		for _, value := range []*int{b.StateOn, b.StateOff, b.PulseReset} {
			if value != nil && (b.Interface != "set_state" || *value < 0 || *value > 65535) {
				return fmt.Errorf("mapping: custom positions require set_state and values 0..65535")
			}
		}
		if b.PulseReset != nil && b.Control != "LCD_WHEEL" && b.Control != "PITCH_TRIM" {
			return fmt.Errorf("mapping: pulse reset requires an encoder")
		}
		if b.Model != panel.PZ55 && b.Model != panel.PZ70 {
			return fmt.Errorf("mapping: unknown input model")
		}
		switch b.Interface {
		case "action", "set_state", "fixed_step", "variable_step":
		default:
			return fmt.Errorf("mapping: unknown command interface")
		}
		if b.Control == "" || b.Command == "" {
			return fmt.Errorf("mapping: a binding needs both a control and a command")
		}
		// The profile comes from the (unauthenticated on loopback) API and is
		// both persisted to disk and forwarded to DCS-BIOS, so its fields are
		// bounded and restricted to a conservative charset: a caller cannot grow
		// the file unboundedly nor push arbitrary bytes at the command channel.
		if i >= maxBindings {
			return fmt.Errorf("mapping: too many bindings (max %d)", maxBindings)
		}
		if !validToken(b.Control, 64) || !validToken(b.Command, 96) || !validToken(b.Interface, 32) {
			return fmt.Errorf("mapping: invalid characters in binding %q", b.Control)
		}
		k := b.Key()
		if b.Mode != "" && (b.Model != panel.PZ70 || b.Control != "LCD_WHEEL" || !validMode(b.Mode)) {
			return fmt.Errorf("mapping: mode is only valid for the PZ70 LCD wheel")
		}
		if seen[k] {
			return fmt.Errorf("mapping: %s is bound twice", k)
		}
		seen[k] = true
	}
	if p.Bindings == nil {
		p.Bindings = []Binding{}
	}
	// Output bindings are validated the same way: bounded, safe tokens, one binding
	// per indicator so the second cannot silently shadow the first.
	seenOut := map[string]bool{}
	for i, o := range p.Outputs {
		if len(o.Rules) > 16 {
			return fmt.Errorf("mapping: too many LED rules")
		}
		for _, rule := range o.Rules {
			if !validToken(rule.Command, 96) || rule.Export < 0 || rule.Export > 64 || !validColor(rule.Color) ||
				(o.Model == panel.PZ70 && rule.Color != "green" && rule.Color != "off") {
				return fmt.Errorf("mapping: invalid LED rule")
			}
			switch rule.Operator {
			case "eq", "ne", "gt", "lt", "ge", "le":
			default:
				return fmt.Errorf("mapping: invalid LED comparison")
			}
		}
		if o.Target == "" || o.Command == "" {
			return fmt.Errorf("mapping: an output needs both a target and a command")
		}
		if i >= maxBindings {
			return fmt.Errorf("mapping: too many outputs (max %d)", maxBindings)
		}
		if !validToken(o.Target, 64) || !validToken(o.Command, 96) || (o.Color != "" && !validColor(o.Color)) {
			return fmt.Errorf("mapping: invalid characters in output %q", o.Target)
		}
		if o.Model != panel.PZ55 && o.Model != panel.PZ70 {
			return fmt.Errorf("mapping: unknown output model %q", o.Model)
		}
		if !panel.ValidTarget(o.Model, o.Target) {
			return fmt.Errorf("mapping: %s is not a %s indicator", o.Target, o.Model)
		}
		k := string(o.Model) + "/" + o.Target
		if seenOut[k] {
			return fmt.Errorf("mapping: %s is driven twice", k)
		}
		seenOut[k] = true
	}
	if p.Outputs == nil {
		p.Outputs = []OutputBinding{}
	}
	// Displays are validated the same way, plus the mode/line/export rules.
	seenDisp := map[string]bool{}
	for i, dp := range p.Displays {
		if i >= maxBindings {
			return fmt.Errorf("mapping: too many displays (max %d)", maxBindings)
		}
		if dp.Model != panel.PZ70 {
			return fmt.Errorf("mapping: displays are PZ70 only")
		}
		if !validToken(dp.Mode, 8) || !validToken(dp.Line, 8) {
			return fmt.Errorf("mapping: a display needs a mode and a line")
		}
		if !panel.ValidDisplayMode(dp.Mode) {
			return fmt.Errorf("mapping: %q is not a PZ70 selector mode", dp.Mode)
		}
		if !panel.ValidDisplayLine(dp.Line) {
			return fmt.Errorf("mapping: %q is not an LCD line", dp.Line)
		}
		if dp.Command == "" || !validToken(dp.Command, 96) {
			return fmt.Errorf("mapping: a display needs a source command")
		}
		if dp.Export < 0 {
			return fmt.Errorf("mapping: display export index must not be negative")
		}
		if math.IsNaN(dp.Scale) || math.IsInf(dp.Scale, 0) || math.IsNaN(dp.Offset) || math.IsInf(dp.Offset, 0) {
			return fmt.Errorf("mapping: display conversion must be finite")
		}
		k := strings.ToUpper(dp.Mode) + "/" + strings.ToLower(dp.Line)
		if seenDisp[k] {
			return fmt.Errorf("mapping: %s line is configured twice for %s", dp.Line, dp.Mode)
		}
		seenDisp[k] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Persist first, then publish: a failed write must not leave the in-memory
	// state changed, or the UI would report an error while the manager behaves as
	// if the save had succeeded (and a panel could act on the unsaved profile).
	previous, had := s.profiles[p.Aircraft]
	s.profiles[p.Aircraft] = &p
	if err := s.saveLocked(); err != nil {
		if had {
			s.profiles[p.Aircraft] = previous
		} else {
			delete(s.profiles, p.Aircraft)
		}
		return err
	}
	return nil
}

// Binding/profile limits, so the persisted file and the DCS-BIOS command channel
// stay bounded whatever the API is fed.
const maxBindings = 512

// validToken reports whether s is non-empty, no longer than max, and made only
// of characters safe in an identifier-like token. It is deliberately strict:
// these strings are persisted and some reach DCS-BIOS.
func validToken(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_', c == '-', c == '.', c == '/', c == ':':
		default:
			return false
		}
	}
	return true
}

// DeleteProfile removes an aircraft's bindings.
func (s *Store) DeleteProfile(aircraft string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, had := s.profiles[aircraft]
	delete(s.profiles, aircraft)
	if err := s.saveLocked(); err != nil {
		if had {
			s.profiles[aircraft] = previous
		}
		return err
	}
	return nil
}

// Apply handles one panel input event, sending every command that matches. The
// catalog, when provided, supplies the control's declared maximum so a set_state
// binding sends the right position; without it, a two-position command is assumed.
//
// It returns the commands sent, so a caller (a test, or the live monitor) can see
// what happened. Nothing is sent when the store is disabled.
func (s *Store) Apply(aircraft string, ev panel.Event, cat *biosmeta.Catalog) []string {
	s.mu.RLock()
	p := s.profiles[aircraft]
	enabled := s.enabled
	send := s.send
	s.mu.RUnlock()

	if p == nil || !enabled || send == nil {
		return nil
	}

	var sent []string
	for _, command := range commandPlan(p, ev, cat) {
		if command.delay > 0 {
			time.Sleep(command.delay)
		}
		if err := send(command.line); err != nil {
			continue
		}
		sent = append(sent, command.line)
	}
	return sent
}

// ApplyForced sends the commands a panel input matches even when sending is off.
// It exists only for the live mapping test, which deliberately overrides the
// switch so the cockpit can be checked without arming it.
func (s *Store) ApplyForced(aircraft string, ev panel.Event, cat *biosmeta.Catalog) []string {
	s.mu.RLock()
	p := s.profiles[aircraft]
	send := s.send
	s.mu.RUnlock()
	if p == nil || send == nil {
		return nil
	}
	var sent []string
	for _, command := range commandPlan(p, ev, cat) {
		if command.delay > 0 {
			time.Sleep(command.delay)
		}
		if err := send(command.line); err != nil {
			continue
		}
		sent = append(sent, command.line)
	}
	return sent
}

// Simulate returns the commands a panel input would send, without sending any.
// It is what the Panels test view uses to show whether a binding is right: unlike
// Apply it ignores whether sending is enabled, because the point is to check the
// mapping, not to arm it.
func (s *Store) Simulate(aircraft string, ev panel.Event, cat *biosmeta.Catalog) []string {
	s.mu.RLock()
	p := s.profiles[aircraft]
	s.mu.RUnlock()
	if p == nil {
		return nil
	}
	return commandsFor(p, ev, cat)
}

// commandsFor builds the DCS-BIOS lines an input matches, in binding order. It
// does not send anything, so Apply and Simulate share one definition of "what
// this control is bound to".
func commandsFor(p *Profile, ev panel.Event, cat *biosmeta.Catalog) []string {
	var out []string
	for _, command := range commandPlan(p, ev, cat) {
		out = append(out, command.line)
	}
	return out
}

type plannedCommand struct {
	line  string
	delay time.Duration
}

func commandPlan(p *Profile, ev panel.Event, cat *biosmeta.Catalog) []plannedCommand {
	var out []plannedCommand
	contextual := false
	for _, b := range p.Bindings {
		if b.Model == ev.Model && strings.EqualFold(b.Control, ev.Control.ID) && b.Mode != "" && strings.EqualFold(b.Mode, ev.Mode) {
			contextual = true
		}
	}
	for _, b := range p.Bindings {
		if b.Model != ev.Model || !strings.EqualFold(b.Control, ev.Control.ID) {
			continue
		}
		if b.Mode != "" && !strings.EqualFold(b.Mode, ev.Mode) || b.Mode == "" && contextual {
			continue
		}
		// A selector's old position going inactive is not its new position.
		// The spring-centred flaps rocker must not undo a positional command
		// when released. Action controls still receive their release.
		id := strings.ToUpper(ev.Control.ID)
		position := strings.HasPrefix(id, "ENGINE_") || strings.HasPrefix(id, "GEAR_") || strings.HasPrefix(id, "KNOB_")
		flaps := id == "FLAPS_UP" || id == "FLAPS_DOWN"
		if !ev.Active && (position || (flaps && b.Interface != string(biosmeta.Action)) ||
			(ev.Control.Kind == panel.Button && (b.Interface == string(biosmeta.FixedStep) || b.Interface == string(biosmeta.VariableStep)))) {
			continue
		}
		active := ev.Active
		if b.Invert {
			active = !active
		}

		// A binding stores an interface name, not the whole control, so a profile
		// stays valid when DCS-BIOS metadata changes. The maximum and the suggested
		// step come from the catalog when it is known.
		max, step := 0, 0
		if cat != nil {
			if ctl, ok := cat.ByID(b.Command); ok {
				if err := biosmeta.Validate(ctl, biosmeta.Interface(b.Interface)); err != nil {
					continue // the binding no longer fits: skip rather than guess
				}
				for _, in := range ctl.Inputs {
					if in.Interface == string(biosmeta.SetState) && in.MaxValue > max {
						max = in.MaxValue
					}
					if in.Interface == string(biosmeta.VariableStep) && in.SuggestedStep > 0 {
						step = in.SuggestedStep
					}
				}
			}
		}

		// An encoder (a wheel) is decoded as a pulse whose direction, not whose
		// active bit, decides the sign: the two directions both report "active".
		var value int
		var ok bool
		if ev.Control.Kind == panel.EncoderPulse {
			direction := ev.Control.Clockwise
			if b.Invert {
				direction = !direction
			}
			value, ok = biosmeta.ArgForEncoder(biosmeta.Interface(b.Interface), step, max, direction)
			active = direction
		} else {
			value, ok = biosmeta.ArgForInterface(biosmeta.Interface(b.Interface), max, active)
		}
		if !ok {
			continue
		}
		if b.PulseReset != nil && (ev.Control.Kind != panel.EncoderPulse || (cat != nil && *b.PulseReset > max)) {
			continue
		}
		if b.Interface == string(biosmeta.SetState) {
			custom := b.StateOff
			if active {
				custom = b.StateOn
			}
			if custom != nil {
				if cat != nil && *custom > max {
					continue
				}
				value = *custom
			}
		}
		if b.Interface == string(biosmeta.FixedStep) {
			argument := "INC"
			if value == 2 {
				argument = "DEC"
			}
			out = append(out, plannedCommand{line: fmt.Sprintf("%s %s\n", b.Command, argument)})
		} else {
			out = append(out, plannedCommand{line: fmt.Sprintf("%s %d\n", b.Command, value)})
			if b.PulseReset != nil {
				out = append(out, plannedCommand{line: fmt.Sprintf("%s %d\n", b.Command, *b.PulseReset), delay: 50 * time.Millisecond})
			}
		}
	}
	return out
}

// loadResult says how reading the binding file went, which is what lets NewStore
// seed the starter profiles on a fresh install without ever touching a file that
// already exists.
type loadResult int

const (
	// fileAbsent means there is no binding file yet: a fresh install.
	fileAbsent loadResult = iota
	// filePresent means the file was read and parsed.
	filePresent
	// fileUnreadable means the file exists but could not be read or parsed. Its
	// bytes are left alone.
	fileUnreadable
)

// load reads the JSON file. A missing file is a fresh install; an unreadable one
// is left untouched, because it may still hold the operator's bindings.
func (s *Store) load() loadResult {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileAbsent
		}
		return fileUnreadable
	}
	var file struct {
		Profiles []Profile `json:"profiles"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return fileUnreadable
	}
	// A file written before outputs existed has no `outputs` key, so its profiles
	// decode with a nil slice. Fill those from the starters of the same aircraft so
	// the LEDs work without the operator redoing their bindings. An explicit `[]`
	// (a profile the operator cleared) stays empty. Displays are handled the same
	// way.
	starterOut := map[string][]OutputBinding{}
	starterDisp := map[string][]DisplayBinding{}
	for _, p := range Starter() {
		if len(p.Outputs) > 0 {
			starterOut[p.Aircraft] = p.Outputs
		}
		if len(p.Displays) > 0 {
			starterDisp[p.Aircraft] = p.Displays
		}
	}
	for i := range file.Profiles {
		p := file.Profiles[i]
		if p.Outputs == nil {
			if outs, ok := starterOut[p.Aircraft]; ok {
				p.Outputs = outs
			} else {
				p.Outputs = []OutputBinding{}
			}
		}
		if p.Displays == nil {
			if disps, ok := starterDisp[p.Aircraft]; ok {
				p.Displays = disps
			} else {
				p.Displays = []DisplayBinding{}
			}
		}
		s.profiles[p.Aircraft] = &p
	}
	return filePresent
}

// saveLocked writes the file. The caller must hold the mutex.
func (s *Store) saveLocked() error {
	profiles := make([]Profile, 0, len(s.profiles))
	aircraft := make([]string, 0, len(s.profiles))
	for name := range s.profiles {
		aircraft = append(aircraft, name)
	}
	sort.Strings(aircraft)
	for _, name := range aircraft {
		profiles = append(profiles, *s.profiles[name])
	}

	data, err := json.MarshalIndent(struct {
		Profiles []Profile `json:"profiles"`
	}{Profiles: profiles}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	// Write to a temporary file then rename, so a crash mid-write cannot leave a
	// truncated file where the bindings were.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
