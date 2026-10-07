package panel

import "testing"

// TestModelFor checks the vendor/product mapping that identifies the panels.
func TestModelFor(t *testing.T) {
	cases := []struct {
		vendor, product uint16
		want            Model
		ok              bool
	}{
		{VendorID, ProductPZ55, PZ55, true},
		{VendorID, ProductPZ70, PZ70, true},
		{VendorID, 0x1234, "", false},
		{0x1234, ProductPZ55, "", false},
	}
	for _, c := range cases {
		got, ok := ModelFor(c.vendor, c.product)
		if got != c.want || ok != c.ok {
			t.Errorf("ModelFor(%04X, %04X) = (%q, %v), want (%q, %v)",
				c.vendor, c.product, got, ok, c.want, c.ok)
		}
	}
}

// TestDecodePZ70Button checks a button bit turning on and off is reported once
// each way, and that an unchanged report yields nothing.
func TestDecodePZ70Button(t *testing.T) {
	defs := Controls(PZ70)
	var ap Control
	for _, c := range defs {
		if c.ID == "AP_BUTTON" {
			ap = c
		}
	}
	if ap.ID == "" {
		t.Fatal("AP_BUTTON should be defined")
	}

	off := []byte{0x00, 0x00, 0x00, 0x00}
	on := []byte{0x00, 0x00, 0x00, 0x00}
	on[ap.ByteIndex] |= ap.Mask

	// First report from a nil previous state: nothing to report on its own.
	if ev := Decode("d", PZ70, nil, off); len(ev) != 0 {
		t.Errorf("first idle report should yield no event, got %+v", ev)
	}
	// Press.
	ev := Decode("d", PZ70, off, on)
	if len(ev) != 1 || ev[0].Control.ID != "AP_BUTTON" || !ev[0].Active {
		t.Fatalf("press not decoded: %+v", ev)
	}
	// Release.
	ev = Decode("d", PZ70, on, off)
	if len(ev) != 1 || ev[0].Control.ID != "AP_BUTTON" || ev[0].Active {
		t.Fatalf("release not decoded: %+v", ev)
	}
	// No change.
	if ev := Decode("d", PZ70, on, on); len(ev) != 0 {
		t.Fatalf("an unchanged report should yield no event, got %+v", ev)
	}
}

// TestDecodeEncoderIsOnePulse checks the falling edge of an encoder bit does not
// become a second detent in the same direction.
func TestDecodeEncoderIsOnePulse(t *testing.T) {
	defs := Controls(PZ70)
	var cw Control
	for _, c := range defs {
		if c.ID == "PITCH_TRIM" && c.Clockwise {
			cw = c
		}
	}
	if cw.ID == "" {
		t.Fatal("PITCH_TRIM clockwise should be defined")
	}

	idle := []byte{0, 0, 0, 0}
	pulse := []byte{0, 0, 0, 0}
	pulse[cw.ByteIndex] |= cw.Mask

	// Rising edge: one event, clockwise.
	ev := Decode("d", PZ70, idle, pulse)
	if len(ev) != 1 || !ev[0].Control.Clockwise || !ev[0].Active {
		t.Fatalf("rising edge not decoded as one clockwise detent: %+v", ev)
	}
	// Falling edge: nothing, or a single turn would count twice.
	if ev := Decode("d", PZ70, pulse, idle); len(ev) != 0 {
		t.Fatalf("falling edge should not yield an event, got %+v", ev)
	}
}

// TestDecodePZ55Toggle checks a two-state switch.
func TestDecodePZ55Toggle(t *testing.T) {
	defs := Controls(PZ55)
	var master Control
	for _, c := range defs {
		if c.ID == "MASTER_BAT" {
			master = c
		}
	}
	off := []byte{0, 0, 0}
	on := []byte{0, 0, 0}
	on[master.ByteIndex] |= master.Mask

	ev := Decode("d", PZ55, off, on)
	if len(ev) != 1 || ev[0].Control.ID != "MASTER_BAT" || !ev[0].Active {
		t.Fatalf("toggle not decoded: %+v", ev)
	}
}

// TestDecodeRejectsShortReport checks a report too short to hold any control is
// ignored rather than panicking.
func TestDecodeRejectsShortReport(t *testing.T) {
	if ev := Decode("d", PZ70, nil, []byte{0x01}); len(ev) != 0 {
		t.Fatalf("a 1-byte report should be ignored, got %+v", ev)
	}
}

// TestEncodePZ55Lights checks the gear light byte: green and red are separate
// bits, and yellow lights both.
func TestEncodePZ55Lights(t *testing.T) {
	reports := EncodePZ55GearLights(GearLights{Upper: GearGreen, Left: GearRed, Right: GearOff})
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
	got := reports[1][1]
	want := byte(0x01 | 0x10) // upper green, left red
	if got != want {
		t.Errorf("light byte = %#02x, want %#02x", got, want)
	}

	reports = EncodePZ55GearLights(GearLights{Upper: GearYellow})
	if reports[1][1] != 0x01|0x08 {
		t.Errorf("yellow should light both elements, got %#02x", reports[1][1])
	}
}

