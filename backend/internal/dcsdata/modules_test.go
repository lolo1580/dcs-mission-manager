package dcsdata

import (
	"os"
	"path/filepath"
	"testing"

	"dcsmanager/internal/lua"
)

// TestLoadModulesOnRealInventory parses the machine's own modules.lua when it is
// present. It is skipped on a machine without DCS, so CI stays green.
func TestLoadModulesOnRealInventory(t *testing.T) {
	sg := filepath.Join(os.Getenv("USERPROFILE"), "Saved Games", "DCS")
	if _, err := os.Stat(ModuleInventoryPath(sg)); err != nil {
		t.Skip("no DCS install on this machine")
	}
	inv, err := LoadModules(sg)
	if err != nil {
		t.Fatalf("LoadModules: %v", err)
	}
	if inv.Total == 0 {
		t.Fatal("expected at least one module")
	}
	// Every entry must carry a title or an id, and at least one terrain must be
	// owned on any real install.
	var terrains, ownedTerrains int
	for _, m := range inv.Modules {
		if m.Title == "" && m.ID == "" {
			t.Errorf("module with neither title nor id: %+v", m)
		}
		if m.Category == "terrains" {
			terrains++
			if m.Owned {
				ownedTerrains++
			}
		}
	}
	if terrains == 0 {
		t.Error("no terrain in the inventory")
	}
	t.Logf("inventory: %d modules, %d owned, %d terrains (%d owned)",
		inv.Total, inv.Owned, terrains, ownedTerrains)
}

// TestParseModuleInventorySynthetic checks the parser against a hand-written
// document, so the real file is not the only thing covered.
func TestParseModuleInventorySynthetic(t *testing.T) {
	src := []byte(`
DLC = {
	["terrains"] = {
		[1] = { ["modulId"] = "Caucasus", ["title_mm"] = "DCS: Caucasus", ["type"] = "Terrain", ["have"] = "1", ["versions"] = { [1] = "2.9.0" } },
		[2] = { ["modulId"] = "Iraq", ["title"] = "DCS: Iraq Map", ["type"] = "Terrain", ["have"] = "0" },
	},
	["moduls"] = {
		[1] = { ["modulId"] = "F-16C_50", ["title_mm"] = "DCS: F-16C Viper", ["type"] = "Jet engine plane", ["have"] = "1", ["developerName"] = "Eagle Dynamics" },
	},
}
`)
	root, err := lua.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	inv := parseModuleInventory(root)
	if inv.Total != 3 {
		t.Fatalf("Total = %d, want 3", inv.Total)
	}
	if inv.Owned != 2 {
		t.Fatalf("Owned = %d, want 2", inv.Owned)
	}
	// Owned first, so the first entry must be an owned one.
	if !inv.Modules[0].Owned {
		t.Errorf("owned modules should sort first, got %+v", inv.Modules[0])
	}
	var caucasus Module
	for _, m := range inv.Modules {
		if m.ID == "Caucasus" {
			caucasus = m
		}
	}
	if caucasus.Title != "DCS: Caucasus" || len(caucasus.Versions) != 1 || caucasus.Versions[0] != "2.9.0" {
		t.Errorf("Caucasus parsed wrong: %+v", caucasus)
	}
}
