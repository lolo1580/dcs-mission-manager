package charts

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLibrary builds a small chart library mirroring the real folder layout:
// one folder per theatre, with the naming conventions the scans actually use.
func writeLibrary(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		// Caucasus: approach + ground plan for one field, named with its code.
		"DCS Caucasus Maps/01_VAD_UG5X_Kobuleti.png":    "x",
		"DCS Caucasus Maps/01_GND_UG5X_Kobuleti_18.png": "x",
		"DCS Caucasus Maps/07_VAD_UGSB_Batumi.png":      "x",
		"DCS Caucasus Maps/00_08_Aerodrome_Legend.png":  "x",
		// Kola: instrument charts, named with country and RWY.
		"DCS Kola Maps/NORWAY_LAKSELV-ILS-RWY34.png":      "x",
		"DCS Kola Maps/FINLAND_ROVANIEMI-ILS-Y-RWY21.jpg": "x",
		// A nested folder, to prove subfolders are indexed.
		"Marianas_High_Detail_Maps/WWII/beacons.png": "x",
	}
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A non-image file must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "DCS Caucasus Maps", "Radio.lua"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeLibrary(t, dir)

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Count() != 7 {
		t.Fatalf("want 7 charts (the .lua must be ignored), got %d", c.Count())
	}

	// Theatres are inferred from the folder names.
	theatres := c.Theatres()
	want := []string{"Caucasus", "Kola", "MarianasWWII"}
	if len(theatres) != len(want) {
		t.Fatalf("theatres = %v, want %v", theatres, want)
	}
	for i := range want {
		if theatres[i] != want[i] {
			t.Errorf("theatres = %v, want %v", theatres, want)
		}
	}

	// A chart in a subfolder is indexed with its relative path.
	ch, ok := c.Get("Marianas_High_Detail_Maps/WWII/beacons.png")
	if !ok {
		t.Fatal("a chart in a subfolder should be indexed")
	}
	if ch.Theatre != "MarianasWWII" {
		t.Errorf("subfolder theatre = %q, want MarianasWWII", ch.Theatre)
	}
	// A bare name must NOT resolve: the path is the identifier.
	if _, ok := c.Get("beacons.png"); ok {
		t.Error("a bare name should not resolve; paths are the identifiers")
	}
}

// TestKindAndRunway covers the naming conventions the scans use.
func TestKindAndRunway(t *testing.T) {
	dir := t.TempDir()
	writeLibrary(t, dir)
	c, _ := Load(dir)

	cases := []struct{ path, kind, runway string }{
		{"DCS Caucasus Maps/01_VAD_UG5X_Kobuleti.png", "approach", ""},
		{"DCS Caucasus Maps/01_GND_UG5X_Kobuleti_18.png", "ground", "18"},
		{"DCS Caucasus Maps/00_08_Aerodrome_Legend.png", "general", ""},
		{"DCS Kola Maps/NORWAY_LAKSELV-ILS-RWY34.png", "instrument", "34"},
		{"DCS Kola Maps/FINLAND_ROVANIEMI-ILS-Y-RWY21.jpg", "instrument", "21"},
	}
	for _, tc := range cases {
		ch, ok := c.Get(tc.path)
		if !ok {
			t.Errorf("%s not indexed", tc.path)
			continue
		}
		if ch.Kind != tc.kind {
			t.Errorf("%s: kind = %q, want %q", tc.path, ch.Kind, tc.kind)
		}
		if ch.Runway != tc.runway {
			t.Errorf("%s: runway = %q, want %q", tc.path, ch.Runway, tc.runway)
		}
	}
}

// TestForAerodrome covers matching a chart to an airfield by ICAO code or name.
func TestForAerodrome(t *testing.T) {
	dir := t.TempDir()
	writeLibrary(t, dir)
	c, _ := Load(dir)

	// By ICAO code.
	got := c.ForAerodrome("UG5X", "Kobuleti")
	if len(got) != 2 {
		t.Fatalf("Kobuleti: want 2 charts, got %d (%+v)", len(got), got)
	}
	// Approach sorts before ground.
	if got[0].Kind != "approach" {
		t.Errorf("first chart should be the approach, got %q", got[0].Kind)
	}

	// By name only, with no ICAO code.
	if n := len(c.ForAerodrome("", "Lakselv")); n != 1 {
		t.Errorf("Lakselv by name: want 1 chart, got %d", n)
	}

	// A name shortened in the file name still matches ("Rovaniemi" appears in
	// full here, but the longest-word fallback must not break it).
	if n := len(c.ForAerodrome("", "Rovaniemi")); n != 1 {
		t.Errorf("Rovaniemi: want 1 chart, got %d", n)
	}

	// An unknown airfield yields nothing rather than an error.
	if n := len(c.ForAerodrome("ZZZZ", "Nowhere")); n != 0 {
		t.Errorf("unknown airfield: want 0 charts, got %d", n)
	}
}

// TestMissingDirectoryIsHarmless checks the feature is optional.
func TestMissingDirectoryIsHarmless(t *testing.T) {
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "nope")} {
		c, err := Load(dir)
		if err != nil {
			t.Fatalf("Load(%q): %v", dir, err)
		}
		if c.Count() != 0 {
			t.Errorf("Load(%q): want an empty catalogue, got %d", dir, c.Count())
		}
		if _, ok := c.Get("anything.png"); ok {
			t.Errorf("Load(%q): nothing should resolve", dir)
		}
	}
}

// TestNormaliseAndLongestWord covers the matching helpers.
func TestNormaliseAndLongestWord(t *testing.T) {
	if got := normalise("Anapa-Vityazevo"); got != "anapavityazevo" {
		t.Errorf("normalise = %q", got)
	}
	if got := longestWord("Maykop Khanskaya"); got != "Khanskaya" {
		t.Errorf("longestWord = %q, want Khanskaya", got)
	}
	if got := normalise(" UG5X "); got != "ug5x" {
		t.Errorf("normalise = %q", got)
	}
}
