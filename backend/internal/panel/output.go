package panel

import (
	"strconv"
)

// GearLight is one of the PZ55's three landing-gear indicator colours.
type GearLight int

const (
	// GearOff is an unlit indicator.
	GearOff GearLight = iota
	// GearGreen is the down-and-locked indication.
	GearGreen
	// GearRed is the unsafe indication.
	GearRed
	// GearYellow lights both elements, which the panel shows as amber.
	GearYellow
)

// GearLights is the PZ55's landing-gear indicator state, one per wheel.
type GearLights struct {
	Upper GearLight
	Left  GearLight
	Right GearLight
}

// PZ70Lights is the PZ70's autopilot button illumination, one bit per button.
type PZ70Lights uint16

// Bit positions of the PZ70's illuminated buttons, as the panel expects them.
const (
	PZ70LightAP PZ70Lights = 1 << iota
	PZ70LightHDG
	PZ70LightNAV
	PZ70LightIAS
	PZ70LightALT
	PZ70LightVS
	PZ70LightAPR
	PZ70LightREV
	PZ70LightAutoThrottle
)

// PZ70Panel is the whole output state of a Multi Panel: the two display lines and
// the button lights.
type PZ70Panel struct {
	// UpperDisplay is the top line, when set. Nil leaves it untouched.
	UpperDisplay *int
	// LowerDisplay is the bottom line, when set. Nil leaves it untouched.
	LowerDisplay *int
	// Lights is which buttons are lit.
	Lights PZ70Lights
}

// EncodePZ55GearLights builds the reports that set the PZ55's landing-gear
// indicators. The panel takes two reports, the second carrying the light byte.
func EncodePZ55GearLights(l GearLights) [][]byte {
	// Green is bit 0 and red bit 3 for the upper wheel, the left and right wheels
	// following one bit later each. Yellow is both elements on.
	var b byte
	b |= encodeGearLight(l.Upper, 0x01, 0x08)
	b |= encodeGearLight(l.Left, 0x02, 0x10)
	b |= encodeGearLight(l.Right, 0x04, 0x20)
	return [][]byte{{0, 0}, {0, b}}
}

func encodeGearLight(color GearLight, green, red byte) byte {
	switch color {
	case GearOff:
		return 0
	case GearGreen:
		return green
	case GearRed:
		return red
	case GearYellow:
		return green | red
	}
	return 0
}

// EncodePZ70Panel builds the output report for a Multi Panel.
//
// The report is 13 bytes: a report id, ten display/LED bytes, a lights byte and
// a trailing reserved byte. The shorter 12-byte form is rejected by Windows, which
// is a detail worth keeping — it cost time to find.
func EncodePZ70Panel(p PZ70Panel) []byte {
	report := make([]byte, 13)
	report[0] = 0
	for i := 1; i <= 10; i++ {
		report[i] = 0xFF
	}
	report[11] = byte(p.Lights)
	report[12] = 0xFF

	if p.UpperDisplay != nil {
		// Five digits, no minus sign: the top line shows a heading or altitude.
		writeDisplay(report, clamp(abs(*p.UpperDisplay), 0, 99999), 5, false)
	}
	if p.LowerDisplay != nil {
		// Ten characters, with room for a minus: the bottom line shows a
		// vertical speed or a trim value.
		writeDisplay(report, clamp(*p.LowerDisplay, -9999, 99999), 10, true)
	}
	return report
}

// writeDisplay fills the report backwards from a position, one digit per byte,
// which is how the panel's display is laid out.
func writeDisplay(report []byte, value, position int, supportsMinus bool) {
	s := strconv.Itoa(value)
	for i := len(s) - 1; i >= 0; i-- {
		if position <= 0 {
			break
		}
		c := s[i]
		if c == '-' && supportsMinus {
			report[position] = 0xEE
		} else {
			report[position] = c
		}
		position--
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
