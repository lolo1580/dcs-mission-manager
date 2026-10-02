package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dcsmanager/internal/luafiles"
)

const distExport = `--[[
  DCS Manager — Export.lua
]]
` + BeginMarker + `
do
  local x = 1
end
` + EndMarker + "\n"

const distHooks = "-- hooks\n" + BeginMarker + "\ndo end\n" + EndMarker + "\n"

const distConfig = "dcsmanager_host = \"127.0.0.1\"\n"

// setup creates a fake distribution and a fake Saved Games folder.
func setup(t *testing.T) (*Installer, string) {
	t.Helper()
	root := t.TempDir()

	luaDir := filepath.Join(root, "dcs-lua")
	mustWrite(t, filepath.Join(luaDir, "Export.lua"), distExport)
	mustWrite(t, filepath.Join(luaDir, "Hooks", "dcsmanager.lua"), distHooks)
	mustWrite(t, filepath.Join(luaDir, "Config", "dcsmanager.cfg"), distConfig)

	sg := filepath.Join(root, "Saved Games", "DCS")
	if err := os.MkdirAll(sg, 0o755); err != nil {
		t.Fatal(err)
	}

	in := New(luaDir, sg)
	in.Now = func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }
	return in, sg
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFreshCreatesAllFiles(t *testing.T) {
	in, sg := setup(t)

	results, err := in.Install(DefaultTargets())
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	for _, r := range results {
		if r.Action != "created" {
			t.Errorf("%s: want created, got %s", r.DestRel, r.Action)
		}
	}

	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Export.lua")); err != nil {
		t.Error("Export.lua should exist")
	}
	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Hooks", "dcsmanager.lua")); err != nil {
		t.Error("Hooks/dcsmanager.lua should exist")
	}
	if _, err := os.Stat(filepath.Join(sg, "Config", "dcsmanager.cfg")); err != nil {
		t.Error("Config/dcsmanager.cfg should exist")
	}
}

// TestInstallMergesIntoExistingExport is the safety-critical case: an Export.lua
// already used by Tacview/SRS must be preserved, with our block appended.
func TestInstallMergesIntoExistingExport(t *testing.T) {
	in, sg := setup(t)

	existing := `-- Tacview export
function LuaExportStart()
  TacviewInit()
end
`
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), existing)

	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatalf("install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)

	if !strings.Contains(content, "TacviewInit()") {
		t.Fatal("the existing Export.lua content must be preserved")
	}
	if !strings.Contains(content, BeginMarker) || !strings.Contains(content, EndMarker) {
		t.Fatal("the DCSMANAGER block must be present")
	}
	if !strings.Contains(content, "local x = 1") {
		t.Fatal("the block content must be present")
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	in, sg := setup(t)
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), "-- Tacview\n")

	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))

	results, err := in.Install(DefaultTargets())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action != "unchanged" {
			t.Errorf("%s: want unchanged on second run, got %s", r.DestRel, r.Action)
		}
	}

	second, _ := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))
	if string(first) != string(second) {
		t.Fatal("re-installing must not modify the file")
	}
}

func TestInstallUpdatesOutdatedBlockInPlace(t *testing.T) {
	in, sg := setup(t)

	oldBlock := BeginMarker + "\ndo local x = 0 end\n" + EndMarker
	existing := "-- Tacview\n" + oldBlock + "\n-- more of my own code\n"
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), existing)

	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))
	content := string(got)

	if strings.Contains(content, "local x = 0") {
		t.Fatal("the old block should have been replaced")
	}
	if !strings.Contains(content, "local x = 1") {
		t.Fatal("the new block should be present")
	}
	if !strings.Contains(content, "-- more of my own code") {
		t.Fatal("code after the block must be preserved")
	}
}

func TestInstallBacksUpBeforeMerging(t *testing.T) {
	in, sg := setup(t)
	exportPath := filepath.Join(sg, "Scripts", "Export.lua")
	mustWrite(t, exportPath, "-- Tacview\n")

	results, err := in.Install(DefaultTargets())
	if err != nil {
		t.Fatal(err)
	}

	var exportResult *Result
	for i := range results {
		if strings.HasSuffix(results[i].DestRel, "Export.lua") {
			exportResult = &results[i]
		}
	}
	if exportResult == nil || exportResult.Backup == "" {
		t.Fatal("a backup path should be reported for the merged file")
	}
	backup, err := os.ReadFile(exportResult.Backup)
	if err != nil {
		t.Fatalf("backup not readable: %v", err)
	}
	if string(backup) != "-- Tacview\n" {
		t.Fatalf("backup should hold the original content, got %q", backup)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	in, sg := setup(t)
	in.DryRun = true

	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Export.lua")); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create files")
	}
}

