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
// Output bindings (gear lights and autopilot button LEDs) are carried too, for
// the aircraft the original profiles covered. The PZ55's three gear lights are
// bicolour (green for down-and-locked, red for unsafe); the PZ70's autopilot
// lights are single-colour. The PZ70 LCD is not part of the binding model yet, so
// it is not reproduced.
package mapping

import "dcsmanager/internal/panel"

// setState is the interface a two-position selector accepts, and what a panel
// switch drives: the switch position becomes the selector position.
const setState = "set_state"

// starterProfile is the aircraft name, its bindings, its outputs and its LCD
// displays.
type starterProfile struct {
	aircraft string
	bindings []Binding
	outputs  []OutputBinding
	displays []DisplayBinding
}

// selectedDegrees converts a DCS-BIOS normalised angle (a 0..1 float exported as a
// 0..65535 integer, its standard 16-bit form) into degrees. It is the scale the
// starter displays use for a heading or a course.
const selectedDegrees = 360.0 / 65535.0

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
			outputs: []OutputBinding{
				{Model: panel.PZ55, Target: panel.TargetGearLeft, Command: "FLP_LG_LEFT_GEAR_LT", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "FLP_LG_NOSE_GEAR_LT", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearRight, Command: "FLP_LG_RIGHT_GEAR_LT", Color: "green"},
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
			outputs: []OutputBinding{
				{Model: panel.PZ55, Target: panel.TargetGearLeft, Command: "LIGHT_GEAR_L", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "LIGHT_GEAR_N", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearRight, Command: "LIGHT_GEAR_R", Color: "green"},
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
			outputs: []OutputBinding{
				{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "LANDING_GEAR_LEVER_LIGHT", Color: "red"},
				{Model: panel.PZ70, Target: panel.TargetAP, Command: "AP_MASTER_VERT", Color: "green"},
				{Model: panel.PZ70, Target: panel.TargetALT, Command: "AP_ALT_VERT", Color: "green"},
			},
			// The HSI exports the selected heading and course as normalised angles; a
			// 0..1 float is DCS-BIOS' standard 0..65535 integer, so multiplying by
			// 360/65535 gives degrees. These are the two "selected" references the
			// aircraft actually exposes as integers (its selected altitude is not
			// exported on its own).
			displays: []DisplayBinding{
				{Model: panel.PZ70, Mode: "HDG", Line: "upper", Command: "HSI_HDG", Export: 0, Scale: selectedDegrees, Unit: "deg"},
				{Model: panel.PZ70, Mode: "CRS", Line: "upper", Command: "HSI_D_NEEDLE", Export: 0, Scale: selectedDegrees, Unit: "deg"},
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
			outputs: []OutputBinding{
				{Model: panel.PZ55, Target: panel.TargetGearLeft, Command: "LEFT_LIGHT", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "NOSE_LIGHT", Color: "green"},
				{Model: panel.PZ55, Target: panel.TargetGearRight, Command: "RIGHT_LIGHT", Color: "green"},
			},
			// The HSI exports the selected heading and course as normalised angles.
			displays: []DisplayBinding{
				{Model: panel.PZ70, Mode: "HDG", Line: "upper", Command: "HSI_HDG", Export: 0, Scale: selectedDegrees, Unit: "deg"},
				{Model: panel.PZ70, Mode: "CRS", Line: "upper", Command: "HSI_CRS", Export: 0, Scale: selectedDegrees, Unit: "deg"},
			},
		},
	}
}

// Starter returns the built-in profiles as Profiles, ready to seed a store.
func Starter() []Profile {
	defs := starterProfiles()
	out := make([]Profile, 0, len(defs))
	for _, d := range defs {
		out = append(out, Profile{Aircraft: d.aircraft, Bindings: d.bindings, Outputs: d.outputs, Displays: d.displays})
	}
	return out
}
