package backup

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InputAircraft is one aircraft folder under Config\Input.
type InputAircraft struct {
	Name    string   `json:"name"`
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`
	Devices []string `json:"devices,omitempty"`
}

// InputScan describes the player's control bindings.
type InputScan struct {
	SavedGames string          `json:"savedGames,omitempty"`
	InputDir   string          `json:"inputDir,omitempty"`
	Aircraft   []InputAircraft `json:"aircraft"`
	Files      int             `json:"files"`
	Bytes      int64           `json:"bytes"`
}

// ScanInput inventories Config\Input without modifying anything.
func ScanInput(savedGames string) (InputScan, error) {
	scan := InputScan{
		SavedGames: savedGames,
		InputDir:   filepath.Join(savedGames, "Config", "Input"),
	}
	if savedGames == "" {
		return scan, nil
	}
	entries, err := os.ReadDir(scan.InputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return scan, nil
		}
		return scan, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			// Root files such as disabled.lua are part of the bindings too.
			if info, err := entry.Info(); err == nil {
				scan.Files++
				scan.Bytes += info.Size()
			}
			continue
		}
		aircraft := InputAircraft{Name: entry.Name()}
		devices := map[string]bool{}
		_ = filepath.WalkDir(filepath.Join(scan.InputDir, entry.Name()), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				aircraft.Files++
				aircraft.Bytes += info.Size()
			}
			rel, err := filepath.Rel(filepath.Join(scan.InputDir, entry.Name()), path)
			if err == nil {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if len(parts) > 1 && parts[0] != "" {
					devices[parts[0]] = true
				}
			}
			return nil
		})
		for device := range devices {
			aircraft.Devices = append(aircraft.Devices, device)
		}
		sort.Strings(aircraft.Devices)
		scan.Aircraft = append(scan.Aircraft, aircraft)
		scan.Files += aircraft.Files
		scan.Bytes += aircraft.Bytes
	}
	sort.SliceStable(scan.Aircraft, func(i, j int) bool {
		return strings.ToLower(scan.Aircraft[i].Name) < strings.ToLower(scan.Aircraft[j].Name)
	})
	return scan, nil
}

// IsInputArchive reports whether an archive contains only control bindings.
func IsInputArchive(categories []string) bool {
	if len(categories) != 1 {
		return false
	}
	return strings.EqualFold(categories[0], "input")
}
