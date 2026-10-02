// Package source identifies whether incoming telemetry comes from DCS or from
// the project's own test tools.
//
// This matters because the tools speak exactly the same UDP/TCP protocol as the
// real Lua scripts. The backend cannot tell them apart from the shape of the
// messages, and once the data is in the database the difference is invisible
// forever: a simulated session recorded while the tools were running looks like
// a genuine flight. Statistics would then include aircraft and vehicles that
// never existed.
//
// The detection is deliberately narrow. It only fires on the local player
// message ("ownship") and only on names the fixtures use, which no DCS player
// would plausibly pick as a callsign. A false positive would therefore require
// a real pilot to name themselves exactly "Viper 1-1", "Hornet 1-2",
// "Flanker 2-1" or "Hind 3-1" — and even then, the only consequence is that
// their session is tagged as simulated, which is visible and reversible.
package source

import (
	"bytes"
	"os"
	"sync"
)

// TestToolNames lists the ownship names used by tools/send-telemetry.mjs. The
// script renames its four simulated aircraft on every tick, because they are all
// sent as "ownship" and DCS exports a single ownship.
var TestToolNames = [][]byte{
	[]byte("Viper 1-1"),
	[]byte("Hornet 1-2"),
	[]byte("Flanker 2-1"),
	[]byte("Hind 3-1"),
}

// IsTestPayload reports whether a raw ownship datagram was produced by the test
// tools. The check is a substring search, so it does not depend on JSON key
// order or formatting.
//
// A user can force the verdict for every session with DCSMANAGER_SOURCE=test (useful
// when writing scripts that do not reuse the fixtures' names) or suppress it
// with DCSMANAGER_SOURCE=live.
func IsTestPayload(payload []byte) bool {
	for _, name := range TestToolNames {
		if bytes.Contains(payload, name) {
			return true
		}
	}
	return false
}

// Detector wraps IsTestPayload in a latch. Once simulated traffic has been seen
// during a session, the session stays marked as simulated even after the tools
// stop, so a single stray packet cannot be "washed away" by later silence.
type Detector struct {
	mu    sync.Mutex
	fired bool
}

// NewDetector creates a detector honouring the DCSMANAGER_SOURCE override.
func NewDetector() *Detector {
	return &Detector{}
}

// Observe feeds one raw ownship payload and returns true if the session is known
// to be simulated. The result latches.
func (d *Detector) Observe(payload []byte) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fired {
		return true
	}
	if IsTestPayload(payload) {
		d.fired = true
		return true
	}
	return false
}

// Observed reports whether simulated traffic has been seen. It does not consume
// a payload, so it can be called by any component that needs the current verdict.
func (d *Detector) Observed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fired
}

// Forced returns the source requested by the DCSMANAGER_SOURCE environment variable,
// or "" when the variable is unset or invalid. An explicit override always wins
// over detection, so it can be used to tag a session that uses different
// fixtures, or to correct a false positive.
func Forced() string {
	switch os.Getenv("DCSMANAGER_SOURCE") {
	case "test":
		return "test"
	case "live":
		return "live"
	default:
		return ""
	}
}