func TestUninstallRemovesBlockAndKeepsRest(t *testing.T) {
	in, sg := setup(t)
	existing := "-- Tacview start\n"
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), existing)
	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatal(err)
	}

	results, err := in.Uninstall()
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))
	content := string(got)
	if strings.Contains(content, BeginMarker) {
		t.Fatal("the DCSMANAGER block should be removed")
	}
	if !strings.Contains(content, "Tacview start") {
		t.Fatal("the original content must remain")
	}

	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Hooks", "dcsmanager.lua")); !os.IsNotExist(err) {
		t.Fatal("Hooks/dcsmanager.lua should be removed")
	}

	found := false
	for _, r := range results {
		if r.Action == "block-removed" {
			found = true
		}
	}
	if !found {
		t.Fatal("uninstall should report the block removal")
	}
}

func TestStatusReportsMissingThenInstalledThenOutdated(t *testing.T) {
	in, _ := setup(t)
	targets := DefaultTargets()

	// Missing.
	for _, r := range in.Status(targets) {
		if r.Action != "missing" {
			t.Errorf("expected missing, got %s for %s", r.Action, r.DestRel)
		}
	}

	// Installed.
	if _, err := in.Install(targets); err != nil {
		t.Fatal(err)
	}
	for _, r := range in.Status(targets) {
		if r.Action != "installed" {
			t.Errorf("expected installed, got %s for %s", r.Action, r.DestRel)
		}
	}

	// Outdated (the distribution changes).
	mustWrite(t, filepath.Join(in.LuaDir, "Config", "dcsmanager.cfg"), "dcsmanager_host = \"10.0.0.5\"\n")
	status := in.Status(targets)
	var outdated int
	for _, r := range status {
		if r.Action == "outdated" && strings.HasSuffix(r.DestRel, "dcsmanager.cfg") {
			outdated++
		}
	}
	if outdated != 1 {
		t.Fatal("a changed distribution file should be reported as outdated")
	}
}

func TestSpliceBlockAppendsWhenAbsent(t *testing.T) {
	out, changed := spliceBlock("my content\n", "BLOCK")
	if !changed {
		t.Fatal("expected a change")
	}
	if !strings.HasPrefix(out, "my content\n\n") || !strings.HasSuffix(out, "BLOCK\n") {
		t.Fatalf("unexpected splice result: %q", out)
	}
}

func TestSpliceBlockReplacesInPlace(t *testing.T) {
	existing := "before\nBLOCK-old\nafter\n"
	block := "BLOCK"
	// Only markers trigger replacement, so wrap the block.
	existing = "before\n" + BeginMarker + "\nold\n" + EndMarker + "\nafter\n"
	block = BeginMarker + "\nnew\n" + EndMarker

	out, changed := spliceBlock(existing, block)
	if !changed {
		t.Fatal("expected a change")
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Fatal("surrounding content must be preserved")
	}
	if strings.Contains(out, "old") {
		t.Fatal("the old block must be replaced")
	}
	if !strings.Contains(out, "new") {
		t.Fatal("the new block must be present")
	}
}

func TestExtractBlockMissing(t *testing.T) {
	if extractBlock("no markers here") != "" {
		t.Fatal("expected an empty block when markers are absent")
	}
}

// TestSpliceBlockRepairsDuplicateBlocks covers the real-world file produced by an
// older installer whose marker punctuation no longer matched: the block was
// appended a second time instead of replacing the first. Both defined the same
// Lua globals, and the duplicates had to be collapsed to a single block.
func TestSpliceBlockRepairsDuplicateBlocks(t *testing.T) {
	existing := "before\n" +
		BeginMarker + "\nold-a\n" + EndMarker + "\n" +
		"middle\n" +
		BeginMarker + "\nold-b\n" + EndMarker + "\n" +
		"after\n"
	block := BeginMarker + "\nnew\n" + EndMarker

	out, changed := spliceBlock(existing, block)
	if !changed {
		t.Fatal("expected a change")
	}
	if strings.Contains(out, "old-a") || strings.Contains(out, "old-b") {
		t.Fatal("both old blocks must be removed")
	}
	if strings.Count(out, BeginMarker) != 1 || strings.Count(out, EndMarker) != 1 {
		t.Fatalf("expected exactly one block, got: %q", out)
	}
	for _, keep := range []string{"before", "middle", "after", "new"} {
		if !strings.Contains(out, keep) {
			t.Fatalf("surrounding content %q must be preserved", keep)
		}
	}
	want := "before\n" + block + "\nmiddle\nafter\n"
	if out != want {
		t.Fatalf("unexpected collapse:\n got: %q\nwant: %q", out, want)
	}
}

// TestSpliceBlockToleratesDamagedMarkerPunctuation covers a marker that went
// through an encoding round-trip: the em dash became "â€"". The keyword is still
// recognisable, so the block is repaired rather than duplicated again.
func TestSpliceBlockToleratesDamagedMarkerPunctuation(t *testing.T) {
	damagedBegin := "-- >>> DCSMANAGER-BEGIN (managed block \xc3\xa2\xc2\x80\xc2\x94 do not edit by hand) >>>"
	if damagedBegin == BeginMarker {
		t.Fatal("the damaged marker must differ from the current one")
	}

	existing := "keep-me\n" + damagedBegin + "\nstale\n" + EndMarker + "\n"
	block := BeginMarker + "\nfresh\n" + EndMarker

	out, changed := spliceBlock(existing, block)
	if !changed {
		t.Fatal("expected a change")
	}
	if strings.Contains(out, "stale") {
		t.Fatal("the stale block must be replaced")
	}
	if !strings.Contains(out, "fresh") || !strings.Contains(out, "keep-me") {
		t.Fatalf("unexpected result: %q", out)
	}
	if strings.Count(out, beginKeyword) != 1 {
		t.Fatalf("expected a single managed block, got: %q", out)
	}
}

