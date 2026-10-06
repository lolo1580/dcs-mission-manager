// Package led drives the Logitech panels' outputs (the PZ55 landing-gear lights
// and the PZ70 autopilot button LEDs, and the LCD to come) from values DCS-BIOS
// exports.
//
// It is the missing half of the panel bridge: mapping sends a switch move into
// the cockpit, this package drives the panels back from the cockpit. It is the Go
// counterpart of the C# project's output bindings.
//
// Sync builds the COMPLETE report of each device (gear lights for the PZ55; the
// LCD lines and the button lights for the PZ70) in one place, so a later LCD writer
// cannot wipe the display when it updates the lights: there is only ever one
// report, built once, written only when it changes.
package led

import (
	"encoding/binary"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panel"
	"dcsmanager/internal/panelservice"
)

// dcsbiosReader is the part of the DCS-BIOS client the driver reads.
type dcsbiosReader interface {
	Memory() map[uint16]byte
	State() dcsbios.State
}

// panelWriter is the part of the panel service that drives the indicators.
type panelWriter interface {
	Devices() []panelservice.DeviceInfo
	Write(devicePath string, report []byte) error
}

// Driver applies the current aircraft's output bindings. It is safe for
// concurrent use.
type Driver struct {
	bios   dcsbiosReader
	panels panelWriter
	store  *mapping.Store
	// loadMeta reads the DCS-BIOS catalogue of an aircraft (cached by its caller).
	loadMeta func(aircraft string) (*biosmeta.Catalog, error)
	// switchPos, when set, returns the current PZ70 selector position, so a display
	// mode change redraws without waiting for DCS. Optional.
	switchPos func() string

	mu sync.Mutex
	// last is the report last written to each panel path, as a stable key, so a
	// frame that changes nothing does not touch the USB. It is cleared on a device
	// disconnect/reconnect so the panel is redrawn after being unplugged.
	last map[string]string
	// logMu guards logf, which is set once at startup. It is separate from mu so
	// logging never re-enters the lock Sync holds.
	logMu sync.Mutex
	// logf, when set, receives one line per output write.
	logf func(area, format string, args ...any)
}

// SetLogger installs a logging function for output writes. Optional.
func (d *Driver) SetLogger(f func(area, format string, args ...any)) {
	d.logMu.Lock()
	d.logf = f
	d.logMu.Unlock()
}

// SetSwitchPos installs the PZ70 selector reader. Optional.
func (d *Driver) SetSwitchPos(f func() string) {
	d.mu.Lock()
	d.switchPos = f
	d.mu.Unlock()
}

func (d *Driver) log(area, format string, args ...any) {
	d.logMu.Lock()
	f := d.logf
	d.logMu.Unlock()
	if f != nil {
		f(area, format, args...)
	}
}

// New creates a driver.
func New(bios dcsbiosReader, panels panelWriter, store *mapping.Store, loadMeta func(string) (*biosmeta.Catalog, error)) *Driver {
	return &Driver{
		bios:     bios,
		panels:   panels,
		store:    store,
		loadMeta: loadMeta,
		last:     map[string]string{},
	}
}

// Forget drops the cached report for a device, so it is written afresh the next
// time. It is called when a panel disconnects or reconnects: an unplugged panel
// comes back with a blank display, and the cache would otherwise suppress the
// redraw.
func (d *Driver) Forget(devicePath string) {
	d.mu.Lock()
	delete(d.last, devicePath)
	d.mu.Unlock()
}

// ForgetAll drops every cached report, so all panels are redrawn on the next Sync.
// It is called when the active aircraft changes: what is displayed belongs to the
// previous one.
func (d *Driver) ForgetAll() {
	d.mu.Lock()
	d.last = map[string]string{}
	d.mu.Unlock()
}

