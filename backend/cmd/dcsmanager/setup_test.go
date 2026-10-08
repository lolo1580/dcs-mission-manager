package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestDetectionPreservesUnicodeForWindowsINI(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("DCSMANAGER_SAVED_GAMES", `C:\Joueurs\André\DCS`)
	t.Setenv("DCSMANAGER_DCS_INSTALL", `D:\Jeux\DCS été`)
	output := filepath.Join(t.TempDir(), "detection.ini")
	if runSetupDetect([]string{"--output", output}) != 0 {
		t.Fatal("detection failed")
	}
	decoded := detectionText(t, output)
	if !strings.Contains(decoded, `SavedGames=C:\Joueurs\André\DCS`) || !strings.Contains(decoded, `Game=D:\Jeux\DCS été`) {
		t.Fatalf("paths corrupted: %q", decoded)
	}
}

func TestDetectionUsesSavedSettingsAndEnvironmentPriority(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("DCSMANAGER_SAVED_GAMES", "")
	t.Setenv("DCSMANAGER_DCS_INSTALL", "")
	root := filepath.Join(base, "DCS Manager")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"savedGames":"saved-sg","dcsInstall":"saved-game"}`), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "detection.ini")
	if runSetupDetect([]string{"--output", output}) != 0 {
		t.Fatal("detection failed")
	}
	if got := detectionText(t, output); !strings.Contains(got, "SavedGames=saved-sg\r\nGame=saved-game") {
		t.Fatalf("saved settings not detected: %q", got)
	}
	t.Setenv("DCSMANAGER_SAVED_GAMES", "env-sg")
	t.Setenv("DCSMANAGER_DCS_INSTALL", "env-game")
	if runSetupDetect([]string{"--output", output}) != 0 {
		t.Fatal("detection with environment overrides failed")
	}
	if got := detectionText(t, output); !strings.Contains(got, "SavedGames=env-sg\r\nGame=env-game") {
		t.Fatalf("environment overrides lost: %q", got)
	}
}

func detectionText(t *testing.T, output string) string {
	t.Helper()
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xfe {
		t.Fatal("Windows Unicode BOM missing")
	}
	units := make([]uint16, (len(data)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	return string(utf16.Decode(units))
}
