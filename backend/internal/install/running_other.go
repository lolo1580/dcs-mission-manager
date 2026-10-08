//go:build !windows

package install

func DCSRunning() (bool, error)     { return false, nil }
func ManagerRunning() (bool, error) { return false, nil }
