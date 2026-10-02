// Package dcsdata reads DCS World's own files from a Saved Games folder that the
// manager runs next to: the installed module inventory, the player's logbook and
// the mission library. It only reads; it never writes to the simulator's files.
//
// Everything here degrades gracefully: a missing or unreadable file yields an
// empty result and a reason, never an error that would take the manager down.
package dcsdata

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dcsmanager/internal/lua"
)

// Module is one entry of DCS's own module inventory (MissionEditor/modules.lua):
// a terrain, an aircraft, a campaign, a tech pack or a bundle.
type Module struct {
	// Category is the inventory section it came from: terrains, campaigns,
	// moduls or bundles.
	Category string `json:"category"`
	// Type is DCS's own label: "Terrain", "Jet engine plane", "Campaign"…
	Type string `json:"type,omitempty"`
	// ID is DCS's internal id (modulId, update_id or code), the stable key.
	ID string `json:"id,omitempty"`
	// Title is the human name, TitleMM falling back to Title.
	Title string `json:"title,omitempty"`
	// Developer is the author (Eagle Dynamics, Heatblur…).
	Developer string `json:"developer,omitempty"`
	// Owned is true when DCS reports have="1".
	Owned bool `json:"owned"`
	// Versions lists the installed versions, newest last.
	Versions []string `json:"versions,omitempty"`
	// Description is the store blurb, trimmed.
	Description string `json:"description,omitempty"`
	// Image is the store thumbnail URL.
	Image string `json:"image,omitempty"`
}

// ModuleInventory is the parsed modules.lua.
type ModuleInventory struct {
	Modules []Module `json:"modules"`
	// Owned counts the modules flagged have="1".
	Owned int `json:"owned"`
	// Total counts every entry.
	Total int `json:"total"`
}

// ModuleInventoryPath returns MissionEditor/modules.lua inside a Saved Games
// folder.
func ModuleInventoryPath(savedGames string) string {
	return filepath.Join(savedGames, "MissionEditor", "modules.lua")
}

// LoadModules parses DCS's module inventory. A missing file is not an error: it
// returns an empty inventory, so the UI can say "nothing found" rather than fail.
func LoadModules(savedGames string) (ModuleInventory, error) {
	path := ModuleInventoryPath(savedGames)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ModuleInventory{}, nil
		}
		return ModuleInventory{}, err
	}
	root, err := lua.Parse(data)
	if err != nil {
		return ModuleInventory{}, err
	}
	return parseModuleInventory(root), nil
}

// parseModuleInventory walks the DLC table DCS writes.
//
// Its shape is:
//
//	DLC = { ["terrains"] = { [1] = { ["title"] = "...", ... }, ... }, ["moduls"] = …, … }
//
// The Lua parser returns the sections either as slices (explicit 1..n keys) or as
// maps, so both are handled.
func parseModuleInventory(root map[string]any) ModuleInventory {
	dlc, _ := root["DLC"].(map[string]any)
	if dlc == nil {
		return ModuleInventory{}
	}

	var out ModuleInventory
	for _, section := range []string{"terrains", "moduls", "campaigns", "bundles"} {
		for _, entry := range asList(dlc[section]) {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			mod := Module{
				Category:    section,
				Type:        str(m["type"]),
				ID:          firstNonEmpty(str(m["modulId"]), str(m["update_id"]), str(m["code"])),
				Title:       firstNonEmpty(str(m["title_mm"]), str(m["title"])),
				Developer:   str(m["developerName"]),
				Owned:       str(m["have"]) == "1",
				Description: strings.TrimSpace(str(m["description"])),
				Image:       str(m["image"]),
			}
			mod.Versions = stringList(m["versions"])
			if mod.Title == "" && mod.ID == "" {
				continue
			}
			if mod.Owned {
				out.Owned++
			}
			out.Modules = append(out.Modules, mod)
		}
	}

	// A stable order: owned first, then by category, then by title. The UI shows
	// the list as-is, so the order belongs here rather than in every caller.
	sort.SliceStable(out.Modules, func(i, j int) bool {
		a, b := out.Modules[i], out.Modules[j]
		if a.Owned != b.Owned {
			return a.Owned
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	})
	out.Total = len(out.Modules)
	return out
}

// asList normalises a Lua table that may be a slice or a numeric-keyed map.
func asList(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		out := make([]any, 0, len(t))
		for _, k := range sortedNumericKeys(t) {
			out = append(out, t[k])
		}
		return out
	default:
		return nil
	}
}

// sortedNumericKeys returns the numeric keys of a map in increasing order, then
// any remaining keys sorted, so iteration is deterministic.
func sortedNumericKeys(m map[string]any) []string {
	var numeric, other []string
	for k := range m {
		if _, err := strconv.Atoi(k); err == nil {
			numeric = append(numeric, k)
		} else {
			other = append(other, k)
		}
	}
	sort.Slice(numeric, func(i, j int) bool {
		a, _ := strconv.Atoi(numeric[i])
		b, _ := strconv.Atoi(numeric[j])
		return a < b
	})
	sort.Strings(other)
	return append(numeric, other...)
}

// stringList flattens a Lua list of strings.
func stringList(v any) []string {
	list := asList(v)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// str returns a string value, or "" for anything else.
func str(v any) string {
	s, _ := v.(string)
	return s
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
