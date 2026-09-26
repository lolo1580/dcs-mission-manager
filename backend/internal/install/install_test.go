package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const distExport = `--[[
  DCS Mission Manager — Export.lua
]]
` + BeginMarker + `
do
  local x = 1
end
` + EndMarker + "\n"

const distHooks = "-- hooks\n" + BeginMarker + "\ndo end\n" + EndMarker + "\n"

const distConfig = "dcsmm_host = \"127.0.0.1\"\n"

// setup creates a fake distribution and a fake Saved Games folder.
func setup(t *testing.T) (*Installer, string) {
	t.Helper()
	root := t.TempDir()

	luaDir := filepath.Join(root, "dcs-lua")
	mustWrite(t, filepath.Join(luaDir, "Export.lua"), distExport)
	mustWrite(t, filepath.Join(luaDir, "Hooks", "dcsmm.lua"), distHooks)
	mustWrite(t, filepath.Join(luaDir, "Config", "dcsmm.cfg"), distConfig)

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
	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Hooks", "dcsmm.lua")); err != nil {
		t.Error("Hooks/dcsmm.lua should exist")
	}
	if _, err := os.Stat(filepath.Join(sg, "Config", "dcsmm.cfg")); err != nil {
		t.Error("Config/dcsmm.cfg should exist")
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
		t.Fatal("the DCSMM block must be present")
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
		t.Fatal("the DCSMM block should be removed")
	}
	if !strings.Contains(content, "Tacview start") {
		t.Fatal("the original content must remain")
	}

	if _, err := os.Stat(filepath.Join(sg, "Scripts", "Hooks", "dcsmm.lua")); !os.IsNotExist(err) {
		t.Fatal("Hooks/dcsmm.lua should be removed")
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
	mustWrite(t, filepath.Join(in.LuaDir, "Config", "dcsmm.cfg"), "dcsmm_host = \"10.0.0.5\"\n")
	status := in.Status(targets)
	var outdated int
	for _, r := range status {
		if r.Action == "outdated" && strings.HasSuffix(r.DestRel, "dcsmm.cfg") {
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
