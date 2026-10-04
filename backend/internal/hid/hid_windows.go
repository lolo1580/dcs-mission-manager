//go:build windows

package hid

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hidGUID is the HID class interface GUID:
// 4d1e55b2-f16f-11cf-88cb-001111000030
var hidGUID = windows.GUID{
	Data1: 0x4d1e55b2,
	Data2: 0xf16f,
	Data3: 0x11cf,
	Data4: [8]byte{0x88, 0xcb, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30},
}

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010

	genericRead  = 0x80000000
	genericWrite = 0x40000000
	openExisting = 3
	fileShareAll = 0x00000001 | 0x00000002
	// fileFlagOverlapped makes reads asynchronous; without it, ReadFile on a HID
	// handle blocks until the device sends something.
	fileFlagOverlapped = 0x40000000

	invalidHandle = ^uintptr(0)

	// HidD_Get* need a buffer, not a length; these bound the strings they return.
	maxHIDString = 256

	errorInsufficientBuffer = 122
	waitObject0             = 0
	waitTimeout             = 258
)

var (
	setupapi = windows.NewLazySystemDLL("setupapi.dll")
	hidDLL   = windows.NewLazySystemDLL("hid.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetClassDevs    = setupapi.NewProc("SetupDiGetClassDevsW")
	procEnumInterfaces  = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procGetInterfaceDet = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procDestroyList     = setupapi.NewProc("SetupDiDestroyDeviceInfoList")

	procHidDGetAttributes     = hidDLL.NewProc("HidD_GetAttributes")
	procHidDGetManufacturer   = hidDLL.NewProc("HidD_GetManufacturerString")
	procHidDGetProductString  = hidDLL.NewProc("HidD_GetProductString")
	procHidDGetSerialNumber   = hidDLL.NewProc("HidD_GetSerialNumberString")
	procHidDGetPreparsedData  = hidDLL.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData = hidDLL.NewProc("HidD_FreePreparsedData")
	procHidDSetFeature        = hidDLL.NewProc("HidD_SetFeature")
	procHidPGetCaps           = hidDLL.NewProc("HidP_GetCaps")

	procCreateFileW   = kernel32.NewProc("CreateFileW")
	procReadFile      = kernel32.NewProc("ReadFile")
	procWriteFile     = kernel32.NewProc("WriteFile")
	procCloseHandle   = kernel32.NewProc("CloseHandle")
	procCreateEventW  = kernel32.NewProc("CreateEventW")
	procWaitForSingle = kernel32.NewProc("WaitForSingleObject")
	procGetOverlapped = kernel32.NewProc("GetOverlappedResult")
	procCancelIOEx    = kernel32.NewProc("CancelIoEx")
)

// handle is an open Windows object handle.
type handle uintptr

func (h handle) close() error {
	if h == 0 || uintptr(h) == invalidHandle {
		return nil
	}
	r, _, err := procCloseHandle.Call(uintptr(h))
	if r == 0 {
		return err
	}
	return nil
}

// These mirror the C structs. Sizes and field order matter: they are passed
// straight to the Win32 API.
type spDeviceInterfaceData struct {
	cbSize             uint32
	interfaceClassGUID windows.GUID
	flags              uint32
	reserved           uintptr
}

type hidAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

// Enumerate returns every present HID collection on the machine. A device that
// cannot be described is skipped rather than failing the whole call: a broken
// peripheral must not hide the working ones.
func Enumerate() ([]DeviceInfo, error) {
	h, _, _ := procGetClassDevs.Call(
		uintptr(unsafe.Pointer(&hidGUID)), 0, 0,
		uintptr(digcfPresent|digcfDeviceInterface),
	)
	if h == 0 || h == invalidHandle {
		return nil, errors.New("hid: SetupDiGetClassDevs failed")
	}
	defer procDestroyList.Call(h)

	buf := make([]byte, 2048)
	var out []DeviceInfo

	for i := 0; ; i++ {
		var iface spDeviceInterfaceData
		iface.cbSize = uint32(unsafe.Sizeof(iface))
		// Signature: (DeviceInfoSet, ClassGuid, MemberIndex, DeviceInterfaceData).
		r, _, _ := procEnumInterfaces.Call(h, 0,
			uintptr(unsafe.Pointer(&hidGUID)), uintptr(i),
			uintptr(unsafe.Pointer(&iface)))
		if r == 0 {
			break // ERROR_NO_MORE_ITEMS
		}

		path, ok := interfacePath(h, &iface, buf)
		if !ok || path == "" {
			continue
		}
		// The VID/PID are right there in the path (\\?\hid#vid_06a3&pid_0d06#…),
		// so they are read without opening the device. Opening each collection in
		// turn fails for those another process holds (keyboards, mice) and would
		// leave half the list unidentified.
		info := DeviceInfo{Path: path}
		info.VendorID, info.ProductID = parseIDs(path)
		out = append(out, info)
	}
	return out, nil
}

// parseIDs extracts the VID and PID from a HID interface path. Windows spells
// them lowercase hex: \\?\hid#vid_06a3&pid_0d06#…
func parseIDs(path string) (vendor, product uint16) {
	lower := strings.ToLower(path)
	vi := strings.Index(lower, "vid_")
	pi := strings.Index(lower, "pid_")
	if vi < 0 || pi < 0 {
		return 0, 0
	}
	vendor = parseHex16(lower[vi+4:])
	product = parseHex16(lower[pi+4:])
	return vendor, product
}

// parseHex16 reads up to four hex digits at the start of s.
func parseHex16(s string) uint16 {
	var n uint32
	for i := 0; i < 4 && i < len(s); i++ {
		var d uint32
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		default:
			return uint16(n)
		}
		n = n<<4 | d
	}
	return uint16(n)
}

