//go:build windows

package dcsdir

import (
	"os/exec"
	"strings"
)

// registryKeys are the products to probe, most recent first. A machine may have
// both the stable and the openbeta build installed.
var registryKeys = []string{
	`HKCU\Software\Eagle Dynamics\DCS World OpenBeta`,
	`HKCU\Software\Eagle Dynamics\DCS World`,
}

// regQuery asks reg.exe for the install paths. Using the command rather than the
// Windows API keeps the build free of a dependency, which is the rule everywhere
// else in this project.
func regQuery() (string, error) {
	var out strings.Builder
	for _, key := range registryKeys {
		//nolint:gosec // the key is a constant, never user input
		cmd := exec.Command("reg", "query", key, "/v", "Path")
		b, err := cmd.Output()
		if err != nil {
			continue
		}
		out.Write(b)
	}
	return out.String(), nil
}
