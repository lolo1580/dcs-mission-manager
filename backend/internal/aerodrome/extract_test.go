package aerodrome

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTerrain builds a minimal but faithful terrain folder: the shapes below
// are copied from real DCS files (Caucasus radio.lua / beacons.lua / towns.lua),
// including the gettext wrapper and the enum constants, so the extractor is
// tested against the real grammar.
func writeTerrain(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "map"), 0o755); err != nil {
		t.Fatal(err)
	}

	radio := `
dofile('Scripts/World/Radio/ModulationTypes.lua')
local gettext = require("i_18n")
local _ = gettext.translate

radioTableFormat = 3
radio = {
	{
		radioId = 'airfield22_0';
		role = {"ground", "tower", "approach"};
		callsign = {{["nato"] = {_("Batumi"), "Batumi"}}, {["ussr"] = {_("Druzhinnik"), "Druzhinnik"}}};
		frequency = {[VHF_HI] = {MODULATIONTYPE_AM, 131000000.000000}};
		sceneObjects = {'t:1'};
	};
	{
		radioId = 'airfield12_0';
		callsign = {{["common"] = {_("Anapa"), "Anapa"}}};
		frequency = {[VHF_HI] = {MODULATIONTYPE_AM, 121000000.000000}};
	};
}
`
	beacons := `
dofile('Scripts/Database/wsTypes.lua')

beaconsTableFormat = 2
beacons = {
	{
		display_name = _('Batumi');
		beaconId = 'airfield22_0';
		type = BEACON_TYPE_ILS_LOCALIZER;
		callsign = 'ILU';
		frequency = 110300000.000000;
		position = { -356584.812500, 10.030140, 618472.437500 };
		positionGeo = { latitude = 41.601731, longitude = 41.612203 };
	};
	{
		display_name = _('Batumi');
		beaconId = 'airfield22_2';
		type = BEACON_TYPE_TACAN;
		callsign = 'BTM';
		frequency = 977000000.000000;
		channel = 16;
		positionGeo = { latitude = 41.601, longitude = 41.612 };
	};
	{
		display_name = _('Batumi');
		beaconId = 'airfield22_3';
		type = BEACON_TYPE_AIRPORT_HOMER;
		callsign = 'LU';
		frequency = 430000.000000;
		positionGeo = { latitude = 41.602594, longitude = 41.613238 };
	};
	{
		display_name = _('Anapa-Vityazevo');
		beaconId = 'airfield12_0';
		type = BEACON_TYPE_ILS_FAR_HOMER;
		callsign = 'AP';
		frequency = 443000.000000;
		positionGeo = { latitude = 45.039907, longitude = 37.396435 };
	};
	{
		display_name = _('Kish');
		beaconId = 'world_0';
		type = BEACON_TYPE_VOR_DME;
		callsign = 'KIS';
		channel = 121;
		frequency = 117400000.000000;
		positionGeo = { latitude = 26.5, longitude = 53.9 };
	};
}
`
	towns := `
towns = {
["BATUMI"] = { latitude = 41.654059, longitude = 41.655372, display_name = _("BATUMI")},
["KUTAISI"] = { latitude = 42.267086, longitude = 42.696849, display_name = _("KUTAISI")},
}
`
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("radio.lua", radio)
	write("beacons.lua", beacons)
	write(filepath.Join("map", "towns.lua"), towns)
}

// TestLoadTerrain merges radio.lua and beacons.lua on their shared airfield id
// and checks the result matches what DCS's F10 view shows for Batumi.
func TestLoadTerrain(t *testing.T) {
	dir := t.TempDir()
	writeTerrain(t, dir)

	terrain, err := LoadTerrain(dir, "Caucasus")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(terrain.Airfields) != 2 {
		t.Fatalf("want 2 airfields, got %d: %+v", len(terrain.Airfields), terrain.Airfields)
	}
	if terrain.RadioAirfields != 2 {
		t.Errorf("radio airfields = %d", terrain.RadioAirfields)
	}
	// The world_0 VOR is not tied to an airfield and must be ignored: only the
	// four beacons carrying an "airfield" id are counted.
	if terrain.BeaconTotal != 4 {
		t.Errorf("beacon total = %d, want 4", terrain.BeaconTotal)
	}

	byName := map[string]Aerodrome{}
	for _, a := range terrain.Airfields {
		byName[a.Name] = a
	}

	batumi, ok := byName["Batumi"]
	if !ok {
		t.Fatalf("Batumi missing, got %+v", byName)
	}
	if batumi.Tower != 131 {
		t.Errorf("Batumi tower = %v (want 131 MHz)", batumi.Tower)
	}
	if batumi.TACAN != "16X BTM" {
		t.Errorf("Batumi TACAN = %q (want 16X BTM)", batumi.TACAN)
	}
	if len(batumi.ILS) != 1 || batumi.ILS[0].MHz != 110.3 {
		t.Errorf("Batumi ILS = %+v (want 110.3 MHz)", batumi.ILS)
	}
	if len(batumi.NDB) != 1 || batumi.NDB[0].KHz != 430 {
		t.Errorf("Batumi NDB = %+v (want 430 kHz)", batumi.NDB)
	}
	if batumi.Lat != 41.601731 || batumi.Lng != 41.612203 {
		t.Errorf("Batumi position = %v,%v (want the localizer's)", batumi.Lat, batumi.Lng)
	}
	if batumi.ElevationM == 0 {
		t.Error("Batumi elevation should come from position[2]")
	}
	if batumi.Source != SourceDCS {
		t.Errorf("source = %q, want %q", batumi.Source, SourceDCS)
	}

	// A field with only a radio entry still appears, with its name and frequency.
	anapa, ok := byName["Anapa"]
	if !ok {
		t.Fatalf("Anapa missing, got %+v", byName)
	}
	if anapa.Tower != 121 {
		t.Errorf("Anapa tower = %v", anapa.Tower)
	}
	if anapa.Lat == 0 && anapa.Lng == 0 {
		t.Error("Anapa should have a position from its far homer")
	}

	// Towns are read separately.
	if len(terrain.Towns) != 2 {
		t.Fatalf("want 2 towns, got %d", len(terrain.Towns))
	}
	if terrain.Towns[0].Name != "BATUMI" {
		t.Errorf("towns should be sorted by name, got %q first", terrain.Towns[0].Name)
	}
}

