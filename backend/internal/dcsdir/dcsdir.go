// Package dcsdir locates the DCS World installation on this machine.
//
// The manager runs locally, so it can read the simulator's own files: the
// terrain data (airfields, beacons) and the mission editor data. This package is
// the single place that answers "where is DCS installed?".
//
// Detection is best-effort and never fatal: when DCS cannot be found, callers
// fall back to the data embedded in the binary.
package dcsdir

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxLogScan bounds how much of dcs.log is searched. The install path appears in
// the first few lines; there is no reason to read a multi-megabyte log.
const maxLogScan = 256 * 1024

// Find returns the DCS installation directory, or "" when it cannot be
// determined. savedGames is the Saved Games\DCS folder, used for the dcs.log
// fallback; it may be empty.
func Find(savedGames string) string {
	if dir := findFromRegistry(); dir != "" {
		return dir
	}
	return findFromLog(savedGames)
}

// terrainDirs lists the candidate terrain roots inside an installation.
func terrainRoots(dcsDir string) []string {
	return []string{filepath.Join(dcsDir, "Mods", "terrains")}
}

// TerrainsDir returns the Mods\terrains directory of an installation, or "" if
// it does not exist.
func TerrainsDir(dcsDir string) string {
	for _, root := range terrainRoots(dcsDir) {
		if isDir(root) {
			return root
		}
	}
	return ""
}

// findFromLog extracts the install path from the "Command line:" line DCS writes
// at the top of every dcs.log:
//
//	... APP (Main): Command line: "E:\Games\DCS World\bin/DCS.exe"
func findFromLog(savedGames string) string {
	if savedGames == "" {
		return ""
	}
	logPath := filepath.Join(savedGames, "Logs", "dcs.log")
	f, err := os.Open(logPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, maxLogScan)
	n, _ := f.Read(buf)
	text := string(buf[:n])

	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "Command line:") {
			continue
		}
		// Take the quoted executable path.
		start := strings.Index(line, `"`)
		end := strings.LastIndex(line, `"`)
		if start < 0 || end <= start {
			continue
		}
		exe := line[start+1 : end]
		// ...\bin/DCS.exe or ...\bin\DCS.exe -> the install root.
		bin := filepath.Dir(filepath.FromSlash(exe))
		if strings.EqualFold(filepath.Base(bin), "bin") {
			return filepath.Dir(bin)
		}
		return bin
	}
	return ""
}

// findFromRegistry reads the install path from the Windows registry, under
// HKCU\Software\Eagle Dynamics\<product>\Path. It shells out to reg.exe so the
// build stays dependency-free and cross-compilable (this file is Windows-only in
// practice, but it must still compile elsewhere).
func findFromRegistry() string {
	out, err := regQuery()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		// e.g. "    Path    REG_SZ    E:\Games\DCS World"
		fields := regexp.MustCompile(`\s{2,}`).Split(strings.TrimRight(line, "\r"), -1)
		if len(fields) < 3 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(fields[0]), "Path") {
			continue
		}
		// The value is everything after "REG_SZ".
		idx := strings.Index(line, "REG_SZ")
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(line[idx+len("REG_SZ"):])
		if value != "" && isDir(value) {
			return filepath.Clean(value)
		}
	}
	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
