//go:build windows

// Native window for the manager, backed by the WebView2 runtime.
//
// This file uses github.com/jchv/go-webview2, a pure-Go binding (no CGO, no C
// toolchain) to the WebView2 component that ships with Windows 10 and 11. The
// build only ever targets Windows — DCS is a Windows simulator and the manager
// reads its installation — so no other platform needs a real implementation.
package desktop

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"dcsmm/internal/app"
)

// show opens url in a WebView2 window and blocks until it is closed.
//
// The returned error is non-nil only when the window could not be created at
// all (typically a missing WebView2 runtime). The caller then keeps running
// headless rather than taking the manager down with it.
func show(url string) error {
	// WebView2 requires every call to happen on ONE OS thread, the same one that
	// runs its message loop. Go schedules goroutines across threads freely, so
	// without this lock the window is created on one thread and its completion
	// callback fires on another: that dereferences a half-initialised COM object
	// and crashes the process (observed as an access violation in
	// Chromium.EnvironmentCompleted). Locking the goroutine to its thread for the
	// whole lifetime is what makes it safe.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := waitReady(url, 10*time.Second); err != nil {
		// The server is not answering yet; still open the window (it will
		// retry) but say so rather than showing a blank page for no reason.
		log.Printf("desktop: %v", err)
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "DCS Mission Manager",
			Width:  1280,
			Height: 800,
			Center: true,
		},
	})
	if w == nil {
		return errors.New("could not create the WebView2 window (is the WebView2 runtime installed?)")
	}
	defer w.Destroy()

	w.Navigate(url)
	w.Run()

	// The window is gone: ask the manager to stop so the process does not stay
	// behind with no interface.
	app.Stop()
	return nil
}

// notifyAlreadyRunning brings an existing window to the front, or tells the user
// where the manager is when it is running headless. A second launch exits on
// purpose and a double-click has no console to explain it.
func notifyAlreadyRunning(url string) {
	if focusRunningInstance() {
		return
	}
	msg := "DCS Mission Manager is already running.\n\nOpen " + url +
		" in your browser to reach it."
	messageBox("DCS Mission Manager", msg)
}

// focusRunningInstance raises the window of an already-running instance and
// reports whether one was found. A `serve` instance has no window, in which case
// the caller falls back to a message box.
func focusRunningInstance() bool {
	class := windows.StringToUTF16Ptr("webview")
	title := windows.StringToUTF16Ptr("DCS Mission Manager")
	hwnd, _, _ := findWindowW.Call(
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(title)),
	)
	if hwnd == 0 {
		return false
	}
	const swRestore = 9
	showWindowW.Call(hwnd, swRestore)
	setForegroundWindowW.Call(hwnd)
	return true
}

// notifyWindowUnavailable tells the user that the window could not be created
// (typically a missing WebView2 runtime) and where to find the UI instead.
func notifyWindowUnavailable(url string) {
	msg := "The native window could not be created (the WebView2 runtime may be missing).\n\n" +
		"The manager is still running: open " + url + " in your browser."
	messageBox("DCS Mission Manager", msg)
}

// messageBox shows a modal message box, so the desktop mode never fails
// invisibly.
func messageBox(title, text string) {
	const mbIconInformation = 0x00000040
	messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(title))),
		mbIconInformation,
	)
}

// messageBoxW is user32!MessageBoxW, declared here because the binding package
// keeps its own w32 helpers internal.
var messageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

// Window lookup and raising, declared for the same reason.
var (
	findWindowW          = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")
	showWindowW          = windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow")
	setForegroundWindowW = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")
)

// waitReady polls the health endpoint until the server answers. It matters
// because the window is opened from another goroutine and WebView2 will show a
// connection error if it navigates before the listener is accepting.
func waitReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "api/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("the manager did not answer at %sapi/health within %s", url, timeout)
}