// TestSpliceBlockIgnoresKeywordInCode guards the safety rule: a mere mention of
// the keyword in real Lua code must never be treated as a block boundary.
func TestSpliceBlockIgnoresKeywordInCode(t *testing.T) {
	existing := "local s = \"DCSMANAGER-BEGIN is just a string\"\nlocal t = \"DCSMANAGER-END\"\n"
	block := BeginMarker + "\nnew\n" + EndMarker

	out, changed := spliceBlock(existing, block)
	if !changed {
		t.Fatal("expected the block to be appended")
	}
	if !strings.Contains(out, "just a string") {
		t.Fatal("the user's code must be preserved")
	}
	if strings.Count(out, BeginMarker) != 1 {
		t.Fatalf("expected exactly one real block, got: %q", out)
	}
}

// TestUninstallRemovesEveryBlock covers the same duplicate-block file: removal
// must not leave a stale block behind to keep defining the Lua globals.
func TestUninstallRemovesEveryBlock(t *testing.T) {
	in, sg := setup(t)

	existing := "Tacview\n" +
		BeginMarker + "\nold-a\n" + EndMarker + "\n" +
		BeginMarker + "\nold-b\n" + EndMarker + "\n"
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), existing)

	if _, err := in.Uninstall(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(sg, "Scripts", "Export.lua"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	if strings.Contains(content, beginKeyword) || strings.Contains(content, endKeyword) {
		t.Fatalf("every block should be gone, got: %q", content)
	}
	if !strings.Contains(content, "Tacview") {
		t.Fatal("the surrounding content must be preserved")
	}
}

// TestEmbeddedFallback covers the released-binary case: no dcs-lua folder next
// to the executable, so the scripts must come from the embedded copies. This
// guards the regression where a lone dcsmanager.exe could not install anything.
func TestEmbeddedFallback(t *testing.T) {
	root := t.TempDir()
	sg := filepath.Join(root, "Saved Games", "DCS")
	if err := os.MkdirAll(sg, 0o755); err != nil {
		t.Fatal(err)
	}

	// LuaDir deliberately empty: embedded copies only.
	in := New("", sg)
	in.Now = func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }

	results, err := in.Install(DefaultTargets())
	if err != nil {
		t.Fatalf("install from embedded scripts: %v", err)
	}
	for _, r := range results {
		if r.Action != "created" {
			t.Errorf("%s: want created, got %s", r.DestRel, r.Action)
		}
	}

	// The installed block must contain the real payload, not a placeholder.
	exportPath := filepath.Join(sg, "Scripts", "Export.lua")
	content, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), BeginMarker) {
		t.Fatal("the installed Export.lua should contain the managed block")
	}
	if !strings.Contains(string(content), "LuaExportActivityNextEvent") {
		t.Fatal("the installed Export.lua should contain the real export code")
	}

	// Status must report them as installed when reading from the embedded set.
	for _, r := range in.Status(DefaultTargets()) {
		if r.Action != "installed" {
			t.Errorf("status: %s should be installed, got %s", r.DestRel, r.Action)
		}
	}

	// Uninstall must work too, without any on-disk distribution.
	if _, err := in.Uninstall(); err != nil {
		t.Fatalf("uninstall from embedded scripts: %v", err)
	}
	remaining, _ := os.ReadFile(exportPath)
	if strings.Contains(string(remaining), BeginMarker) {
		t.Fatal("the managed block should have been removed")
	}
}

// TestEmbeddedPathsAreForwardSlashed ensures the embedded keys use the same
// separator style as DefaultTargets, which is what readSource normalises to.
func TestEmbeddedPathsAreForwardSlashed(t *testing.T) {
	for _, name := range luafiles.List() {
		if strings.Contains(name, "\\") {
			t.Errorf("embedded path %q should use forward slashes", name)
		}
	}
	// Every default target must resolve, either on disk or embedded.
	in := New("", t.TempDir())
	for _, target := range DefaultTargets() {
		if _, err := in.readSource(target.Source); err != nil {
			t.Errorf("default target %q does not resolve: %v", target.Source, err)
		}
	}
}

func TestFindSavedGamesPrefersOpenBeta(t *testing.T) {
	// This test relies on the real user profile, so it only checks that the
	// function returns a path or a helpful error.
	path, err := FindSavedGames()
	if err != nil {
		if !strings.Contains(err.Error(), "Saved Games") {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if path == "" {
		t.Fatal("a detected path should not be empty")
	}
}
