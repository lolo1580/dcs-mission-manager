//go:build windows

package hid

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// reader performs overlapped reads on one device handle.
//
// It issues exactly one read at a time, each with its own auto-reset event and
// its own OVERLAPPED. Sharing those across reads is a trap: a manual-reset event
// stays signalled, so every later wait returns immediately and the read loop
// spins; and reusing an OVERLAPPED whose I/O is still outstanding corrupts the
// completion bookkeeping.
type reader struct {
	h handle
	// reportLen is the device's input report size, from its caps.
	reportLen int
	// buf is the stable destination buffer. It must not move while the kernel
	// may still write into it, so it lives here rather than on the stack.
	buf []byte
}

func newReader(h handle, reportLen int) (*reader, error) {
	if reportLen <= 0 {
		reportLen = 64
	}
	return &reader{h: h, reportLen: reportLen, buf: make([]byte, reportLen)}, nil
}

// close is a no-op: every read owns and releases its own event.
func (rd *reader) close() {}

// read returns one input report, waiting at most timeout. On timeout it cancels
// the read so no I/O is left orphaned on the device.
func (rd *reader) read(timeout time.Duration) ([]byte, error) {
	ev, err := newEvent()
	if err != nil {
		return nil, err
	}
	defer ev.close()

	var ov windows.Overlapped
	ov.HEvent = windows.Handle(ev)

	var done uint32
	r, _, callErr := procReadFile.Call(
		uintptr(rd.h),
		uintptr(unsafe.Pointer(&rd.buf[0])),
		uintptr(len(rd.buf)),
		uintptr(unsafe.Pointer(&done)),
		uintptr(unsafe.Pointer(&ov)),
	)
	if r == 0 {
		if errno, ok := callErr.(windows.Errno); !ok || errno != windows.ERROR_IO_PENDING {
			return nil, fmt.Errorf("hid: ReadFile: %w", callErr)
		}

		w, _, werr := procWaitForSingle.Call(uintptr(ev), uintptr(timeout.Milliseconds()))
		switch uint32(w) {
		case waitObject0:
			// Completed.
		case waitTimeout:
			// Abort and reap, so nothing stays queued on the device.
			procCancelIOEx.Call(uintptr(rd.h), uintptr(unsafe.Pointer(&ov)))
			procWaitForSingle.Call(uintptr(ev), 1000)
			return nil, ErrTimeout
		default:
			return nil, fmt.Errorf("hid: WaitForSingleObject: %w", werr)
		}

		var transferred uint32
		ok, _, gerr := procGetOverlapped.Call(
			uintptr(rd.h),
			uintptr(unsafe.Pointer(&ov)),
			uintptr(unsafe.Pointer(&transferred)),
			0, // already signalled: do not wait again
		)
		if ok == 0 {
			return nil, fmt.Errorf("hid: GetOverlappedResult: %w", gerr)
		}
		if transferred == 0 {
			return nil, ErrTimeout
		}
		return rd.buf[:transferred], nil
	}

	if done == 0 {
		return nil, ErrTimeout
	}
	return rd.buf[:done], nil
}

// event is a Windows auto-reset event.
type event handle

func newEvent() (event, error) {
	// Auto-reset (bManualReset = 0), initially non-signalled, unnamed.
	r, _, err := procCreateEventW.Call(0, 0, 0, 0)
	if r == 0 {
		return 0, fmt.Errorf("hid: CreateEvent failed: %w", err)
	}
	return event(r), nil
}

func (e event) close() { handle(e).close() }
