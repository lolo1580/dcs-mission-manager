//go:build windows

package content

import "os/exec"

// revealPath opens a folder in Explorer. The path has already been resolved
// under an allowed root by the caller.
func revealPath(path string) error {
	return exec.Command("explorer.exe", path).Start()
}
