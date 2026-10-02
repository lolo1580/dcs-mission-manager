package panelservice

import (
	"bytes"
	"testing"
	"time"

	"dcsmanager/internal/hid"
	"dcsmanager/internal/panel"
)

// fakeDevice feeds a scripted sequence of reports to the read loop, then behaves
// as idle (timeouts). It lets the whole chain — scan, open, read, decode, emit —
// be exercised without hardware.
type fakeDevice struct {
	reports [][]byte
	next    int
	closed  bool
	written [][]byte
}

func (f *fakeDevice) Read(buf []byte, timeout time.Duration) (int, error) {
	if f.closed {
		return 0, errClosed
	}
	if f.next >= len(f.reports) {
		// Idle: a real panel does the same, and the loop must treat it as nothing
		// to report rather than a failure.
		time.Sleep(time.Millisecond)
		return 0, hid.ErrTimeout
	}
	r := f.reports[f.next]
	f.next++
	return copy(buf, r), nil
}

func (f *fakeDevice) Write(report []byte) (int, error) {
	f.written = append(f.written, append([]byte(nil), report...))
	return len(report), nil
}

func (f *fakeDevice) Close() error {
	f.closed = true
	return nil
}

// Info mirrors what hid.Device.Info returns once opened.
func (f *fakeDevice) Info() hid.DeviceInfo {
	return hid.DeviceInfo{
		Path:      panelPath,
		VendorID:  panel.VendorID,
		ProductID: panel.ProductPZ70,
		Product:   "Logitech Flight Multi Panel (fake)",
	}
}

var errClosed = &closedErr{}

type closedErr struct{}

func (*closedErr) Error() string { return "closed" }

// panelPath is a path the fake enumerate reports.
const panelPath = `\\?\hid#vid_06a3&pid_0d06#fake`

// newFakeService builds a service wired to one fake PZ70.
func newFakeService(fake *fakeDevice, c *collector) *Service {
	s := New(Options{PollInterval: 20 * time.Millisecond, ReadTimeout: 20 * time.Millisecond}, c.emit)
	s.enumerate = func() ([]hid.DeviceInfo, error) {
		return []hid.DeviceInfo{{Path: panelPath, VendorID: panel.VendorID, ProductID: panel.ProductPZ70}}, nil
	}
	s.openDevice = func(string) (readWriter, error) { return fake, nil }
	return s
}

// TestReadLoopDecodesAndEmits is the end-to-end proof without hardware: a report
// sequence goes in, the matching input events come out.
func TestReadLoopDecodesAndEmits(t *testing.T) {
	// Find the AP button's bit so the report is built from the real definition,
	// not a guessed offset.
	var ap panel.Control
	for _, c := range panel.Controls(panel.PZ70) {
		if c.ID == "AP_BUTTON" {
			ap = c
		}
	}
	if ap.ID == "" {
		t.Fatal("AP_BUTTON should be defined")
	}

	idle := []byte{0, 0, 0, 0}
	pressed := []byte{0, 0, 0, 0}
	pressed[ap.ByteIndex] |= ap.Mask

	fake := &fakeDevice{reports: [][]byte{idle, pressed, idle, idle}}
	c := &collector{}
	s := newFakeService(fake, c)
	s.Start()
	defer s.Stop()

	// Wait for the press and release to be decoded.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, inputs, _ := c.snapshot()
		if len(inputs) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	_, inputs, connects := c.snapshot()
	if len(connects) != 1 {
		t.Fatalf("want 1 connect event, got %d", len(connects))
	}
	if len(inputs) != 2 {
		t.Fatalf("want a press and a release, got %d: %+v", len(inputs), inputs)
	}
	if !inputs[0].Input.Active || inputs[0].Input.Control.ID != "AP_BUTTON" {
		t.Errorf("first event should be the AP press: %+v", inputs[0].Input)
	}
	if inputs[1].Input.Active {
		t.Errorf("second event should be the release: %+v", inputs[1].Input)
	}
	if inputs[0].Model != panel.PZ70 {
		t.Errorf("model = %q, want pz70", inputs[0].Model)
	}
}

// TestWriteGoesThroughTheService checks an output report reaches the device, with
// the service serialising it rather than the caller touching the handle.
func TestWriteGoesThroughTheService(t *testing.T) {
	fake := &fakeDevice{reports: [][]byte{{0, 0, 0, 0}}}
	c := &collector{}
	s := newFakeService(fake, c)
	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(s.Devices()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if len(s.Devices()) == 0 {
		t.Fatal("the fake panel should have connected")
	}

	report := panel.EncodePZ70Panel(panel.PZ70Panel{Lights: panel.PZ70LightAP})
	if err := s.Write(panelPath, report); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(fake.written) != 1 || !bytes.Equal(fake.written[0], report) {
		t.Fatalf("the report did not reach the device: %+v", fake.written)
	}
}

// TestUnplugPublishesDisconnect checks a panel that disappears is closed and
// reported, once.
func TestUnplugPublishesDisconnect(t *testing.T) {
	fake := &fakeDevice{reports: [][]byte{{0, 0, 0, 0}}}
	c := &collector{}
	s := newFakeService(fake, c)

	// Enumerate returns the panel, then nothing: an unplug.
	var once bool
	s.enumerate = func() ([]hid.DeviceInfo, error) {
		if once {
			return nil, nil
		}
		once = true
		return []hid.DeviceInfo{{Path: panelPath, VendorID: panel.VendorID, ProductID: panel.ProductPZ70}}, nil
	}

	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		events, _, _ := c.snapshot()
		var connected, disconnected int
		for _, e := range events {
			switch e.Kind {
			case KindConnected:
				connected++
			case KindDisconnected:
				disconnected++
			}
		}
		if connected == 1 && disconnected == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	events, _, _ := c.snapshot()
	var connected, disconnected int
	for _, e := range events {
		switch e.Kind {
		case KindConnected:
			connected++
		case KindDisconnected:
			disconnected++
		}
	}
	if connected != 1 || disconnected != 1 {
		t.Fatalf("want 1 connect and 1 disconnect, got %d and %d: %+v", connected, disconnected, events)
	}
	if !fake.closed {
		t.Error("the device should have been closed on disconnect")
	}
}
