package dcsdata

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanLiveriesLayouts(t *testing.T) {
	sg := t.TempDir()
	writeTestFile(t, filepath.Join(sg, "Liveries", "F-5E-3", "J-3080", "description.lua"), `
name = "Swiss J-3080"
countries = {"Switzerland"}
livery = {
  {"F-5E3_bort_number_nose_100", 0, "../J-3XXX PS/PS_nose", false},
  {"F-5E3_nose", 0, "J-3080_nose", false},
}
`)
	writeTestFile(t, filepath.Join(sg, "Liveries", "F-5E-3", "J-3080", "J-3080_nose.dds"), "x")
	writeTestFile(t, filepath.Join(sg, "Liveries", "F-5E-3", "J-3XXX PS", "description.lua"), `name = "Shared"`)
	writeTestFile(t, filepath.Join(sg, "Liveries", "F-5E-3", "J-3XXX PS", "PS_nose.dds"), "x")

	writeTestFile(t, filepath.Join(sg, "Liveries", "ADA 1-2 Cigognes", "description.lua"), `
name = "ADA 1-2 Cigognes"
livery = {
  {"ext1", 0, "M2KC_1T", false},
}
`)
	writeTestFile(t, filepath.Join(sg, "Liveries", "ADA 1-2 Cigognes", "M2KC_1T.dds"), "x")

	writeTestFile(t, filepath.Join(sg, "Liveries", "Swiss Mirage III", "CoreMods", "aircraft", "M-2000C", "Liveries", "M-2000C", "Fictional Swiss J-5011", "description.lua"), `
name = "Fictional Swiss J-5011"
livery = {
  {"ext1", 0, "M2KC_1T", false},
}
`)
	writeTestFile(t, filepath.Join(sg, "Liveries", "Swiss Mirage III", "CoreMods", "aircraft", "M-2000C", "Liveries", "M-2000C", "Fictional Swiss J-5011", "M2KC_1T.dds"), "x")

	writeTestFile(t, filepath.Join(sg, "Mods", "aircraft", "A-4E-C", "entry.lua"), `declare_plugin("A-4E-C")`)
	writeTestFile(t, filepath.Join(sg, "Mods", "aircraft", "A-4E-C", "Liveries", "A-4E-C", "Default", "description.lua"), `name = "Default"`)

	scan, err := ScanLiveries(sg)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Liveries) != 5 {
		t.Fatalf("liveries = %d, want 5: %+v", len(scan.Liveries), scan.Liveries)
	}
	byName := map[string]Livery{}
	for _, livery := range scan.Liveries {
		byName[livery.Name] = livery
	}

	j3080 := byName["Swiss J-3080"]
	if j3080.Aircraft != "F-5E-3" || j3080.Structure != "standard" {
		t.Errorf("J-3080 = aircraft %q structure %q", j3080.Aircraft, j3080.Structure)
	}
	if len(j3080.Dependencies) != 1 || j3080.Dependencies[0] != "F-5E-3/J-3XXX PS" {
		t.Errorf("J-3080 dependencies = %v", j3080.Dependencies)
	}

	ada := byName["ADA 1-2 Cigognes"]
	if ada.Aircraft != "M-2000C" || ada.Structure != "flat" {
		t.Errorf("ADA = aircraft %q structure %q", ada.Aircraft, ada.Structure)
	}
	if len(ada.Countries) != 0 {
		t.Errorf("ADA countries = %v", ada.Countries)
	}

	misplaced := byName["Fictional Swiss J-5011"]
	if misplaced.Aircraft != "M-2000C" || misplaced.Structure != "misplaced" {
		t.Errorf("misplaced = aircraft %q structure %q", misplaced.Aircraft, misplaced.Structure)
	}

	mod := byName["Default"]
	if mod.Source != "mod" || mod.Aircraft != "A-4E-C" || !mod.ModEnabled {
		t.Errorf("mod livery = %+v", mod)
	}
}

func TestScanLiveriesMissingDependency(t *testing.T) {
	sg := t.TempDir()
	writeTestFile(t, filepath.Join(sg, "Liveries", "Broken", "description.lua"), `
name = "Broken"
livery = {
  {"ext", 0, "../Missing/tex", false},
}
`)
	scan, err := ScanLiveries(sg)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Liveries) != 1 {
		t.Fatalf("liveries = %d", len(scan.Liveries))
	}
	has := false
	for _, warning := range scan.Liveries[0].Warnings {
		if warning.Code == "missing_dependency" {
			has = true
		}
	}
	if !has {
		t.Errorf("warnings = %+v, want missing_dependency", scan.Liveries[0].Warnings)
	}
	if scan.Liveries[0].ID == "" {
		t.Fatal("empty ID")
	}
}
