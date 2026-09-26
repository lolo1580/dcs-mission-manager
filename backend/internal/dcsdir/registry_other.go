//go:build !windows

package dcsdir

import "errors"

// regQuery is unavailable outside Windows. DCS only runs on Windows, but the
// code must still compile elsewhere so the project can be built and tested on
// any machine (and in CI).
func regQuery() (string, error) {
	return "", errors.New("registry lookup is only available on Windows")
}
