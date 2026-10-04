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
	inv, err := LoadModules(sg, "")
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

// TestInstalledIsMatchedFromAutoupdate checks the distinction the UI needs: a
// module can be owned (have="1", bought) without being installed on disk. This
// machine's Kola and Persian Gulf are exactly that case.
func TestInstalledIsMatchedFromAutoupdate(t *testing.T) {
	inv := ModuleInventory{Modules: []Module{
		{Category: "terrains", ID: "Caucasus", Owned: true, matchKeys: []string{"Caucasus", "CAUCASUS_terrain"}},
		{Category: "terrains", ID: "Kola", Owned: true, matchKeys: []string{"KOLA_terrain"}},
		{Category: "moduls", ID: "F-16C", Owned: true, matchKeys: []string{"F-16C"}},
		{Category: "moduls", ID: "F-14", Owned: true, matchKeys: []string{"HEATBLUR_F-14"}},
		{Category: "campaigns", ID: "c1", Owned: true},
	}}
	inv.markInstalled(map[string]bool{
		"CAUCASUS_TERRAIN": true,
		"F-16C":            true,
	})

	byID := map[string]Module{}
	for _, m := range inv.Modules {
		byID[m.ID] = m
	}
	if !byID["Caucasus"].Installed {
		t.Error("Caucasus is in the install list and must be installed")
	}
	if byID["Kola"].Installed {
		t.Error("Kola is owned but absent from the install list, so not installed")
	}
	if !byID["F-16C"].Installed {
		t.Error("F-16C is in the install list and must be installed")
	}
	if byID["F-14"].Installed {
		t.Error("F-14 is owned but absent from the install list, so not installed")
	}
	// A campaign is not installable on its own: its state stays unknown.
	if byID["c1"].InstallKnown {
		t.Error("a campaign must not claim an install state")
	}
	if inv.Installed != 2 {
		t.Errorf("Installed = %d, want 2", inv.Installed)
	}

	// Without autoupdate.cfg nothing is known, and nothing is reported installed.
	unknown := ModuleInventory{Modules: inv.Modules}
	unknown.markInstalled(nil)
	if unknown.Installed != 0 {
		t.Errorf("with no install list, Installed = %d, want 0", unknown.Installed)
	}
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
