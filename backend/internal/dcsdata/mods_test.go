package dcsdata

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstalledModsSynthetic builds a Mods tree and checks the mods and their
// metadata (size, file count, entry.lua presence) are reported.
func TestInstalledModsSynthetic(t *testing.T) {
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

	// A well-formed mod: entry.lua plus a couple of files.
	a4 := mk("Mods", "aircraft", "A-4E-C")
	write(filepath.Join(a4, "entry.lua"), "-- mod entry")
	write(filepath.Join(a4, "readme.txt"), "hello")
	// A broken mod: no entry.lua.
	broken := mk("Mods", "aircraft", "Broken")
	write(filepath.Join(broken, "junk.lua"), "x")
	// A tech mod.
	tac := mk("Mods", "tech", "Tacview")
	write(filepath.Join(tac, "entry.lua"), "--")

	mods, err := InstalledMods(sg)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 3 {
		t.Fatalf("mods = %d, want 3: %+v", len(mods), mods)
	}
	// Sorted by category then name: A-4E-C, Broken (aircraft), Tacview (tech).
	if mods[0].Name != "A-4E-C" || mods[0].Category != "aircraft" {
		t.Errorf("first mod = %+v, want aircraft/A-4E-C", mods[0])
	}
	if !mods[0].HasEntryLua {
		t.Error("A-4E-C should have entry.lua")
	}
	if mods[0].Files != 2 {
		t.Errorf("A-4E-C files = %d, want 2", mods[0].Files)
	}
	if mods[0].SizeBytes == 0 {
		t.Error("A-4E-C size should not be zero")
	}
	// The broken one is flagged.
	var broke InstalledMod
	for _, m := range mods {
		if m.Name == "Broken" {
			broke = m
		}
	}
	if broke.HasEntryLua {
		t.Error("Broken should not have entry.lua")
	}
	if mods[2].Name != "Tacview" || mods[2].Category != "tech" {
		t.Errorf("last mod = %+v, want tech/Tacview", mods[2])
	}
}

// TestInstalledModsMissingFolder checks a machine with no Mods folder yields an
// empty list, not an error.
func TestInstalledModsMissingFolder(t *testing.T) {
	mods, err := InstalledMods(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mods) != 0 {
		t.Fatalf("expected no mod, got %d", len(mods))
	}
}
