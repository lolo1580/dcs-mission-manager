package backup

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// CreateMappings writes a control-mapping archive. When aircraft is empty every
// folder under Config\Input is included (including root files such as
// disabled.lua); otherwise only the named aircraft folders are archived.
func CreateMappings(savedGames, outDir string, aircraft []string, appVersion string) (Archive, error) {
	if savedGames == "" {
		return Archive{}, fmt.Errorf("DCS Saved Games folder not found")
	}
	if info, err := os.Stat(savedGames); err != nil || !info.IsDir() {
		return Archive{}, fmt.Errorf("the Saved Games folder %q is not a directory", savedGames)
	}
	if outDir == "" {
		return Archive{}, fmt.Errorf("no output directory for the backup")
	}
	inputDir := filepath.Join(savedGames, "Config", "Input")
	if info, err := os.Stat(inputDir); err != nil || !info.IsDir() {
		return Archive{}, fmt.Errorf("Config\\Input folder not found")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Archive{}, err
	}

	selected, err := normalizeAircraft(inputDir, aircraft)
	if err != nil {
		return Archive{}, err
	}

	name := uniqueMappingName(outDir)
	path := filepath.Join(outDir, name)
	if err := createMappingsTo(savedGames, inputDir, path, selected, appVersion); err != nil {
		return Archive{}, err
	}
	return readArchive(path)
}

func normalizeAircraft(inputDir string, aircraft []string) ([]string, error) {
	if len(aircraft) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, raw := range aircraft {
		name := strings.TrimSpace(raw)
		if name == "" || name == "." || name == ".." {
			return nil, fmt.Errorf("invalid aircraft folder %q", raw)
		}
		if strings.ContainsAny(name, `/\:`) {
			return nil, fmt.Errorf("invalid aircraft folder %q", raw)
		}
		if info, err := os.Stat(filepath.Join(inputDir, name)); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("aircraft folder %q not found", name)
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out, nil
}

func uniqueMappingName(dir string) string {
	base := "dcsmanager-mappings-" + time.Now().Format("20060102-150405")
	name := base + ".zip"
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d.zip", base, i)
	}
}

func createMappingsTo(savedGames, inputDir, path string, aircraft []string, appVersion string) (err error) {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	zw := zip.NewWriter(f)
	var entries int
	var bytes int64

	add := func(p string) error {
		relToSG, relErr := filepath.Rel(savedGames, p)
		if relErr != nil {
			return relErr
		}
		// The category segment keeps Restore's existing safety-copy logic and
		// archive layout unchanged.
		zipName := filesPrefix + "input/" + filepath.ToSlash(relToSG)
		n, wErr := addFile(zw, p, zipName)
		if wErr != nil {
			return wErr
		}
		if n >= 0 {
			entries++
			bytes += n
		}
		return nil
	}

	roots := aircraft
	if len(roots) == 0 {
		roots = []string{""}
	}
	for _, aircraftName := range roots {
		root := inputDir
		if aircraftName != "" {
			root = filepath.Join(inputDir, aircraftName)
		}
		if wErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			return add(p)
		}); wErr != nil {
			return wErr
		}
	}

	man := Manifest{
		Version:    1,
		CreatedAt:  time.Now().UTC(),
		SavedGames: filepath.Base(savedGames),
		AppVersion: appVersion,
		Categories: []string{"input"},
		Aircraft:   aircraft,
		Entries:    entries,
		Bytes:      bytes,
	}
	if err = writeManifest(zw, man); err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
