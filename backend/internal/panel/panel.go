// Package panel decodes and encodes the Logitech/Saitek Pro Flight panel
// protocol: the PZ55 Switch Panel and the PZ70 Multi Panel.
//
// The bit layouts are not published; they were mapped by observing the reports
// the panels send and the reports they accept. They are reproduced here because
// the manager drives these devices directly, with no DCS-BIOS dependency for the
// hardware side.
package panel

// Model identifies a supported panel.
type Model string

const (
	// PZ55 is the Pro Flight Switch Panel: toggle switches and a gear lever.
	PZ55 Model = "pz55"
	// PZ70 is the Pro Flight Multi Panel: autopilot selector, buttons, an LCD
	// wheel and a trim wheel.
	PZ70 Model = "pz70"
)

// VendorID is Logitech/Saitek. Both panels share it.
const VendorID = 0x06A3

// Product ids of the supported panels.
const (
	ProductPZ55 = 0x0D67
	ProductPZ70 = 0x0D06
)

// ModelFor returns the panel model for a USB product id, and whether it is
// supported.
func ModelFor(vendor, product uint16) (Model, bool) {
	if vendor != VendorID {
		return "", false
	}
	switch product {
	case ProductPZ55:
		return PZ55, true
	case ProductPZ70:
		return PZ70, true
	}
	return "", false
}

// ControlKind says how a control behaves, which the manager needs to decide what
// a change means.
type ControlKind int

const (
	// Toggle is a two-state switch: on and off.
	Toggle ControlKind = iota
	// Button is momentary: pressed then released.
	Button
	// EncoderPulse is one detent of a rotating control, reported as a short
	// pulse. Only the active edge counts: a release is not a second detent.
	EncoderPulse
)

// Control is one physical input of a panel.
type Control struct {
	// ID is a stable, human-readable name (MASTER_BAT, AP_BUTTON…).
	ID string
	// Kind is how the control behaves.
	Kind ControlKind
	// ByteIndex and Mask locate the control in an input report.
	ByteIndex int
	Mask      byte
	// Clockwise is set on encoder pulses that turn one way. It has no meaning
	// for other kinds.
	Clockwise bool
}

// Event is one state change decoded from a report.
type Event struct {
	// Device is the panel it came from, as the caller identifies it (a serial or
	// an index); the decoder does not invent one.
	Device string
	Model  Model
	// Control is the input that changed.
	Control Control
	// Active is the new state: true for a switch turned on, a button pressed, or
	// an encoder pulse.
	Active bool
}

// Controls returns the input map of a model. It is a copy, so a caller cannot
// corrupt the shared definition.
func Controls(m Model) []Control {
	base, ok := definitions[m]
	if !ok {
		return nil
	}
	out := make([]Control, len(base))
	copy(out, base)
	return out
}

// Decode returns the events between two reports of the same panel.
//
// prev may be nil for the first report, in which case only the controls already
// active in cur are not reported: the panel's power-on state is not a change the
// user made. Encoder pulses report only on the active edge, so a single detent is
// one event rather than two.
func Decode(device string, model Model, prev, cur []byte) []Event {
	defs, ok := definitions[model]
	if !ok || len(cur) < 3 {
		return nil
	}
	var out []Event
	for _, c := range defs {
		if c.ByteIndex >= len(cur) {
			continue
		}
		wasActive := len(prev) > c.ByteIndex && prev[c.ByteIndex]&c.Mask != 0
		isActive := cur[c.ByteIndex]&c.Mask != 0
		if wasActive == isActive {
			continue
		}
		if c.Kind == EncoderPulse && !isActive {
			continue // the falling edge of a pulse is not a second detent
		}
		out = append(out, Event{Device: device, Model: model, Control: c, Active: isActive})
	}
	return out
}

// bit builds a toggle or button control at one bit of a report byte.
func bit(byteIndex int, bitIndex int, id string) Control {
	return Control{ID: id, Kind: Toggle, ByteIndex: byteIndex, Mask: 1 << uint(bitIndex)}
}

// button is a momentary control at one bit.
func button(byteIndex int, bitIndex int, id string) Control {
	return Control{ID: id, Kind: Button, ByteIndex: byteIndex, Mask: 1 << uint(bitIndex)}
}

// encoder is one direction of a rotating control.
func encoder(byteIndex int, bitIndex int, id string, clockwise bool) Control {
	return Control{ID: id, Kind: EncoderPulse, ByteIndex: byteIndex, Mask: 1 << uint(bitIndex), Clockwise: clockwise}
}

// definitions holds the bit layout of each supported panel.
var definitions = map[Model][]Control{
	PZ55: {
		bit(0, 0, "MASTER_BAT"), bit(0, 1, "MASTER_ALT"), bit(0, 2, "AVIONICS_MASTER"),
		bit(0, 3, "FUEL_PUMP"), bit(0, 4, "DE_ICE"), bit(0, 5, "PITOT_HEAT"),
		bit(0, 6, "COWL"), bit(0, 7, "LIGHTS_PANEL"), bit(1, 0, "LIGHTS_BEACON"),
		bit(1, 1, "LIGHTS_NAV"), bit(1, 2, "LIGHTS_STROBE"), bit(1, 3, "LIGHTS_TAXI"),
		bit(1, 4, "LIGHTS_LANDING"), bit(1, 5, "ENGINE_OFF"), bit(1, 6, "ENGINE_RIGHT"),
		bit(1, 7, "ENGINE_LEFT"), bit(2, 0, "ENGINE_BOTH"), bit(2, 1, "ENGINE_START"),
		bit(2, 2, "GEAR_UP"), bit(2, 3, "GEAR_DOWN"),
	},
	PZ70: {
		button(0, 0, "KNOB_ALT"), button(0, 1, "KNOB_VS"), button(0, 2, "KNOB_IAS"),
		button(0, 3, "KNOB_HDG"), button(0, 4, "KNOB_CRS"),
		encoder(0, 5, "LCD_WHEEL", true), encoder(0, 6, "LCD_WHEEL", false),
		button(0, 7, "AP_BUTTON"), button(1, 0, "HDG_BUTTON"),
		button(1, 1, "NAV_BUTTON"), button(1, 2, "IAS_BUTTON"), button(1, 3, "ALT_BUTTON"),
		button(1, 4, "VS_BUTTON"), button(1, 5, "APR_BUTTON"), button(1, 6, "REV_BUTTON"),
		button(1, 7, "AUTO_THROTTLE"), button(2, 0, "FLAPS_UP"), button(2, 1, "FLAPS_DOWN"),
		encoder(2, 2, "PITCH_TRIM", false), encoder(2, 3, "PITCH_TRIM", true),
	},
}
