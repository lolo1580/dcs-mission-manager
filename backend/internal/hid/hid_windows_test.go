package hid

import (
	"runtime"
	"testing"
	"time"
)

// saitek is the vendor id Logitech/Saitek flight panels use.
const saitek = 0x06A3

// TestEnumerateFindsHIDDevices checks enumeration works and returns at least one
// HID collection on a normal Windows machine (keyboard, mouse…). It is skipped
// off Windows.
func TestEnumerateFindsHIDDevices(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	devices, err := Enumerate()
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("no HID device on this machine")
	}
	// Every entry must carry the path CreateFile needs. A virtual HID collection
	// has no VID at all, so only the path is mandatory.
	for _, d := range devices {
		if d.Path == "" {
			t.Errorf("device without a path: %+v", d)
		}
	}
	t.Logf("%d HID collection(s)", len(devices))
	for _, d := range devices {
		if d.VendorID == saitek {
			t.Logf("  Saitek: VID=%04X PID=%04X %q", d.VendorID, d.ProductID, d.Product)
		}
	}
}

// TestOpenAndReadSaitekPanel exercises the full path on a real panel: open, read
// the caps, then read reports with a timeout. It is skipped when no Saitek panel
// is connected, so it is harmless on a machine without one — but on a machine
// with a PZ55/PZ70 it is what proves the pure-Go HID path really drives it.
func TestOpenAndReadSaitekPanel(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	devices, err := Enumerate()
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	var panel DeviceInfo
	for _, d := range devices {
		if d.VendorID == saitek {
			panel = d
			break
		}
	}
	if panel.Path == "" {
		t.Skip("no Saitek panel connected")
	}

	dev, err := Open(panel.Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dev.Close()

	caps := dev.Caps()
	t.Logf("panel PID=%04X product=%q input=%d output=%d feature=%d",
		panel.ProductID, panel.Product, caps.InputReportByteLen, caps.OutputReportByteLen, caps.FeatureReportByteLen)
	if caps.InputReportByteLen == 0 {
		t.Error("caps should report an input report length")
	}

	// The panels stream their full state while idle, so a generous timeout should
	// yield at least one report without touching any control.
	buf := make([]byte, 64)
	deadline := time.Now().Add(5 * time.Second)
	reads := 0
	for time.Now().Before(deadline) {
		n, err := dev.Read(buf, 500*time.Millisecond)
		if err == ErrTimeout {
			continue
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		reads++
		if reads == 1 {
			t.Logf("first report (%d bytes): % X", n, buf[:n])
		}
	}
	if reads == 0 {
		t.Log("no report within 5s: the panel may be idle or held by another tool")
	} else {
		t.Logf("read %d report(s)", reads)
	}
}
