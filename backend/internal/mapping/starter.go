// Starter profiles: ready-made panel bindings for the aircraft the original
// DCS Panel Manager shipped profiles for. They are seeded once, the first time
// the binding file does not exist, so a new install has something that works
// out of the box; the operator edits them in the Cockpit panels tab.
//
// They are adapted to this manager's binding model, which is one command per
// panel control: the original C# profiles could send several commands and carry
// literal arguments ("+3200", "DEC"), while here the argument is derived from the
// control's interface and its declared maximum. Two consequences, deliberate:
//
//   - a control like the PZ55's battery switch maps to a single set_state binding
//     (the value follows the switch position) rather than two entries;
//   - the gear lever is two controls (GEAR_UP and GEAR_DOWN) driving set_state
//     positions 0 and 1 of one command, rather than one entry with two actions.
//
// Output bindings (gear lights, the PZ70 LCD and button LEDs) are not part of the
// binding model yet, so they are not reproduced.
package mapping

import "dcsmanager/internal/panel"

// setState is the interface a two-position selector accepts, and what a panel
// switch drives: the switch position becomes the selector position.
const setState = "set_state"

// starterProfile is the aircraft name and its bindings.
type starterProfile struct {
	aircraft string
	bindings []Binding
}

// starterProfiles returns the built-in profiles, in a stable order.
//
// The command identifiers come from DCS-BIOS' own metadata and were checked
// against it (they exist for each aircraft, with the interface listed here). The
// interface a control accepts decides the value a binding sends:
//
//   - set_state: a selector; the switch position is the selector position.
//   - variable_step: a trim wheel or a dial nudged one step per detent.
func starterProfiles() []starterProfile {
	return []starterProfile{
		{
			aircraft: "FA-18C_hornet",
			bindings: []Binding{
				{Model: panel.PZ55, Control: "MASTER_BAT", Command: "BATTERY_SW", Interface: setState},
				{Model: panel.PZ55, Control: "GEAR_UP", Command: "GEAR_LEVER", Interface: setState, Invert: true},
				{Model: panel.PZ55, Control: "GEAR_DOWN", Command: "GEAR_LEVER", Interface: setState},
				{Model: panel.PZ70, Control: "FLAPS_UP", Command: "FLAP_SW", Interface: setState, Invert: true},
				{Model: panel.PZ70, Control: "FLAPS_DOWN", Command: "FLAP_SW", Interface: setState},
			},
		},
		{
			aircraft: "F-16C_50",
			bindings: []Binding{
				{Model: panel.PZ55, Control: "MASTER_BAT", Command: "FUEL_MASTER_SW", Interface: setState},
				{Model: panel.PZ55, Control: "GEAR_UP", Command: "GEAR_HANDLE", Interface: setState, Invert: true},
				{Model: panel.PZ55, Control: "GEAR_DOWN", Command: "GEAR_HANDLE", Interface: setState},
				{Model: panel.PZ70, Control: "PITCH_TRIM", Command: "PITCH_TRIM", Interface: "variable_step"},
			},
		},
		{
			aircraft: "M-2000C",
			bindings: []Binding{
				{Model: panel.PZ55, Control: "MASTER_BAT", Command: "MAIN_BATT_SW", Interface: setState},
				{Model: panel.PZ55, Control: "FUEL_PUMP", Command: "ENG_FUEL_L_PUMP_SW", Interface: setState},
				{Model: panel.PZ55, Control: "GEAR_UP", Command: "LDG_LEV", Interface: setState, Invert: true},
				{Model: panel.PZ55, Control: "GEAR_DOWN", Command: "LDG_LEV", Interface: setState},
				{Model: panel.PZ70, Control: "AP_BUTTON", Command: "AP_MASTER_BTN", Interface: "action"},
			},
		},
		{
			aircraft: "F-5E-3",
			bindings: []Binding{
				{Model: panel.PZ55, Control: "MASTER_BAT", Command: "SW_BATTERY", Interface: setState},
				{Model: panel.PZ55, Control: "FUEL_PUMP", Command: "L_BOOSTPUMP", Interface: setState},
				{Model: panel.PZ55, Control: "GEAR_UP", Command: "LG_LEVER_SWITCH", Interface: setState, Invert: true},
				{Model: panel.PZ55, Control: "GEAR_DOWN", Command: "LG_LEVER_SWITCH", Interface: setState},
				{Model: panel.PZ70, Control: "FLAPS_UP", Command: "FLAPS", Interface: setState, Invert: true},
				{Model: panel.PZ70, Control: "FLAPS_DOWN", Command: "FLAPS", Interface: setState},
			},
		},
	}
}

// Starter returns the built-in profiles as Profiles, ready to seed a store.
func Starter() []Profile {
	defs := starterProfiles()
	out := make([]Profile, 0, len(defs))
	for _, d := range defs {
		out = append(out, Profile{Aircraft: d.aircraft, Bindings: d.bindings})
	}
	return out
}
