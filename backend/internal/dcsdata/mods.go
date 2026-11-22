package dcsdata

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// InstalledMod is one mod folder found under Saved Games\DCS\Mods or its
// disabled sibling. It is a folder-based mod: DCS identifies it by its folder
// name, and a real mod ships an entry.lua.
type InstalledMod struct {
	// Category is the Mods subfolder: "aircraft", "tech"…
	Category string `json:"category"`
	// Name is the folder name, which is how DCS identifies the mod.
	Name string `json:"name"`
	// Path is the absolute folder path.
	Path string `json:"path"`
	// SizeBytes is the total size of the folder.
	SizeBytes int64 `json:"sizeBytes"`
	// Files counts the files it contains.
	Files int `json:"files"`
	// HasEntryLua is true when the mod ships an entry.lua, which every real DCS
	// mod needs. Its absence usually means a half-installed or broken mod.
	HasEntryLua bool `json:"hasEntryLua"`
	// Enabled is false when the folder lives in Mods.disabled.
	Enabled bool `json:"enabled"`
	// Version, Author and Description are best-effort values read from entry.lua.
	Version     string `json:"version,omitempty"`
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	// Warnings carries machine-readable codes the UI translates.
	Warnings []string `json:"warnings,omitempty"`
}

// ModsScan is the complete mod picture for the content manager.
type ModsScan struct {
	Mods        []InstalledMod `json:"mods"`
	Active      int            `json:"active"`
	Disabled    int            `json:"disabled"`
	TotalBytes  int64          `json:"totalBytes"`
	SavedGames  string         `json:"savedGames,omitempty"`
	ModsDir     string         `json:"modsDir,omitempty"`
	DisabledDir string         `json:"disabledDir,omitempty"`
}

// ModsDir returns the Saved Games\Mods folder, where enabled mods live.
func ModsDir(savedGames string) string {
	return filepath.Join(savedGames, "Mods")
}

// DisabledModsDir returns the sibling folder that holds disabled mods. It is
// deliberately outside Mods so DCS never scans it.
func DisabledModsDir(savedGames string) string {
	return filepath.Join(savedGames, "Mods.disabled")
}

// ModsQuarantineDir returns the sibling folder that holds removed mods until
// the user restores or cleans them up by hand.
func ModsQuarantineDir(savedGames string) string {
	return filepath.Join(savedGames, "Mods.quarantine")
}

// InstalledMods lists enabled mods under an installation's Saved Games Mods
// folder. It is kept for the historical /api/mods endpoint; the content
// manager uses ScanMods, which also reports disabled mods.
func InstalledMods(savedGames string) ([]InstalledMod, error) {
	scan, err := ScanMods(savedGames)
	if err != nil {
		return nil, err
	}
	enabled := make([]InstalledMod, 0, len(scan.Mods))
	for _, mod := range scan.Mods {
		if mod.Enabled {
			enabled = append(enabled, mod)
		}
	}
	return enabled, nil
}

// ScanMods lists enabled and disabled mods. A missing folder yields an empty
// list, never an error.
func ScanMods(savedGames string) (ModsScan, error) {
	scan := ModsScan{
		SavedGames:  savedGames,
		ModsDir:     ModsDir(savedGames),
		DisabledDir: DisabledModsDir(savedGames),
	}
	if savedGames == "" {
		return scan, nil
	}
	enabled, err := scanModRoot(scan.ModsDir, true)
	if err != nil {
		return scan, err
	}
	disabled, err := scanModRoot(scan.DisabledDir, false)
	if err != nil {
		return scan, err
	}
	scan.Mods = append(enabled, disabled...)
	sort.SliceStable(scan.Mods, func(i, j int) bool {
		if scan.Mods[i].Category != scan.Mods[j].Category {
			return scan.Mods[i].Category < scan.Mods[j].Category
		}
		return strings.ToLower(scan.Mods[i].Name) < strings.ToLower(scan.Mods[j].Name)
	})
	for _, mod := range scan.Mods {
		scan.TotalBytes += mod.SizeBytes
		if mod.Enabled {
			scan.Active++
		} else {
			scan.Disabled++
		}
	}
	return scan, nil
}

// scanModRoot walks one Mods-style root: <category>/<mod>.
func scanModRoot(root string, enabled bool) ([]InstalledMod, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []InstalledMod
	for _, cat := range entries {
		if !cat.IsDir() {
			continue
		}
		catDir := filepath.Join(root, cat.Name())
		mods, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}
		for _, mod := range mods {
			if !mod.IsDir() {
				continue
			}
			full := filepath.Join(catDir, mod.Name())
			im := InstalledMod{
				Category: cat.Name(),
				Name:     mod.Name(),
				Path:     full,
				Enabled:  enabled,
			}
			im.SizeBytes, im.Files = folderSize(full)
			im.HasEntryLua = fileExists(filepath.Join(full, "entry.lua"))
			im.readMetadata()
			if !im.HasEntryLua {
				im.Warnings = append(im.Warnings, "missing_entry_lua")
			}
			if im.Files == 0 {
				im.Warnings = append(im.Warnings, "empty_mod")
			}
			out = append(out, im)
		}
	}
	return out, nil
}

var (
	modVersionRE = regexp.MustCompile(`(?m)\bversion\s*=\s*["']([^"']{1,80})["']`)
	modAuthorRE  = regexp.MustCompile(`(?mi)\b(?:developer|author)\s*=\s*["']([^"']{1,120})["']`)
	modInfoRE    = regexp.MustCompile(`(?m)\b(?:info|description)\s*=\s*["']([^"']{1,240})["']`)
)

// readMetadata extracts the few fields mods commonly declare in entry.lua. It
// is deliberately best-effort: a mod with a different entry.lua still lists.
func (m *InstalledMod) readMetadata() {
	path := filepath.Join(m.Path, "entry.lua")
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := io.ReadAll(io.LimitReader(f, 512*1024))
	text := string(data)
	if v := firstMatch(modVersionRE, text); v != "" {
		m.Version = v
	}
	if v := firstMatch(modAuthorRE, text); v != "" {
		m.Author = v
	}
	if v := firstMatch(modInfoRE, text); v != "" {
		m.Description = v
	}
}

func firstMatch(re *regexp.Regexp, text string) string {
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// folderSize walks a folder, summing file sizes and counting files. It never
// fails: an unreadable entry is simply not counted.
func folderSize(dir string) (bytes int64, files int) {
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			bytes += info.Size()
			files++
		}
		return nil
	})
	return bytes, files
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
