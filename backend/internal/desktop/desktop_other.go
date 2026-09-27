//go:build !windows

// The product targets Windows only: DCS World is a Windows simulator and the
// manager reads its installation and Saved Games folder. This file exists so the
// package still builds and vets on other platforms (CI runs go vet and go test
// on Linux); the real window lives in desktop_windows.go.
package desktop

import (
	"errors"
	"log"
)

func show(string) error {
	return errors.New("a native window is only available on Windows")
}

func notifyAlreadyRunning(url string) {
	log.Printf("desktop: another instance is already running (%s)", url)
}

func notifyWindowUnavailable(url string) {
	log.Printf("desktop: window unavailable; the manager is still at %s", url)
}
