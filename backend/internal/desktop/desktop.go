// Package desktop opens the manager in a native window instead of a browser
// tab. The window hosts the very same embedded web UI the HTTP server already
// serves, through the WebView2 runtime that ships with Windows 10 and 11.
//
// The HTTP server keeps running behind the window: it is how the page reaches
// the API, and it is what still lets a second monitor or a tablet open the UI.
// The desktop mode simply replaces "open your browser at localhost:8080" with a
// window the user can double-click.
package desktop

import (
	"errors"
	"io"
	"log"
	"os"

	"dcsmanager/internal/app"
	"dcsmanager/internal/config"
)

// Run starts the manager and shows it in a native window. It blocks until the
// window is closed, then returns.
//
// It never dies silently: a second launch (another instance already running) is
// reported on screen and in the log, and if the WebView2 runtime is missing it
// falls back to headless mode so the manager stays reachable from a browser.
func Run() {
	// A double-clicked executable has no console, so errors would vanish. Keep a
	// log file next to the database; this is where "it does not start" gets an
	// answer.
	if path, err := app.LogFilePath(); err == nil {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			defer f.Close()
			// The file comes first: io.MultiWriter stops at the first writer that
			// errors, and a double-clicked exe may have no usable stderr. Writing
			// the file first guarantees the log survives that case.
			log.SetOutput(io.MultiWriter(f, os.Stderr))
			log.Printf("desktop: log file at %s", path)
		}
	}

	err := app.Run(func(addr string) {
		url := "http://" + addr + "/"
		log.Printf("desktop: opening %s in a native window", url)
		if err := show(url); err != nil {
			log.Printf("desktop: window unavailable (%v); open %s in your browser instead", err, url)
			notifyWindowUnavailable(url)
		}
	})
	if errors.Is(err, app.ErrAlreadyRunning) {
		url := "http://" + config.Load().HTTPAddr + "/"
		log.Printf("desktop: another instance is already running (%s)", url)
		// A double-click would otherwise look like nothing happened: say so.
		notifyAlreadyRunning(url)
	}
}
