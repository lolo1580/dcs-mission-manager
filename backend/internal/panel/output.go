package panel

import (
	"strconv"
	"strings"
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

// DisplayLine identifies one of the PZ70's two LCD lines.
type DisplayLine int

const (
	// LineUpper is the top line: five digits, no sign.
	LineUpper DisplayLine = iota
	// LineLower is the bottom line: ten characters, room for a minus.
	LineLower
)

// DisplayModes are the PZ70 selector positions a display binding can answer to,
// matching the panel's own control ids.
var DisplayModes = []string{"ALT", "VS", "IAS", "HDG", "CRS"}

// displayModeSet is the lookup form of DisplayModes (case-insensitive).
var displayModeSet = func() map[string]bool {
	m := map[string]bool{}
	for _, mode := range DisplayModes {
		m[strings.ToUpper(mode)] = true
	}
	return m
}()

// ValidDisplayMode reports whether mode is a PZ70 selector position.
func ValidDisplayMode(mode string) bool {
	return displayModeSet[strings.ToUpper(mode)]
}

// ValidDisplayLine reports whether line names an LCD line ("upper" or "lower").
func ValidDisplayLine(line string) bool {
	return strings.EqualFold(line, "upper") || strings.EqualFold(line, "lower")
}

// GearLights is the PZ55's landing-gear indicator state, one per wheel.
type GearLights struct {
	Upper GearLight
	Left  GearLight
	Right GearLight
}

// Output targets: which indicator on which panel an output binding drives,
// independent of the aircraft. They are the stable names a profile stores.
const (
	// PZ55 landing-gear indicators.
	TargetGearUpper = "LIGHT_GEAR_UPPER"
	TargetGearLeft  = "LIGHT_GEAR_LEFT"
	TargetGearRight = "LIGHT_GEAR_RIGHT"
	// PZ70 autopilot button lights.
	TargetAP  = "LIGHT_AP"
	TargetHDG = "LIGHT_HDG"
	TargetNAV = "LIGHT_NAV"
	TargetIAS = "LIGHT_IAS"
	TargetALT = "LIGHT_ALT"
	TargetVS  = "LIGHT_VS"
	TargetAPR = "LIGHT_APR"
	TargetREV = "LIGHT_REV"
)

// GearLightFor turns a stored colour name into a GearLight. An unknown or empty
// colour is green, the common case for a gear-down indicator.
func GearLightFor(color string) GearLight {
	switch strings.ToLower(color) {
	case "red":
		return GearRed
	case "yellow":
		return GearYellow
	case "off":
		return GearOff
	default:
		return GearGreen
	}
}

// gearTargets maps the PZ55 gear targets to the struct field they set.
var gearTargets = map[string]func(*GearLights, GearLight){
	TargetGearUpper: func(g *GearLights, c GearLight) { g.Upper = c },
	TargetGearLeft:  func(g *GearLights, c GearLight) { g.Left = c },
	TargetGearRight: func(g *GearLights, c GearLight) { g.Right = c },
}

// pz70TargetLights maps the PZ70 light targets to their bit.
var pz70TargetLights = map[string]PZ70Lights{
	TargetAP:  PZ70LightAP,
	TargetHDG: PZ70LightHDG,
	TargetNAV: PZ70LightNAV,
	TargetIAS: PZ70LightIAS,
	TargetALT: PZ70LightALT,
	TargetVS:  PZ70LightVS,
	TargetAPR: PZ70LightAPR,
	TargetREV: PZ70LightREV,
}

// GearTarget reports whether target is a PZ55 gear indicator, returning the
// setter for it.
func GearTarget(target string) (func(*GearLights, GearLight), bool) {
	set, ok := gearTargets[strings.ToUpper(target)]
	return set, ok
}

// PZ70LightTarget reports whether target is a PZ70 button light, returning its bit.
func PZ70LightTarget(target string) (PZ70Lights, bool) {
	bit, ok := pz70TargetLights[strings.ToUpper(target)]
	return bit, ok
}

// ValidTarget reports whether target names an indicator that model has.
func ValidTarget(model Model, target string) bool {
	switch model {
	case PZ55:
		_, ok := gearTargets[strings.ToUpper(target)]
		return ok
	case PZ70:
		_, ok := pz70TargetLights[strings.ToUpper(target)]
		return ok
	}
	return false
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
	// UpperDisplay is the top line. Nil blanks it in the complete report.
	UpperDisplay *int
	// LowerDisplay is the bottom line. Nil blanks it in the complete report.
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
		// Five characters, with room for a minus: the bottom line shows a
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

// FormatPZ70Line renders, as text, what a PZ70 line would show for a value. It
// mirrors EncodePZ70Panel exactly (abs and five digits on the top line, signed and
// five characters on the bottom), so a preview in the interface is what the panel
// displays rather than an approximation. Unused cells are spaces.
func FormatPZ70Line(line DisplayLine, value int) string {
	if line == LineUpper {
		return padLeft(strconv.Itoa(clamp(abs(value), 0, 99999)), 5)
	}
	return padLeft(strconv.Itoa(clamp(value, -9999, 99999)), 5)
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
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
