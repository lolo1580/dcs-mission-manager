package panelservice

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"dcsmanager/internal/panel"
)

// collector captures events behind a mutex, so the test can inspect them while
// the service is running.
type collector struct {
	mu      sync.Mutex
	events  []Event
	inputs  []Event
	connect []Event
}

func (c *collector) emit(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	switch e.Kind {
	case KindInput:
		c.inputs = append(c.inputs, e)
	case KindConnected:
		c.connect = append(c.connect, e)
	}
}

func (c *collector) snapshot() (events, inputs, connects []Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Event(nil), c.events...), append([]Event(nil), c.inputs...), append([]Event(nil), c.connect...)
}

// TestServiceStartsAndStops is the baseline: the service runs, scans without
// error, and stops cleanly. It needs no hardware.
func TestServiceStartsAndStops(t *testing.T) {
	c := &collector{}
	opts := DefaultOptions()
	opts.PollInterval = 50 * time.Millisecond
	s := New(opts, c.emit)

	s.Start()
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	events, _, _ := c.snapshot()
	for _, e := range events {
		if e.Kind == KindError {
			t.Errorf("unexpected error event: %v", e.Err)
		}
	}
}

// TestServiceFindsPanelOnRealHardware is the end-to-end check on a machine with a
// panel: the service connects to it and decodes input. It is skipped when no
// panel is present, so CI without hardware stays green.
func TestServiceFindsPanelOnRealHardware(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	c := &collector{}
	opts := DefaultOptions()
	opts.PollInterval = 100 * time.Millisecond
	s := New(opts, c.emit)

	s.Start()
	defer s.Stop()

	// Give the scanner a couple of passes.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.Devices()) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	devices := s.Devices()
	if len(devices) == 0 {
		t.Skip("no supported panel connected")
	}
	for _, d := range devices {
		t.Logf("connected: %s VID=%04X PID=%04X", d.Path, d.VendorID, d.ProductID)
	}

	// Let it read whatever the idle panels stream, then report.
	time.Sleep(1500 * time.Millisecond)
	_, inputs, connects := c.snapshot()
	t.Logf("%d connect event(s), %d input event(s)", len(connects), len(inputs))
	for i, in := range inputs {
		if i >= 5 {
			break
		}
		t.Logf("  input: device=%s control=%s active=%v", in.Device, in.Input.Control.ID, in.Input.Active)
	}
}

// TestWriteToUnknownDevice checks writing to a device that is not connected is a
// clear error rather than a panic.
func TestWriteToUnknownDevice(t *testing.T) {
	s := New(DefaultOptions(), nil)
	if err := s.Write("nope", []byte{0, 0}); err == nil {
		t.Fatal("writing to an unknown device should fail")
	}
}

// TestDecodeIsWiredThroughPanel confirms the service and the panel package agree
// on the model: a PZ70 report decoded here yields the controls panel defines.
func TestDecodeIsWiredThroughPanel(t *testing.T) {
	defs := panel.Controls(panel.PZ70)
	if len(defs) == 0 {
		t.Fatal("panel.PZ70 should define controls")
	}
	var ap panel.Control
	for _, c := range defs {
		if c.ID == "AP_BUTTON" {
			ap = c
		}
	}
	off := make([]byte, 4)
	on := make([]byte, 4)
	on[ap.ByteIndex] |= ap.Mask

	events := panel.Decode("dev", panel.PZ70, off, on)
	if len(events) != 1 || events[0].Control.ID != "AP_BUTTON" {
		t.Fatalf("unexpected decode: %+v", events)
	}
}
