// Package panelservice watches the Logitech/Saitek flight panels: it finds them,
// keeps a reader running per device, decodes their inputs via internal/panel, and
// publishes the result as events the rest of the manager can consume.
//
// It is the Go counterpart of the C# project's LogitechHidService: the same job,
// with the hot-plug handling that matters in practice (a panel unplugged mid-
// session must not take the manager down, and one plugged back in must work
// again without a restart).
package panelservice

import (
	"time"

	"dcsmanager/internal/panel"
)

// Kind classifies a service event.
type Kind int

const (
	// KindConnected is a panel that appeared (plugged in, or present at start).
	KindConnected Kind = iota
	// KindDisconnected is a panel that went away.
	KindDisconnected
	// KindInput is a control change on a connected panel.
	KindInput
	// KindError is a recoverable failure worth surfacing (a panel that could not
	// be opened, a read that kept failing).
	KindError
)

// String renders a Kind for logs.
func (k Kind) String() string {
	switch k {
	case KindConnected:
		return "connected"
	case KindDisconnected:
		return "disconnected"
	case KindInput:
		return "input"
	case KindError:
		return "error"
	}
	return "unknown"
}

// DeviceInfo is the subset of internal/hid's DeviceInfo the service keeps, so
// callers do not depend on the HID package directly.
type DeviceInfo struct {
	Path         string
	VendorID     uint16
	ProductID    uint16
	Product      string
	Manufacturer string
	Serial       string
}

// Event is what the service publishes.
type Event struct {
	Kind Kind
	// At is when it happened.
	At time.Time
	// Device identifies the panel instance: stable for the life of the connection.
	Device string
	// Model is the panel model, when it is a supported one.
	Model panel.Model
	// Info is the device description, on connect.
	Info DeviceInfo
	// Input is the decoded change, on KindInput.
	Input panel.Event
	// Active is the full active state of every control of that model, on
	// KindInput. An output binding that has no dedicated input event of its own
	// (a two-position lever) can be read from here.
	Active map[string]bool
	// Err carries the failure, on KindError.
	Err error
}

// Options configures the service.
type Options struct {
	// PollInterval is how often the device list is rescanned for hot-plug. Panels
	// appear on USB, so a couple of seconds is responsive without being busy.
	PollInterval time.Duration
	// ReadTimeout is how long a single read waits before it is considered idle.
	ReadTimeout time.Duration
}

// DefaultOptions returns the options the manager runs with.
func DefaultOptions() Options {
	return Options{
		PollInterval: 2 * time.Second,
		ReadTimeout:  500 * time.Millisecond,
	}
}
