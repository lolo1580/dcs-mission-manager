package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanInput(t *testing.T) {
	sg := t.TempDir()
	put := func(rel, body string) {
		t.Helper()
		path := filepath.Join(sg, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("Config/Input/disabled.lua", "return")
	put("Config/Input/F-16C_50/keyboard/Keyboard.diff.lua", "kb")
	put("Config/Input/F-16C_50/joystick/Stick.diff.lua", "stick")
	put("Config/Input/FA-18C_hornet/keyboard/Keyboard.diff.lua", "kb")

	scan, err := ScanInput(sg)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Files != 4 || scan.Bytes != int64(len("return")+len("kb")+len("stick")+len("kb")) {
		t.Fatalf("scan = %+v", scan)
	}
	if len(scan.Aircraft) != 2 {
		t.Fatalf("aircraft = %+v", scan.Aircraft)
	}
	if scan.Aircraft[0].Name != "F-16C_50" || scan.Aircraft[0].Files != 2 {
		t.Fatalf("first aircraft = %+v", scan.Aircraft[0])
	}
	if len(scan.Aircraft[0].Devices) != 2 {
		t.Fatalf("devices = %v", scan.Aircraft[0].Devices)
	}
}

func TestScanInputMissing(t *testing.T) {
	scan, err := ScanInput(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Aircraft) != 0 || scan.Files != 0 {
		t.Fatalf("scan = %+v", scan)
	}
}

func TestIsInputArchive(t *testing.T) {
	if !IsInputArchive([]string{"input"}) || !IsInputArchive([]string{"INPUT"}) {
		t.Fatal("input archive not recognized")
	}
	if IsInputArchive([]string{"input", "config"}) || IsInputArchive(nil) {
		t.Fatal("non-input archive accepted")
	}
}
