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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	Invert bool `json:"invert,omitempty"`
}

// Profile is the set of bindings for one aircraft.
type Profile struct {
	// Aircraft is the module name the bindings were made for (F-16C_50).
	Aircraft string    `json:"aircraft"`
	Bindings []Binding `json:"bindings"`
}

// Key identifies the control a binding is attached to.
func (b Binding) Key() string {
	return string(b.Model) + "/" + b.Control
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

// Profile returns a copy of an aircraft's profile, empty when none exists.
func (s *Store) Profile(aircraft string) Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.profiles[aircraft]
	if p == nil {
		return Profile{Aircraft: aircraft, Bindings: []Binding{}}
	}
	out := Profile{Aircraft: aircraft, Bindings: make([]Binding, len(p.Bindings))}
	copy(out.Bindings, p.Bindings)
	return out
}

// SetProfile replaces an aircraft's bindings and persists them.
func (s *Store) SetProfile(p Profile) error {
	// Reject two bindings on the same control: the second would silently shadow
	// the first, and the panel would look broken.
	seen := map[string]bool{}
	for _, b := range p.Bindings {
		if b.Control == "" || b.Command == "" {
			return fmt.Errorf("mapping: a binding needs both a control and a command")
		}
		k := b.Key()
		if seen[k] {
			return fmt.Errorf("mapping: %s is bound twice", k)
		}
		seen[k] = true
	}
	if p.Bindings == nil {
		p.Bindings = []Binding{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Aircraft == "" {
		return fmt.Errorf("mapping: a profile needs an aircraft")
	}
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
	for _, b := range p.Bindings {
		if b.Model != ev.Model || !strings.EqualFold(b.Control, ev.Control.ID) {
			continue
		}
		active := ev.Active
		if b.Invert {
			active = !active
		}

		// A binding stores an interface name, not the whole control, so a profile
		// stays valid when DCS-BIOS metadata changes. The maximum comes from the
		// catalog when it is known.
		max := 0
		if cat != nil {
			if ctl, ok := cat.ByID(b.Command); ok {
				if err := biosmeta.Validate(ctl, biosmeta.Interface(b.Interface)); err != nil {
					continue // the binding no longer fits: skip rather than guess
				}
				for _, in := range ctl.Inputs {
					if in.Interface == string(biosmeta.SetState) && in.MaxValue > max {
						max = in.MaxValue
					}
				}
			}
		}
		value, ok := biosmeta.ArgForInterface(biosmeta.Interface(b.Interface), max, active)
		if !ok {
			continue
		}
		line := fmt.Sprintf("%s %d\n", b.Command, value)
		if err := send(line); err != nil {
			continue
		}
		sent = append(sent, line)
	}
	return sent
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
	for i := range file.Profiles {
		p := file.Profiles[i]
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
