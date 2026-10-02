package dcsdata

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dcsmanager/internal/lua"
)

// Setting is one key/value pair from DCS's options.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConfigSection is one group of options (graphics, difficulty, VR, sound…).
type ConfigSection struct {
	Name     string    `json:"name"`
	Settings []Setting `json:"settings"`
}

// Plugin is one entry of pluginsEnabled.lua: a terrain or module switched off.
type Plugin struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// DCSConfig is the game's own configuration, as stored in Saved Games\DCS\Config.
type DCSConfig struct {
	// Sections are the options.lua groups, in DCS's own order.
	Sections []ConfigSection `json:"sections"`
	// Plugins lists the toggles from pluginsEnabled.lua (disabled terrains/modules).
	Plugins []Plugin `json:"plugins"`
	// Manager lists the manager's own dcsmanager.cfg values, when the file exists.
	Manager []Setting `json:"manager"`
	// Language is the UI language from lang.cfg ("fr", "en"…).
	Language string `json:"language,omitempty"`
	// Path is the Config folder inspected.
	Path string `json:"path"`
}

// ConfigDir returns Saved Games\DCS\Config.
func ConfigDir(savedGames string) string {
	return filepath.Join(savedGames, "Config")
}

// Section order for options.lua: DCS writes them in this order and the UI keeps it
// so the page reads like the game's own settings screen.
var configSectionOrder = []string{"graphics", "difficulty", "VR", "sound", "views", "cockpit", "miscellaneous", "plugins"}

// LoadConfig reads DCS's configuration. Missing files are not an error: the
// corresponding field is simply left empty.
func LoadConfig(savedGames string) (DCSConfig, error) {
	dir := ConfigDir(savedGames)
	cfg := DCSConfig{Path: dir}

	if root, err := loadLuaFile(filepath.Join(dir, "options.lua")); err != nil {
		return cfg, err
	} else if root != nil {
		options, _ := root["options"].(map[string]any)
		if options == nil {
			options = root
		}
		cfg.Sections = orderedSections(options)
	}

	if root, err := loadLuaFile(filepath.Join(dir, "pluginsEnabled.lua")); err == nil && root != nil {
		if plugins, ok := root["pluginsEnabled"].(map[string]any); ok {
			for _, name := range sortedKeys(plugins) {
				cfg.Plugins = append(cfg.Plugins, Plugin{
					Name:    name,
					Enabled: truthy(plugins[name]),
				})
			}
		}
	}

	// The manager's own config: flat `key = value` assignments, not a table.
	if root, err := loadLuaFile(filepath.Join(dir, "dcsmanager.cfg")); err == nil && root != nil {
		for _, key := range sortedKeys(root) {
			cfg.Manager = append(cfg.Manager, Setting{Key: key, Value: formatValue(root[key])})
		}
	}

	if data, err := os.ReadFile(filepath.Join(dir, "lang.cfg")); err == nil {
		cfg.Language = strings.TrimSpace(string(data))
	}
	return cfg, nil
}

// loadLuaFile reads and parses a Lua data file. A missing file yields (nil, nil) so
// callers can treat "absent" and "empty" the same way.
func loadLuaFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return lua.Parse(data)
}

// orderedSections turns the options table into sections, following DCS's own order
// for the ones it knows and appending any unknown section at the end.
func orderedSections(options map[string]any) []ConfigSection {
	var out []ConfigSection
	seen := map[string]bool{}

	add := func(name string, v any) {
		sub, ok := v.(map[string]any)
		if !ok {
			return // a scalar at this level is not a section
		}
		sec := ConfigSection{Name: name}
		for _, key := range sortedKeys(sub) {
			// A nested table inside a section (e.g. views' sub-tables) is shown as
			// its key with a compact rendering, so nothing is silently dropped.
			sec.Settings = append(sec.Settings, Setting{Key: key, Value: formatValue(sub[key])})
		}
		out = append(out, sec)
		seen[name] = true
	}

	for _, name := range configSectionOrder {
		if v, ok := options[name]; ok {
			add(name, v)
		}
	}
	// Any other section DCS added in a future version.
	for _, name := range sortedKeys(options) {
		if !seen[name] {
			add(name, options[name])
		}
	}
	return out
}

// sortedKeys returns a map's keys in a stable order: numeric keys first and in
// numeric order, then the rest alphabetically.
func sortedKeys(m map[string]any) []string {
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

// formatValue renders a Lua value for display. Numbers lose a trailing ".0" and
// booleans become "on"/"off", which reads better than "true"/"false" in a table.
func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "—"
	case bool:
		if t {
			return "on"
		}
		return "off"
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case []any:
		return "(" + strconv.Itoa(len(t)) + ")"
	case map[string]any:
		return "(" + strconv.Itoa(len(t)) + ")"
	default:
		return ""
	}
}
