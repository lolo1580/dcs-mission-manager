package dcsdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInspectScripts checks the whole picture: our managed files, the tools
// merged into Export.lua, and other tools' hooks — including the backups
// installers leave behind.
func TestInspectScripts(t *testing.T) {
	sg := t.TempDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{sg}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Export.lua shared with Tacview and DCS-BIOS.
	mk("Scripts")
	write(filepath.Join(sg, "Scripts", "Export.lua"), `
dofile(lfs.writedir() .. [[Scripts\DCS-BIOS\BIOS.lua]])
dofile(lfs.writedir()..'Scripts/TacviewGameExport.lua')
`)
	// A third-party hook and a backup in Hooks/.
	mk("Scripts", "Hooks")
	write(filepath.Join(sg, "Scripts", "Hooks", "TacviewGameGUI.lua"), "--")
	write(filepath.Join(sg, "Scripts", "Hooks", "lottafGameGUI.lua"), "--")
	write(filepath.Join(sg, "Scripts", "Hooks", "bhHook.lua.bak-20260101-000000"), "--")

	managed := []ScriptState{{DestRel: "Scripts/Hooks/dcsmanager.lua", State: "missing"}}
	st := InspectScripts(sg, managed)

	if st.SavedGames != sg {
		t.Errorf("SavedGames = %q", st.SavedGames)
	}
	if len(st.Managed) != 1 || st.Managed[0].State != "missing" {
		t.Errorf("managed = %+v", st.Managed)
	}
	// Export.lua users.
	if len(st.Exports) != 2 {
		t.Errorf("Exports = %v, want Tacview and DCS-BIOS", st.Exports)
	}
	// Third-party hooks, with their tool guessed from the name.
	tools := map[string]string{}
	backups := map[string]bool{}
	for _, h := range st.ThirdParty {
		tools[h.Name] = h.Tool
		backups[h.Name] = h.Backup
	}
	if tools["TacviewGameGUI.lua"] != "Tacview" {
		t.Errorf("TacviewGameGUI tool = %q", tools["TacviewGameGUI.lua"])
	}
	if tools["lottafGameGUI.lua"] != "LotAtc" {
		t.Errorf("lottafGameGUI tool = %q", tools["lottafGameGUI.lua"])
	}
	// A backup is still listed, and flagged as one.
	if !backups["bhHook.lua.bak-20260101-000000"] {
		t.Errorf("the backup should be flagged: %+v", st.ThirdParty)
	}
	// The manager's own files are never reported as another tool's hook.
	for name := range tools {
		if strings.HasPrefix(strings.ToLower(name), "dcsmanager") ||
			strings.HasPrefix(strings.ToLower(name), "export.lua") {
			t.Errorf("%q should not be listed as third-party: it is ours", name)
		}
	}
}

// TestInspectScriptsExcludesOurFiles checks the manager's own files are never
// reported as third-party.
func TestInspectScriptsExcludesOurFiles(t *testing.T) {
	sg := t.TempDir()
	hooks := filepath.Join(sg, "Scripts", "Hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dcsmanager.lua", "dcsmanager.lua.bak-20260101-000000"} {
		if err := os.WriteFile(filepath.Join(hooks, name), []byte("--"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := InspectScripts(sg, nil)
	if len(st.ThirdParty) != 0 {
		t.Fatalf("our files should be excluded, got %+v", st.ThirdParty)
	}
}

// TestInspectScriptsEmptySavedGames checks an empty path yields an empty picture
// rather than a panic or an error.
func TestInspectScriptsEmptySavedGames(t *testing.T) {
	st := InspectScripts("", nil)
	if st.SavedGames != "" || len(st.Managed) != 0 || len(st.ThirdParty) != 0 {
		t.Fatalf("unexpected result for empty path: %+v", st)
	}
}
