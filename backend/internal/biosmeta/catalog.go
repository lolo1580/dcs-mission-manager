// Package biosmeta reads the control metadata DCS-BIOS publishes about each
// aircraft.
//
// DCS-BIOS writes one JSON file per module under
// `Saved Games\DCS\Scripts\DCS-BIOS\doc\json`, describing every cockpit control:
// its identifier (the name a command uses), its kind, the commands it accepts,
// and the address its state is exported at. That catalogue is what turns "a
// panel button" into "a DCS-BIOS command", so the manager reads it rather than
// hard-coding a mapping per aircraft.
package biosmeta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Input is one command a control accepts.
type Input struct {
	// Interface is how the command is sent: "fixed_step" to step through states,
	// "set_state" to jump to one, "variable_step" to nudge a dial, "action" to
	// trigger.
	Interface string `json:"interface"`
	// Description is DCS-BIOS' own wording.
	Description string `json:"description,omitempty"`
	// MaxValue is the highest position set_state accepts (0 when not applicable).
	MaxValue int `json:"max_value,omitempty"`
	// SuggestedStep is the increment variable_step suggests.
	SuggestedStep int `json:"suggested_step,omitempty"`
}

// Output is one value DCS-BIOS exports for a control.
type Output struct {
	Address     int    `json:"address"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	MaxValue    int    `json:"max_value,omitempty"`
	Mask        int    `json:"mask,omitempty"`
	ShiftBy     int    `json:"shift_by,omitempty"`
}

// Control is one cockpit control of an aircraft.
type Control struct {
	// Category groups controls in the cockpit (ADI, UFC, Flaps…).
	Category string `json:"category"`
	// Identifier is the name used in commands, e.g. "MASTER_ARM_SW".
	Identifier string `json:"identifier"`
	// ControlType is DCS-BIOS' own kind: selector, momentary, dial, display…
	ControlType string `json:"control_type"`
	// Description is a human sentence.
	Description string `json:"description,omitempty"`
	// Inputs are the commands the control accepts. Empty means it is read-only
	// (a gauge or an indicator), which a mapping cannot drive.
	Inputs []Input `json:"inputs,omitempty"`
	// Outputs are the values DCS-BIOS exports for it.
	Outputs []Output `json:"outputs,omitempty"`
}

// Writable reports whether a control accepts a command, which is what a panel
// mapping needs. A control with no inputs is a display.
func (c Control) Writable() bool { return len(c.Inputs) > 0 }

// Supports reports whether the control accepts the given interface.
func (c Control) Supports(iface string) bool {
	for _, in := range c.Inputs {
		if in.Interface == iface {
			return true
		}
	}
	return false
}

// Catalog is the controls of one module, indexed by identifier.
type Catalog struct {
	// Module is the file name without its extension, e.g. "F-16C_50".
	Module string
	// Controls is every control, in a stable order.
	Controls []Control
	// byID indexes them for lookup.
	byID map[string]Control
	// byCategory groups them for the UI.
	byCategory map[string][]Control
}

// ByID returns a control by its identifier (case-insensitive, as DCS-BIOS
// identifiers are upper-case but callers may not be).
func (c *Catalog) ByID(id string) (Control, bool) {
	control, ok := c.byID[strings.ToUpper(id)]
	return control, ok
}

// Categories returns the category names present, sorted.
func (c *Catalog) Categories() []string {
	out := make([]string, 0, len(c.byCategory))
	for name := range c.byCategory {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// InCategory returns the controls of one category, in a stable order.
func (c *Catalog) InCategory(name string) []Control {
	out := make([]Control, 0, len(c.byCategory[name]))
	out = append(out, c.byCategory[name]...)
	return out
}

// Writable returns every control that accepts a command, which is what a mapping
// editor offers.
func (c *Catalog) Writable() []Control {
	out := make([]Control, 0, len(c.Controls))
	for _, ctl := range c.Controls {
		if ctl.Writable() {
			out = append(out, ctl)
		}
	}
	return out
}

// JSONDir returns the folder DCS-BIOS writes its metadata to.
func JSONDir(savedGames string) string {
	return filepath.Join(savedGames, "Scripts", "DCS-BIOS", "doc", "json")
}

// DCS-BIOS spells the module name with underscores the user does not use: the
// F-16C's module is "F-16C_50", but the aircraft is reported as "F-16C_50" too, so
// the name generally matches. A few aliases smooth the rest.
//
// LoadModule tries the exact name first; this map only covers the cases where the
// export name and the module file differ.
var moduleAliases = map[string]string{
	"F-16C_50": "F-16C_50",
}

// LoadModule reads the catalogue for one module. It returns an error the caller
// can ignore: an aircraft without metadata simply offers no controls.
func LoadModule(savedGames, module string) (*Catalog, error) {
	if alias, ok := moduleAliases[module]; ok {
		module = alias
	}
	path := filepath.Join(JSONDir(savedGames), module+".json")

	// The file name must resolve inside the metadata folder: the module name
	// comes from the aircraft DCS reports, but a crafted one must not escape.
	clean := filepath.Clean(path)
	base := filepath.Clean(JSONDir(savedGames))
	if !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]Control
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	// DCS-BIOS writes the same file for a few modules and expects them to be
	// merged; an empty file yields an empty catalogue rather than an error.
	return newCatalog(module, raw), nil
}

// newCatalog flattens DCS-BIOS' `{"Category": {"IDENTIFIER": {...}}}` shape.
func newCatalog(module string, raw map[string]map[string]Control) *Catalog {
	cat := &Catalog{
		Module:     module,
		byID:       map[string]Control{},
		byCategory: map[string][]Control{},
	}
	// Iterate the categories in a stable order so the catalogue is deterministic.
	categories := make([]string, 0, len(raw))
	for name := range raw {
		categories = append(categories, name)
	}
	sort.Strings(categories)

	for _, category := range categories {
		controls := raw[category]
		ids := make([]string, 0, len(controls))
		for id := range controls {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			ctl := controls[id]
			ctl.Category = category
			if ctl.Identifier == "" {
				ctl.Identifier = id
			}
			cat.Controls = append(cat.Controls, ctl)
			cat.byID[strings.ToUpper(ctl.Identifier)] = ctl
			cat.byCategory[category] = append(cat.byCategory[category], ctl)
		}
	}
	return cat
}
