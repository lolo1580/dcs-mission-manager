//go:build !windows

// The manager is a Windows-only application in practice, but the Go build must
// still compile elsewhere: this stub keeps the package importable on other
// platforms, where HID panels do not exist anyway.
package hid

import (
	"time"
)

// Device is an opened HID collection.
type Device struct {
	info DeviceInfo
	caps Caps
}

// Enumerate always fails outside Windows.
func Enumerate() ([]DeviceInfo, error) { return nil, ErrUnsupported }

// Open always fails outside Windows.
func Open(string) (*Device, error) { return nil, ErrUnsupported }

// Info returns what enumeration found for this device.
func (d *Device) Info() DeviceInfo { return d.info }

// Caps returns the report layout read when the device was opened.
func (d *Device) Caps() Caps { return d.caps }

// Read always fails outside Windows.
func (d *Device) Read([]byte, time.Duration) (int, error) { return 0, ErrUnsupported }

// Write always fails outside Windows.
func (d *Device) Write([]byte) (int, error) { return 0, ErrUnsupported }

// Close is a no-op outside Windows.
func (d *Device) Close() error { return nil }