// TestEncodePZ70Panel checks the 13-byte layout and the display rendering.
func TestEncodePZ70Panel(t *testing.T) {
	upper, lower := 270, -1500
	report := EncodePZ70Panel(PZ70Panel{
		UpperDisplay: &upper,
		LowerDisplay: &lower,
		Lights:       PZ70LightAP | PZ70LightALT,
	})
	if len(report) != 13 {
		t.Fatalf("report length = %d, want 13", len(report))
	}
	if report[0] != 0 || report[12] != 0xFF {
		t.Errorf("report framing wrong: id=%#02x last=%#02x", report[0], report[12])
	}
	if report[11] != byte(PZ70LightAP|PZ70LightALT) {
		t.Errorf("lights byte = %#02x, want %#02x", report[11], byte(PZ70LightAP|PZ70LightALT))
	}
	// "270" is written backwards from position 5: 0,7,2 at 5,4,3.
	if report[5] != '0' || report[4] != '7' || report[3] != '2' {
		t.Errorf("upper display wrong: %q", string(report[3:6]))
	}
	// "-1500" ends at position 10: 0,0,5,1,- at 10,9,8,7,6.
	if report[10] != '0' || report[9] != '0' || report[8] != '5' || report[7] != '1' || report[6] != 0xEE {
		t.Errorf("lower display wrong: % X", report[6:11])
	}
}

// TestEncodePZ70PanelClamps checks the limits DCS's panels can show. The top line
// has no room for a sign, so a negative altitude is shown as its magnitude.
func TestEncodePZ70PanelClamps(t *testing.T) {
	big := 999999
	report := EncodePZ70Panel(PZ70Panel{UpperDisplay: &big})
	// Clamped to 99999: digits 9,9,9,9,9 at 5..1.
	for i := 1; i <= 5; i++ {
		if report[i] != '9' {
			t.Errorf("clamped display byte %d = %q, want '9'", i, string(report[i]))
		}
	}

	neg := -500
	report = EncodePZ70Panel(PZ70Panel{UpperDisplay: &neg})
	// The magnitude is shown, without a sign (no room for one on that line):
	// "500" written backwards from position 5, so 0,0,5 sit at 5,4,3.
	if report[5] != '0' || report[4] != '0' || report[3] != '5' {
		t.Errorf("negative upper display should show its magnitude at 3..5: % X", report[3:6])
	}
}

// TestEncodePZ70PanelLeavesUnsetDisplay checks a nil display is not written, so a
// caller can update only the lights or only one line.
func TestEncodePZ70PanelLeavesUnsetDisplay(t *testing.T) {
	report := EncodePZ70Panel(PZ70Panel{Lights: PZ70LightHDG})
	for i := 1; i <= 10; i++ {
		if report[i] != 0xFF {
			t.Errorf("unset display byte %d = %#02x, want 0xFF (blank)", i, report[i])
		}
	}
}

// TestFormatPZ70Line checks the preview text matches the LCD layout: five digits
// (abs, no sign) on top, five characters with a minus on the bottom, right-aligned
// with leading spaces.
func TestFormatPZ70Line(t *testing.T) {
	cases := []struct {
		line  DisplayLine
		value int
		want  string
	}{
		{LineUpper, 12345, "12345"},
		{LineUpper, 7, "    7"},
		{LineUpper, -42, "   42"}, // the top line has no sign
		{LineLower, -1234, "-1234"},
		{LineLower, 300, "  300"},
	}
	for _, c := range cases {
		if got := FormatPZ70Line(c.line, c.value); got != c.want {
			t.Errorf("FormatPZ70Line(%v, %d) = %q, want %q", c.line, c.value, got, c.want)
		}
	}
}

// TestDisplayModeValidation checks the selector modes and lines a display binding
// may name.
func TestDisplayModeValidation(t *testing.T) {
	for _, m := range []string{"ALT", "alt", "VS", "IAS", "HDG", "CRS"} {
		if !ValidDisplayMode(m) {
			t.Errorf("%q should be a valid mode", m)
		}
	}
	for _, m := range []string{"", "NOPE", "KNOB_ALT"} {
		if ValidDisplayMode(m) {
			t.Errorf("%q should not be a valid mode", m)
		}
	}
	for _, l := range []string{"upper", "UPPER", "lower"} {
		if !ValidDisplayLine(l) {
			t.Errorf("%q should be a valid line", l)
		}
	}
	if ValidDisplayLine("middle") {
		t.Error("middle is not a line")
	}
}
