// Package hid gives the manager access to Windows HID devices — flight
// simulator panels, in practice — with no cgo and no external dependency beyond
// golang.org/x/sys/windows.
//
// It is a small, deliberate subset of what a full HID library offers: enumerate
// the present HID collections, read their product identity, open one, and read
// its input reports with a timeout. That is exactly what driving a Logitech
// PZ55/PZ70 panel needs, and it keeps the single-binary, CGO_ENABLED=0 build.
package hid

import "errors"

// ErrTimeout is returned by Read when no report arrived in time. Callers treat it
// as "nothing happened", not as a failure.
var ErrTimeout = errors.New("hid: read timed out")

// ErrUnsupported is returned by every call on a platform without Windows HID
// support (the manager is Windows-only in practice; the stub keeps it building
// elsewhere). It is a "nothing to see here" signal, not a failure, so callers
// like panelservice must not turn it into an error the user sees.
var ErrUnsupported = errors.New("hid: only supported on Windows")

// DeviceInfo describes a HID collection found on the machine.
type DeviceInfo struct {
	// Path is the Windows device interface path (\\?\hid#...), the handle to open.
	Path string
	// VendorID and ProductID identify the maker and model (0x06A3 is Saitek).
	VendorID  uint16
	ProductID uint16
	// Version is the device's own version number.
	Version uint16
	// Manufacturer and Product are the strings the device reports.
	Manufacturer string
	Product      string
	// Serial is the instance-specific serial, when the device has one.
	Serial string
	// UsagePage and Usage come from the top-level collection caps.
	UsagePage uint16
	Usage     uint16
}

// Caps describes a collection's report layout, from HidP_GetCaps.
type Caps struct {
	UsagePage            uint16
	Usage                uint16
	InputReportByteLen   int
	OutputReportByteLen  int
	FeatureReportByteLen int
}
