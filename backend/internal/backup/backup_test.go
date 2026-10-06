package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a fake Saved Games tree with two categories' worth of files,
// returning its path and the backup output directory.
func fixture(t *testing.T) (savedGames, outDir string) {
	t.Helper()
	savedGames = filepath.Join(t.TempDir(), "DCS")
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(savedGames, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("MissionEditor/logbook.lua", "logbook")
	write("MissionEditor/modules.lua", "modules")
	write("Config/options.lua", "options")
	write("Config/dcsmanager.cfg", "cfg")
	write("Config/Input/F-16C_50/keyboard.diff.lua", "binds-kb")
	write("Config/Input/F-16C_50/joystick.diff.lua", "binds-js")
	write("Scripts/Export.lua", "export")
	write("Scripts/Hooks/dcsmanager.lua", "hook")
	// A category left out by default, present so we can prove exclusion.
	write("Kneeboard/plate.png", "png")
	// A noisy sibling that no category includes.
	write("Logs/dcs.log", "noise")
	return savedGames, filepath.Join(t.TempDir(), "backups")
}

func TestCreateAndList(t *testing.T) {
	sg, out := fixture(t)

	a, err := Create(sg, out, []string{"logbook", "config", "input", "scripts"}, "test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Entries != 8 {
		t.Errorf("entries = %d, want 8", a.Entries)
	}
	if a.Bytes == 0 {
		t.Error("no bytes recorded")
	}

	list, err := List(out)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list[0].Name != a.Name || len(list[0].Categories) != 4 {
		t.Errorf("listed archive mismatch: %+v", list[0])
	}
}

// TestArchiveContents checks the zip layout: manifest at root, files under
// files/<category>/, and excluded categories absent.
func TestArchiveContents(t *testing.T) {
	sg, out := fixture(t)
	a, err := Create(sg, out, nil, "test") // defaults: logbook, input, config, scripts
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	want := []string{
		"manifest.json",
		"files/logbook/MissionEditor/logbook.lua",
		"files/input/Config/Input/F-16C_50/joystick.diff.lua",
		"files/config/Config/options.lua",
		"files/scripts/Scripts/Hooks/dcsmanager.lua",
	}
	for _, w := range want {
		if !names[w] {
			t.Errorf("missing %q in archive", w)
		}
	}
	// Kneeboard and Logs must not be present (not selected / not a category).
	for _, n := range []string{"Kneeboard/plate.png", "Logs/dcs.log"} {
		for existing := range names {
			if filepath.ToSlash(existing) == "files/kneeboard/"+n {
				t.Errorf("kneeboard should not be included: %q", existing)
			}
		}
	}
}

// TestRestoreRoundTrip destroys the source tree, restores, and checks the files
// come back with their content.
func TestRestoreRoundTrip(t *testing.T) {
	sg, out := fixture(t)
	a, err := Create(sg, out, nil, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Wipe the profile the archive covers.
	for _, rel := range []string{"MissionEditor", "Config/Input", "Config/options.lua", "Scripts"} {
		if err := os.RemoveAll(filepath.Join(sg, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}

	res, err := Restore(a.Path, sg, false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if res.Files != a.Entries {
		t.Errorf("restored %d files, want %d", res.Files, a.Entries)
	}
	if res.SafetyBackup == "" {
		t.Error("a safety backup should have been written")
	}

	for rel, want := range map[string]string{
		"MissionEditor/logbook.lua":               "logbook",
		"Config/options.lua":                      "options",
		"Config/Input/F-16C_50/joystick.diff.lua": "binds-js",
		"Scripts/Hooks/dcsmanager.lua":            "hook",
	} {
		got, err := os.ReadFile(filepath.Join(sg, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (err %v), want %q", rel, got, err, want)
		}
	}
}

// TestRestoreRefusesWhenSafetyBackupFails locks the rule that a restore must not
// proceed if the pre-restore safety copy cannot be written: replacing live DCS
// files with no way back is worse than refusing. The safety archive is written
// next to the input archive as "<name>.prerestore.zip"; a directory there makes the
// write fail.
func TestRestoreRefusesWhenSafetyBackupFails(t *testing.T) {
	sg, out := fixture(t)
	a, err := Create(sg, out, nil, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Occupy the safety archive's path with a directory so createTo cannot write it.
	safety := strings.TrimSuffix(a.Path, ".zip") + ".prerestore.zip"
	if err := os.MkdirAll(safety, 0o755); err != nil {
		t.Fatal(err)
	}

	// Change a live file, so a wrong restore would be visible.
	live := filepath.Join(sg, "Config", "options.lua")
	if err := os.WriteFile(live, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Restore(a.Path, sg, false); err == nil {
		t.Fatal("a restore whose safety backup fails must be refused")
	}
	got, _ := os.ReadFile(live)
	if string(got) != "changed" {
		t.Fatalf("the restore overwrote a live file despite the failed safety backup: %q", got)
	}
}

func TestRestoreDryRun(t *testing.T) {
	sg, out := fixture(t)
	a, err := Create(sg, out, nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(sg, "MissionEditor", "logbook.lua")); err != nil {
		t.Fatal(err)
	}

	res, err := Restore(a.Path, sg, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Files == 0 {
		t.Fatalf("dry run = %+v", res)
	}
	// Nothing should have been written.
	if _, err := os.Stat(filepath.Join(sg, "MissionEditor", "logbook.lua")); !os.IsNotExist(err) {
		t.Error("dry run must not write files")
	}
}

func TestResolveUnknownCategory(t *testing.T) {
	if _, err := Resolve([]string{"logbook", "nope"}); err == nil {
		t.Fatal("unknown category should be refused")
	}
	if _, err := Resolve(nil); err != nil {
		t.Fatalf("defaults should resolve: %v", err)
	}
}

// TestBackupSkipsSymlinks locks the fix for a file leak: os.Open follows a
// symlink, so a link planted under a backed-up folder would pull an arbitrary
// readable file into the archive.
func TestBackupSkipsSymlinks(t *testing.T) {
	if testing.Short() {
		t.Skip("symlinks")
	}
	sg, out := fixture(t)

	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(sg, "Scripts", "leak.lua")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	a, err := Create(sg, out, []string{"scripts"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.Contains(f.Name, "leak") {
			t.Fatalf("symlink was archived: %s", f.Name)
		}
	}
}

// TestRestoreRefusesSymlinkEntry locks the fix that a symlink zip entry cannot
// be extracted (which would plant a link a later restore follows outside Saved
// Games).
func TestRestoreRefusesSymlinkEntry(t *testing.T) {
	dir := t.TempDir()
	sg := filepath.Join(dir, "DCS")
	if err := os.MkdirAll(sg, 0o755); err != nil {
		t.Fatal(err)
	}
	arc := filepath.Join(dir, "link.zip")
	f, err := os.Create(arc)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "files/scripts/Scripts/evil.lua"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(hdr)
	_, _ = w.Write([]byte("/etc/passwd"))
	_ = zw.Close()
	_ = f.Close()

	if _, err := Restore(arc, sg, false); err == nil {
		t.Fatal("a symlink entry must be refused, not extracted")
	}
}

// TestRestoreRefusesTraversal checks a hostile archive cannot write outside
// Saved Games.
func TestRestoreRefusesTraversal(t *testing.T) {
	dir := t.TempDir()
	sg := filepath.Join(dir, "DCS")
	if err := os.MkdirAll(sg, 0o755); err != nil {
		t.Fatal(err)
	}
	arc := filepath.Join(dir, "evil.zip")
	f, err := os.Create(arc)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("files/logbook/../../../../pwned.txt")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	_ = f.Close()

	if _, err := Restore(arc, sg, false); err == nil {
		t.Fatal("restore should refuse a path-traversal entry")
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned.txt")); !os.IsNotExist(err) {
		t.Error("traversal entry escaped Saved Games")
	}
}
