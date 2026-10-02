package dcsdata

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScriptState is one of our managed files, as found on the DCS side.
type ScriptState struct {
	// DestRel is the path relative to Saved Games.
	DestRel string `json:"destRel"`
	// State is "installed", "outdated", "missing" or "unknown".
	State string `json:"state"`
	// Note carries the reason when the state is not a plain installed/missing.
	Note string `json:"note,omitempty"`
}

// ThirdPartyHook is another tool's hook found in Scripts or Scripts/Hooks.
type ThirdPartyHook struct {
	// Name is the file name.
	Name string `json:"name"`
	// Dir is "Scripts" or "Scripts/Hooks".
	Dir string `json:"dir"`
	// Tool guesses which tool it belongs to, from the file name.
	Tool string `json:"tool,omitempty"`
	// SizeBytes is the file size.
	SizeBytes int64 `json:"sizeBytes"`
	// Backup is true for a ".bak-…" file left by an installer.
	Backup bool `json:"backup,omitempty"`
}

// ScriptStatus is the whole picture of what is installed on the DCS side.
type ScriptStatus struct {
	// Managed is our own files and whether they are up to date.
	Managed []ScriptState `json:"managed"`
	// Exports is the Export.lua picture: which tools hook into it.
	Exports []string `json:"exports"`
	// Legacy holds leftovers from the pre-rename install (dcsmm.* / DCSMM-*
	// markers), which the current version does not manage and which the user
	// has to remove.
	Legacy []string `json:"legacy"`
	// ThirdParty lists other tools' hook files.
	ThirdParty []ThirdPartyHook `json:"thirdParty"`
	// SavedGames is the folder inspected.
	SavedGames string `json:"savedGames"`
}

// InspectScripts reports what is installed in a Saved Games folder: our managed
// files (via the installer's own status), the tools merged into Export.lua, the
// legacy pre-rename leftovers, and other tools' hook files.
func InspectScripts(savedGames string, managed []ScriptState) ScriptStatus {
	st := ScriptStatus{SavedGames: savedGames, Managed: managed}
	if savedGames == "" {
		return st
	}
	scripts := filepath.Join(savedGames, "Scripts")

	// Which tools are merged into Export.lua.
	if data, err := os.ReadFile(filepath.Join(scripts, "Export.lua")); err == nil {
		st.Exports = detectExportUsers(string(data))
	}

	// Legacy leftovers from before the rename.
	for _, rel := range []string{
		filepath.Join("Scripts", "Hooks", "dcsmm.lua"),
		filepath.Join("Config", "dcsmm.cfg"),
	} {
		if fileExists(filepath.Join(savedGames, rel)) {
			st.Legacy = append(st.Legacy, filepath.ToSlash(rel))
		}
	}
	// The old marker block, still present in Export.lua.
	if data, err := os.ReadFile(filepath.Join(scripts, "Export.lua")); err == nil {
		if strings.Contains(string(data), "DCSMM-BEGIN") {
			st.Legacy = append(st.Legacy, filepath.ToSlash(filepath.Join("Scripts", "Export.lua"))+" (bloc DCSMM)")
		}
	}

	// Other tools' hooks.
	for _, dir := range []string{"Scripts", "Scripts/Hooks"} {
		full := filepath.Join(savedGames, filepath.FromSlash(dir))
		entries, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".lua") &&
				!strings.Contains(strings.ToLower(name), ".lua.") {
				continue
			}
			if isOurFile(name) {
				continue
			}
			var size int64
			if info, err := e.Info(); err == nil {
				size = info.Size()
			}
			st.ThirdParty = append(st.ThirdParty, ThirdPartyHook{
				Name:      name,
				Dir:       dir,
				Tool:      guessTool(name),
				SizeBytes: size,
				Backup:    strings.Contains(name, ".bak-"),
			})
		}
	}
	sort.SliceStable(st.ThirdParty, func(i, j int) bool {
		if st.ThirdParty[i].Dir != st.ThirdParty[j].Dir {
			return st.ThirdParty[i].Dir < st.ThirdParty[j].Dir
		}
		return strings.ToLower(st.ThirdParty[i].Name) < strings.ToLower(st.ThirdParty[j].Name)
	})
	return st
}

// isOurFile reports whether a hook file is one the manager itself owns — either
// the current name or a leftover from before the rename. Those are reported as
// legacy, not as another tool's hook.
func isOurFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ours := range []string{"dcsmanager.lua", "export.lua", "dcsmm.lua"} {
		if lower == ours || strings.HasPrefix(lower, ours+".bak-") {
			return true
		}
	}
	return lower == "export.lua" || strings.HasPrefix(lower, "export.lua.bak-")
}

// guessTool matches a hook file name to the tool it belongs to, for the ones
// commonly found next to DCS. An unknown name is reported as-is (empty tool).
func guessTool(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "tacview"):
		return "Tacview"
	case strings.Contains(lower, "dcs-bios"), strings.Contains(lower, "bios"):
		return "DCS-BIOS"
	case strings.Contains(lower, "srs"), strings.Contains(lower, "simpleradio"):
		return "SRS"
	case strings.Contains(lower, "lottaf"):
		return "LotAtc"
	case strings.Contains(lower, "bhhook"), strings.Contains(lower, "battlehub"):
		return "BattleHub"
	case strings.Contains(lower, "vaicom"):
		return "VAICOM"
	}
	return ""
}

// detectExportUsers returns the tools referenced by an Export.lua, matched on
// the dofile/require lines other installers add. It is what tells the user their
// Export.lua is shared, and with what.
func detectExportUsers(content string) []string {
	var found []string
	add := func(name string) {
		for _, f := range found {
			if f == name {
				return
			}
		}
		found = append(found, name)
	}
	lower := strings.ToLower(content)
	known := []struct{ needle, tool string }{
		{"tacview", "Tacview"},
		{"dcs-bios", "DCS-BIOS"},
		{"tacviewgameexport", "Tacview"},
		{"simpleradio", "SRS"},
		{"srs", "SRS"},
		{"vaicom", "VAICOM"},
		{"lottaf", "LotAtc"},
		{"bhook", "BattleHub"},
	}
	for _, k := range known {
		if strings.Contains(lower, k.needle) {
			add(k.tool)
		}
	}
	sort.Strings(found)
	return found
}