// interfacePath performs the two-call dance SetupDiGetDeviceInterfaceDetailW
// requires: once to learn the size, then to get the path.
func interfacePath(h uintptr, iface *spDeviceInterfaceData, buf []byte) (string, bool) {
	// cbSize is 8 on 64-bit (DWORD + padding). The path itself starts at offset
	// 4, NOT 8: the struct is `DWORD cbSize; WCHAR DevicePath[]`, and the size
	// only rounds up to 8 for alignment. Reading at 8 yields a NUL immediately
	// and an empty path — a silent, confusing failure.
	*(*uint32)(unsafe.Pointer(&buf[0])) = 8

	var need uint32
	// Signature: (DeviceInfoSet, DeviceInterfaceData, DetailData, DetailSize,
	//             RequiredSize, DeviceInfoData).
	r, _, err := procGetInterfaceDet.Call(h,
		uintptr(unsafe.Pointer(iface)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&need)), 0)
	if r == 0 {
		if errno, ok := err.(windows.Errno); ok && uint32(errno) == errorInsufficientBuffer && need > uint32(len(buf)) {
			// A path longer than our buffer: ask again with the right size.
			big := make([]byte, need)
			*(*uint32)(unsafe.Pointer(&big[0])) = 8
			r, _, _ = procGetInterfaceDet.Call(h,
				uintptr(unsafe.Pointer(iface)),
				uintptr(unsafe.Pointer(&big[0])), uintptr(len(big)),
				uintptr(unsafe.Pointer(&need)), 0)
			if r == 0 {
				return "", false
			}
			return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(&big[4]))), true
		}
		return "", false
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(&buf[4]))), true
}

// fillIdentity opens the device just long enough to read its descriptors.
func fillIdentity(info *DeviceInfo) error {
	h, err := openPath(info.Path)
	if err != nil {
		return err
	}
	defer h.close()

	var attrs hidAttributes
	attrs.Size = uint32(unsafe.Sizeof(attrs))
	if r, _, _ := procHidDGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&attrs))); r == 0 {
		return errors.New("hid: HidD_GetAttributes failed")
	}
	info.VendorID = attrs.VendorID
	info.ProductID = attrs.ProductID
	info.Version = attrs.VersionNumber

	info.Manufacturer = hidString(procHidDGetManufacturer, h)
	info.Product = hidString(procHidDGetProductString, h)
	info.Serial = hidString(procHidDGetSerialNumber, h)
	return nil
}

// hidString reads one of the HidD_Get*String descriptors.
func hidString(proc *windows.LazyProc, h handle) string {
	buf := make([]uint16, maxHIDString)
	if r, _, _ := proc.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2)); r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// openPath opens a device interface path with the sharing a panel needs: other