// TestAirfieldNumber covers the id parsing that joins the two files.
func TestAirfieldNumber(t *testing.T) {
	cases := map[string]int{
		"airfield12_0":  12,
		"airfield1_0":   1,
		"airfield220_3": 220,
		"world_0":       -1,
		"airfield":      -1,
		"":              -1,
	}
	for id, want := range cases {
		if got := airfieldNumber(id); got != want {
			t.Errorf("airfieldNumber(%q) = %d, want %d", id, got, want)
		}
	}
}

// TestRoundMHz checks the Hz -> MHz conversion, which must match how DCS
// displays frequencies (three decimals).
func TestRoundMHz(t *testing.T) {
	cases := map[float64]float64{
		121000000: 121,
		110300000: 110.3,
		124350000: 124.35,
		131000000: 131,
		0:         0,
	}
	for hz, want := range cases {
		if got := roundMHz(hz); got != want {
			t.Errorf("roundMHz(%v) = %v, want %v", hz, got, want)
		}
	}
}

// TestFormatTACAN checks the DCS display format, including the X/Y band split
// (channels above 126 belong to the Y band).
func TestFormatTACAN(t *testing.T) {
	cases := []struct {
		callsign string
		channel  int
		want     string
	}{
		{"BTM", 16, "16X BTM"},
		{"KIS", 121, "121X KIS"},
		{"ABC", 130, "130Y ABC"},
		{"XYZ", 0, "XYZ"},
		{"", 16, ""},
	}
	for _, c := range cases {
		if got := formatTACAN(c.callsign, c.channel); got != c.want {
			t.Errorf("formatTACAN(%q, %d) = %q, want %q", c.callsign, c.channel, got, c.want)
		}
	}
}

// TestEmbeddedFallback checks that the catalogue still works without DCS, and
// that every embedded entry is tagged as such.
func TestEmbeddedFallback(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("load embedded: %v", err)
	}
	if c.Count() == 0 {
		t.Fatal("the embedded dataset should not be empty")
	}
	for _, a := range c.ByTheatre("Caucasus") {
		if a.Source != SourceEmbedded {
			t.Errorf("%s: source = %q, want %q", a.ID, a.Source, SourceEmbedded)
		}
	}

	// An empty terrains directory must not fail, and must not change the data.
	c2, reports, err := LoadWithDCS("")
	if err != nil {
		t.Fatalf("LoadWithDCS(\"\"): %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("no reports expected, got %d", len(reports))
	}
	if c2.Count() != c.Count() {
		t.Errorf("count changed: %d -> %d", c.Count(), c2.Count())
	}

	// A non-existent directory is equally harmless.
	c3, _, err := LoadWithDCS(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("LoadWithDCS(missing): %v", err)
	}
	if c3.Count() != c.Count() {
		t.Errorf("count changed: %d -> %d", c.Count(), c3.Count())
	}
}

// TestLoadWithDCSOverlaysEmbedded checks the merge: DCS data replaces the
// theatre, while the embedded-only fields (ICAO, runway, coalition) are carried
// over.
func TestLoadWithDCSOverlaysEmbedded(t *testing.T) {
	root := t.TempDir()
	writeTerrain(t, filepath.Join(root, "Caucasus"))

	c, reports, err := LoadWithDCS(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(reports) != 1 || reports[0].Theatre != "Caucasus" {
		t.Fatalf("unexpected reports: %+v", reports)
	}
	if reports[0].Airfields != 2 {
		t.Errorf("report airfields = %d", reports[0].Airfields)
	}

	list := c.ByTheatre("Caucasus")
	if len(list) != 2 {
		t.Fatalf("want 2 airfields for the overlaid theatre, got %d", len(list))
	}
	// The embedded dataset has Batumi/Anapa; enrichment must have filled the
	// runway and coalition that DCS does not provide.
	var batumi *Aerodrome
	for i := range list {
		if list[i].Name == "Batumi" {
			batumi = &list[i]
		}
	}
	if batumi == nil {
		t.Fatal("Batumi missing after the overlay")
	}
	if batumi.Source != SourceDCS {
		t.Errorf("source = %q, want %q", batumi.Source, SourceDCS)
	}
	if batumi.Runway == "" {
		t.Error("the runway should be carried over from the embedded dataset")
	}
	if batumi.Coalition == "" {
		t.Error("the coalition should be carried over from the embedded dataset")
	}
	if len(batumi.ILS) > 0 && batumi.ILS[0].Runway == "" {
		t.Error("ILS entries should inherit the runway label")
	}

	// Lookup by id must find the overlaid entry.
	if _, ok := c.ByID(batumi.ID); !ok {
		t.Errorf("ByID(%q) should find the overlaid airfield", batumi.ID)
	}
}
