//go:build windows

package hid

import (
	"fmt"
	"testing"
	"unsafe"
)

// TestIdentityStringsOnRealPanel investigates why HidD_GetProductString returns
// empty: it compares a handle opened with FILE_FLAG_OVERLAPPED against one opened
// without, which is the documented-sounding difference between the two.
func TestIdentityStringsOnRealPanel(t *testing.T) {
	devices, err := Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	var path string
	for _, d := range devices {
		if d.VendorID == saitek {
			path = d.Path
			break
		}
	}
	if path == "" {
		t.Skip("no Saitek panel connected")
	}

	// Overlapped handle (what Open uses).
	ov, err := openPath(path)
	if err != nil {
		t.Fatalf("overlapped open: %v", err)
	}
	defer ov.close()
	t.Logf("overlapped: product=%q manufacturer=%q serial=%q",
		hidString(procHidDGetProductString, ov),
		hidString(procHidDGetManufacturer, ov),
		hidString(procHidDGetSerialNumber, ov))

	// Non-overlapped handle, for comparison.
	p, _ := windowsUTF16Ptr(path)
	r, _, callErr := procCreateFileW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(genericRead),
		uintptr(fileShareAll),
		0, openExisting, 0, 0)
	if uintptr(r) == invalidHandle {
		t.Skipf("plain open failed: %v", callErr)
	}
	plain := handle(r)
	defer plain.close()
	t.Logf("plain:      product=%q manufacturer=%q serial=%q",
		hidString(procHidDGetProductString, plain),
		hidString(procHidDGetManufacturer, plain),
		hidString(procHidDGetSerialNumber, plain))

	var attrs hidAttributes
	attrs.Size = uint32(unsafe.Sizeof(attrs))
	if r, _, _ := procHidDGetAttributes.Call(uintptr(plain), uintptr(unsafe.Pointer(&attrs))); r != 0 {
		t.Logf("plain attrs: VID=%04X PID=%04X", attrs.VendorID, attrs.ProductID)
	}
}

func windowsUTF16Ptr(s string) (*uint16, error) {
	buf := make([]uint16, len(s)+1)
	for i, r := range s {
		buf[i] = uint16(r)
	}
	return &buf[0], nil
}

var _ = fmt.Sprintf