// tools may hold it, and we may want read and write (output reports).
//
// FILE_FLAG_OVERLAPPED is essential, not decorative: without it Windows ignores
// the OVERLAPPED passed to ReadFile and performs a blocking read instead. On an
// idle panel — which sends nothing until a switch moves — that blocks forever.
func openPath(path string) (handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, _, callErr := procCreateFileW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(genericRead|genericWrite),
		uintptr(fileShareAll),
		0, openExisting, uintptr(fileFlagOverlapped), 0,
	)
	if uintptr(h) == invalidHandle {
		return 0, fmt.Errorf("hid: cannot open %s: %w", path, callErr)
	}
	return handle(h), nil
}

// Device is an opened HID collection.
type Device struct {
	info   DeviceInfo
	caps   Caps
	handle handle
	rd     *reader
}

// Info returns what enumeration found for this device.
func (d *Device) Info() DeviceInfo { return d.info }

// Caps returns the report layout read when the device was opened.
func (d *Device) Caps() Caps { return d.caps }

// Close releases the handle and the overlapped state.
func (d *Device) Close() error {
	d.rd.close()
	return d.handle.close()
}

// Open opens a HID collection by its interface path and reads its capabilities.
func Open(path string) (*Device, error) {
	h, err := openPath(path)
	if err != nil {
		return nil, err
	}
	caps, err := readCaps(h)
	if err != nil {
		h.close()
		return nil, err
	}
	info := DeviceInfo{Path: path}
	info.VendorID, info.ProductID = parseIDs(path)
	_ = fillIdentity(&info)

	rd, err := newReader(h, caps.InputReportByteLen)
	if err != nil {
		h.close()
		return nil, err
	}
	return &Device{info: info, caps: caps, handle: h, rd: rd}, nil
}

func readCaps(h handle) (Caps, error) {
	// HidP_GetCaps does NOT take the device handle: it takes the "preparsed data"
	// HID allocates for the collection. Passing the handle crashes the process —
	// the API writes into it as though it were a preparsed structure.
	var preparsed uintptr
	r, _, _ := procHidDGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&preparsed)))
	if r == 0 || preparsed == 0 {
		return Caps{}, errors.New("hid: HidD_GetPreparsedData failed")
	}
	defer procHidDFreePreparsedData.Call(preparsed)

	var c hidpCaps
	r, _, _ = procHidPGetCaps.Call(preparsed, uintptr(unsafe.Pointer(&c)))
	if r == 0 {
		return Caps{}, errors.New("hid: HidP_GetCaps failed")
	}
	return Caps{
		UsagePage:            c.UsagePage,
		Usage:                c.Usage,
		InputReportByteLen:   int(c.InputReportByteLength),
		OutputReportByteLen:  int(c.OutputReportByteLength),
		FeatureReportByteLen: int(c.FeatureReportByteLength),
	}, nil
}

// Read reads one input report, waiting at most timeout. A timeout returns
// ErrTimeout, treated as "nothing happened" rather than a failure.
//
// The read is overlapped: ReadFile on a HID handle blocks indefinitely when the
// device has nothing to send, and an idle panel emits nothing for long stretches.
// A synchronous read freezes the caller — which is exactly what a first attempt
// did, hanging the process.
func (d *Device) Read(buf []byte, timeout time.Duration) (int, error) {
	report, err := d.rd.read(timeout)
	if err != nil {
		return 0, err
	}
	n := copy(buf, report)
	return n, nil
}

// Write sends one output report (a feature/output report driving an LED or LCD).
func (d *Device) Write(report []byte) (int, error) {
	if len(report) == 0 {
		return 0, errors.New("hid: empty report")
	}
	var written uint32
	r, _, callErr := procWriteFile.Call(
		uintptr(d.handle),
		uintptr(unsafe.Pointer(&report[0])),
		uintptr(len(report)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r == 0 {
		// A feature report goes through HidD_SetFeature; some drivers refuse a
		// plain WriteFile on the output report.
		if sr, _, _ := procHidDSetFeature.Call(uintptr(d.handle),
			uintptr(unsafe.Pointer(&report[0])), uintptr(len(report))); sr != 0 {
			return len(report), nil
		}
		return 0, fmt.Errorf("hid: WriteFile: %w", callErr)
	}
	return int(written), nil
}
