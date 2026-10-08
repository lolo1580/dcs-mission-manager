//go:build windows

package install

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strings"
	"unsafe"
)

// DCSRunning prevents changing on-disk exports while DCS has loaded an older
// copy. Checking the process also covers the menu, before any export connects.
func DCSRunning() (bool, error) {
	return processRunning(false)
}

func ManagerRunning() (bool, error) { return processRunning(true) }
func processRunning(manager bool) (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		match := name == "dcs.exe"
		if manager {
			match = (name == "dcsmanager.exe" || name == "dcsmanager-panels.exe") && entry.ProcessID != uint32(os.Getpid())
		}
		if match {
			return true, nil
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return false, nil
	}
	return false, err
}
