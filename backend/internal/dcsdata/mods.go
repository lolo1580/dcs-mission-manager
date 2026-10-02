package dcsdata

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstalledMod is one mod folder found under Saved Games\DCS\Mods.
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
}

// ModsDir returns the Saved Games\Mods folder, where installed mods live.
func ModsDir(savedGames string) string {
	return filepath.Join(savedGames, "Mods")
}

// InstalledMods lists the mods under an installation's Saved Games Mods folder.
// A missing folder yields an empty list, never an error.
func InstalledMods(savedGames string) ([]InstalledMod, error) {
	root := filepath.Join(savedGames, "Mods")
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
			im := InstalledMod{Category: cat.Name(), Name: mod.Name(), Path: full}
			im.SizeBytes, im.Files = folderSize(full)
			im.HasEntryLua = fileExists(filepath.Join(full, "entry.lua"))
			out = append(out, im)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
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