// Sync builds and writes the complete output report of every connected panel.
// It is meant to be called on every DCS-BIOS frame and on every panel connection.
//
// It returns true when something was written. It does nothing until the operator
// turns the outputs on: driving a cockpit is opt-in, but — unlike sending commands
// — reads DCS-BIOS and never touches the aircraft.
func (d *Driver) Sync() bool {
	if d.store == nil || !d.store.OutputsEnabled() {
		return false
	}
	aircraft := d.bios.State().Aircraft
	if aircraft == "" {
		return false
	}
	p := d.store.Profile(aircraft)
	if len(p.Outputs) == 0 && len(p.Displays) == 0 {
		return false
	}
	cat, err := d.loadMeta(aircraft)
	if err != nil {
		return false
	}
	devices := d.panels.Devices()
	if len(devices) == 0 {
		return false
	}
	mem := d.bios.Memory()

	// The selector, read once: it decides which display mode's values are shown.
	sel := ""
	if d.switchPos != nil {
		sel = normalizeMode(d.switchPos())
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// The PZ55's three wheels arrive as three separate bindings but go out in a
	// single report per panel, so they are merged before writing. The PZ70 gathers
	// its lights and its two display lines into one report the same way.
	gear := map[string]*panel.GearLights{}
	pz70 := map[string]*panel.PZ70Panel{}

	for _, o := range p.Outputs {
		ctrl, ok := cat.ByID(o.Command)
		if !ok || len(ctrl.Outputs) == 0 {
			continue
		}
		on := outputOn(ctrl.Outputs[0], mem)

		switch o.Model {
		case panel.PZ55:
			set, ok := panel.GearTarget(o.Target)
			if !ok {
				continue
			}
			color := panel.GearOff
			if on {
				color = panel.GearLightFor(o.Color)
			}
			for _, dev := range devices {
				if model, _ := panel.ModelFor(dev.VendorID, dev.ProductID); model != panel.PZ55 {
					continue
				}
				g := gear[dev.Path]
				if g == nil {
					g = &panel.GearLights{}
					gear[dev.Path] = g
				}
				set(g, color)
			}
		case panel.PZ70:
			bit, ok := panel.PZ70LightTarget(o.Target)
			if !ok {
				continue
			}
			for _, dev := range devices {
				if model, _ := panel.ModelFor(dev.VendorID, dev.ProductID); model != panel.PZ70 {
					continue
				}
				dp := panelFor(pz70, dev.Path)
				if on {
					dp.Lights |= bit
				} else {
					dp.Lights &^= bit
				}
			}
		}
	}

	// Display bindings (PZ70 only): show the value of the control matching the
	// current selector mode, on its configured line.
	for _, disp := range p.Displays {
		if disp.Model != panel.PZ70 || !strings.EqualFold(disp.Mode, sel) {
			continue
		}
		ctrl, ok := cat.ByID(disp.Command)
		if !ok || disp.Export < 0 || disp.Export >= len(ctrl.Outputs) {
			continue
		}
		value, ok := ReadExportInt(ctrl.Outputs[disp.Export], mem)
		if !ok {
			continue // no value yet: leave the line empty rather than show garbage
		}
		shown := Convert(value, disp)
		for _, dev := range devices {
			if model, _ := panel.ModelFor(dev.VendorID, dev.ProductID); model != panel.PZ70 {
				continue
			}
			dp := panelFor(pz70, dev.Path)
			switch displayLine(disp.Line) {
			case panel.LineUpper:
				dp.UpperDisplay = &shown
			case panel.LineLower:
				dp.LowerDisplay = &shown
			}
		}
	}

	written := false
	for path, g := range gear {
		g := *g
		key := fmt.Sprintf("gear|%d|%d|%d", g.Upper, g.Left, g.Right)
		if d.last[path] == key {
			continue
		}
		report := panel.EncodePZ55GearLights(g)
		if !d.writeAll(path, report) {
			continue
		}
		d.last[path] = key
		d.log("led", "PZ55 gear lights %s (upper=%d left=%d right=%d)", path, g.Upper, g.Left, g.Right)
		written = true
	}
	for path, dp := range pz70 {
		key := reportKey(dp)
		if d.last[path] == key {
			continue
		}
		report := panel.EncodePZ70Panel(*dp)
		if err := d.panels.Write(path, report); err != nil {
			log.Printf("led: write to %s failed: %v", path, err)
			continue
		}
		d.last[path] = key
		d.log("led", "PZ70 %s lights=%#02x display=%v", path, uint16(dp.Lights), displaySummary(*dp))
		written = true
	}
	return written
}

// panelFor returns the PZ70 output being built for a device, creating it on first
// use.
func panelFor(m map[string]*panel.PZ70Panel, path string) *panel.PZ70Panel {
	p := m[path]
	if p == nil {
		p = &panel.PZ70Panel{}
		m[path] = p
	}
	return p
}

// writeAll writes every report of a multi-report device and reports success only
// when all of them reached the device.
func (d *Driver) writeAll(path string, reports [][]byte) bool {
	for _, r := range reports {
		if err := d.panels.Write(path, r); err != nil {
			log.Printf("led: write to %s failed: %v", path, err)
			return false
		}
	}
	return true
}

// reportKey is a stable key of a PZ70 report: the two display values (or a marker
// for "unset") and the lights. A change in any of them means a redraw.
func reportKey(p *panel.PZ70Panel) string {
	up, lo := "-", "-"
	if p.UpperDisplay != nil {
		up = strconv.Itoa(*p.UpperDisplay)
	}
	if p.LowerDisplay != nil {
		lo = strconv.Itoa(*p.LowerDisplay)
	}
	return fmt.Sprintf("pz70|%s|%s|%d", up, lo, p.Lights)
}

func displaySummary(p panel.PZ70Panel) string {
	up, lo := "--", "--"
	if p.UpperDisplay != nil {
		up = strconv.Itoa(*p.UpperDisplay)
	}
	if p.LowerDisplay != nil {
		lo = strconv.Itoa(*p.LowerDisplay)
	}
	return up + "/" + lo
}

// displayLine maps a stored line name to a panel line.
func displayLine(line string) panel.DisplayLine {
	if strings.EqualFold(line, "lower") {
		return panel.LineLower
	}
	return panel.LineUpper
}

// ReadExportInt reads an integer export from the memory image and applies its mask
// and shift, which is what DCS-BIOS uses to pack several values into one word. It
// is exported so the API can preview a display binding's raw value.
func ReadExportInt(out biosmeta.Output, mem map[uint16]byte) (int, bool) {
	addr := uint16(out.Address)
	lo, ok := mem[addr]
	if !ok {
		return 0, false
	}
	// A one-byte value is common (a whole integer). A wider one is little-endian.
	var v uint16
	if hi, ok := mem[addr+1]; ok {
		v = binary.LittleEndian.Uint16([]byte{lo, hi})
	} else {
		v = uint16(lo)
	}
	if mask := uint16(out.Mask); mask != 0 {
		v &= mask
	}
	if out.ShiftBy > 0 {
		v >>= uint(out.ShiftBy)
	}
	return int(v), true
}

// normalizeMode turns a PZ70 selector control id (KNOB_ALT) into the mode name a
// display binding uses (ALT).
func normalizeMode(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	return strings.TrimPrefix(id, "KNOB_")
}

// Convert applies the binding's scale and offset to a raw exported value and
// rounds it. A zero scale is treated as 1 so a binding without a scale still
// shows the raw value. It is exported so the API previews the same number the
// panel shows.
func Convert(raw int, disp mapping.DisplayBinding) int {
	scale := disp.Scale
	if scale == 0 {
		scale = 1
	}
	return int(round(float64(raw)*scale + float64(disp.Offset)))
}

func round(f float64) float64 {
	if f < 0 {
		return float64(int(f - 0.5))
	}
	return float64(int(f + 0.5))
}

// outputOn reports whether an exported control's first output is non-zero, which
// is DCS-BIOS' convention for an indicator that is lit ("0 if light is off, 1 if
// light is on"). A frame that has not delivered the address yet reads as off.
func outputOn(out biosmeta.Output, mem map[uint16]byte) bool {
	addr := uint16(out.Address)
	lo, ok := mem[addr]
	if !ok {
		return false
	}
	var v uint16 = uint16(lo)
	if hi, ok := mem[addr+1]; ok {
		v = binary.LittleEndian.Uint16([]byte{lo, hi})
	}
	if mask := uint16(out.Mask); mask != 0 {
		return v&mask != 0
	}
	return v != 0
}
