package dcsdata

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMissionsOnRealFiles lists the machine's own .miz when present.
func TestLoadMissionsOnRealFiles(t *testing.T) {
	sg := filepath.Join(os.Getenv("USERPROFILE"), "Saved Games", "DCS")
	if _, err := os.Stat(MissionsDir(sg)); err != nil {
		t.Skip("no DCS missions folder on this machine")
	}
	missions, err := LoadMissions(sg)
	if err != nil {
		t.Fatalf("LoadMissions: %v", err)
	}
	if len(missions) == 0 {
		t.Skip("no .miz on this machine")
	}
	for _, m := range missions {
		if m.Name == "" || m.Path == "" {
			t.Errorf("mission without a name/path: %+v", m)
		}
		t.Logf("%-24s theatre=%-12s date=%-12s %s %.0f°C weather=%q",
			m.Name, m.Theatre, m.Date, formatClock(m.StartTime), m.TemperatureC, m.Weather)
	}
}

func formatClock(sec int) string {
	if sec <= 0 {
		return "--:--"
	}
	return string([]byte{byte('0' + sec/36000%10), byte('0' + sec/3600%10), ':', byte('0' + sec/600%6), byte('0' + sec/60%10)})
}

// TestReadMissionFileSynthetic builds a .miz in memory and checks the metadata
// is read, including the date and weather shapes DCS writes.
func TestReadMissionFileSynthetic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Test.miz")

	mission := []byte(`
mission =
{
	["theatre"] = "Caucasus",
	["start_time"] = 45000,
	["version"] = 19,
	["date"] =
	{
		Year = 1988,
		Day = 26,
		Month = 8,
	},
	["weather"] =
	{
		["name"] = "Summer. Clean sky, no wind",
		["season"] = { ["temperature"] = 20 },
	},
}
`)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(mission); err != nil {
		t.Fatal(err)
	}
	// An extra entry, as real archives have: it must be ignored.
	w2, _ := zw.Create("l10n/en/dictionary")
	w2.Write([]byte("dictionary = {}\n"))
	zw.Close()
	f.Close()

	got, ok := readMissionFile(path)
	if !ok {
		t.Fatal("readMissionFile returned false")
	}
	if got.Theatre != "Caucasus" {
		t.Errorf("Theatre = %q", got.Theatre)
	}
	if got.Date != "1988-08-26" {
		t.Errorf("Date = %q, want 1988-08-26", got.Date)
	}
	if got.StartTime != 45000 {
		t.Errorf("StartTime = %d", got.StartTime)
	}
	if got.Weather != "Summer. Clean sky, no wind" {
		t.Errorf("Weather = %q", got.Weather)
	}
	if got.TemperatureC != 20 {
		t.Errorf("TemperatureC = %v", got.TemperatureC)
	}
	if got.Version != 19 {
		t.Errorf("Version = %d", got.Version)
	}
	if got.Name != "Test" {
		t.Errorf("Name = %q", got.Name)
	}
}

// TestLoadMissionsMissingFolder checks a machine without Saved Games\DCS\Missions
// yields an empty list, not an error.
func TestLoadMissionsMissingFolder(t *testing.T) {
	ms, err := LoadMissions(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ms) != 0 {
		t.Fatalf("expected no mission, got %d", len(ms))
	}
}
