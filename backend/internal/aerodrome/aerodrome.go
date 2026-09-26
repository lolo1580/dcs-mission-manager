// Package aerodrome provides reference data for DCS airfields: coordinates,
// radio frequencies (Tower, TACAN, ILS) and the associated approach charts.
//
// The data is embedded at build time from data/*.json. It comes from the
// official aerodrome approach charts (VAD/GND); DCS does not expose radio
// frequencies through its Lua API at runtime, so a curated dataset is the only
// reliable source. See data/README.md for provenance.
package aerodrome

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed data/*.json
var dataFS embed.FS

// ILS is an instrument landing system frequency for a runway.
type ILS struct {
	Runway string  `json:"runway"`
	MHz    float64 `json:"mhz"`
}

// NDB is a non-directional beacon (outer/inner marker or airport homer).
type NDB struct {
	Name string  `json:"name,omitempty"`
	KHz  float64 `json:"khz,omitempty"`
}

// Airfield data sources.
const (
	// SourceDCS means the entry was read from the simulator's own files
	// (Mods/terrains/<map>/radio.lua and beacons.lua). It is authoritative and
	// carries the navigation aids and frequencies exactly as DCS uses them.
	SourceDCS = "dcs"
	// SourceEmbedded means the entry comes from the dataset shipped in the
	// binary. It is the fallback when DCS cannot be located.
	SourceEmbedded = "embedded"
)

// Aerodrome is one airfield.
type Aerodrome struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Theatre    string  `json:"theatre"`
	Coalition  string  `json:"coalition"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	ElevationM float64 `json:"elevationM"`
	Runway     string  `json:"runway"`
	Tower      float64 `json:"tower,omitempty"`
	TACAN      string  `json:"tacan,omitempty"`
	ILS        []ILS   `json:"ils,omitempty"`
	// ICAOCode is the four-letter code DCS stores alongside the name, when the
	// terrain file provides one (it often holds a local designator instead).
	ICAOCode string `json:"icaoCode,omitempty"`
	// VOR and RSBN are radio navigation aids read from beacons.lua.
	VOR    string  `json:"vor,omitempty"`
	VORMHz float64 `json:"vorMhz,omitempty"`
	RSBN   string  `json:"rsbn,omitempty"`
	// PRMG is the Russian equivalent of an ILS.
	PRMG []ILS `json:"prmg,omitempty"`
	// NDB lists non-directional beacons (outer/inner markers).
	NDB []NDB `json:"ndb,omitempty"`
	// Source is "dcs" or "embedded": where this entry came from.
	Source string `json:"source,omitempty"`
	// Charts are file names under maps_dcs/ (not shipped with the binary).
	Charts []string `json:"charts,omitempty"`

	// DistanceKm is filled in by the API when a reference position is given.
	DistanceKm float64 `json:"distanceKm,omitempty"`
}

// Catalog is the whole airfield dataset.
type Catalog struct {
	byID      map[string]Aerodrome
	byTheatre map[string][]Aerodrome
	towns     map[string][]Town
}

// Load reads the embedded dataset.
func Load() (*Catalog, error) {
	c := &Catalog{
		byID:      make(map[string]Aerodrome),
		byTheatre: make(map[string][]Aerodrome),
		towns:     make(map[string][]Town),
	}

	entries, err := dataFS.ReadDir("data")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := dataFS.ReadFile("data/" + e.Name())
		if err != nil {
			return nil, err
		}
		var list []Aerodrome
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("aerodrome: %s: %w", e.Name(), err)
		}
		for _, a := range list {
			if a.ID == "" {
				continue
			}
			a.Source = SourceEmbedded
			c.byID[a.ID] = a
			c.byTheatre[a.Theatre] = append(c.byTheatre[a.Theatre], a)
		}
	}

	c.sortAll()
	return c, nil
}

// LoadWithDCS builds the catalogue from the embedded dataset and then overlays
// whatever DCS's own terrain files provide, for the terrains that are readable.
//
// The DCS data wins, because it is the simulator's own truth: it carries every
// navigation aid (TACAN, ILS, VOR, RSBN) and the real positions, whereas the
// embedded dataset is a transcription that covers a single map. The embedded
// entry is kept when DCS has nothing for that theatre, so the feature still
// works without a DCS installation.
//
// terrainsDir is the Mods\terrains folder; an empty string disables the overlay.
func LoadWithDCS(terrainsDir string) (*Catalog, []TerrainReport, error) {
	c, err := Load()
	if err != nil {
		return nil, nil, err
	}
	if terrainsDir == "" {
		return c, nil, nil
	}

	entries, err := os.ReadDir(terrainsDir)
	if err != nil {
		// A missing or unreadable terrains folder is not an error: the embedded
		// dataset is served instead.
		return c, nil, nil
	}

	var reports []TerrainReport
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		theatre := e.Name()
		dir := filepath.Join(terrainsDir, theatre)

		terrain, err := LoadTerrain(dir, theatre)
		if err != nil {
			reports = append(reports, TerrainReport{Theatre: theatre, Error: err.Error()})
			continue
		}
		reports = append(reports, TerrainReport{
			Theatre:        theatre,
			Airfields:      len(terrain.Airfields),
			WithPosition:   terrain.BeaconAirfields,
			RadioAirfields: terrain.RadioAirfields,
			BeaconTotal:    terrain.BeaconTotal,
			Towns:          len(terrain.Towns),
		})

		if len(terrain.Airfields) == 0 {
			continue
		}

		// Keep whatever the embedded dataset knows and DCS does not: the ICAO
		// code, the runway designator, the coalition. DCS is authoritative for
		// frequencies and aids, but it never states who owns the field.
		enrichFromEmbedded(terrain.Airfields, c.byTheatre[theatre])

		// Replace the whole theatre: mixing a complete DCS list with a partial
		// embedded one would duplicate airfields and hide the better data.
		c.byTheatre[theatre] = terrain.Airfields
		if terrain.Towns != nil {
			c.towns[theatre] = terrain.Towns
		}
		// Rebuild the id index for this theatre.
		for id, a := range c.byID {
			if a.Theatre == theatre {
				delete(c.byID, id)
			}
		}
		for _, a := range terrain.Airfields {
			c.byID[a.ID] = a
		}
	}

	c.sortAll()
	return c, reports, nil
}

// enrichFromEmbedded copies the fields DCS does not expose (name, ICAO code,
// runway, coalition, charts) onto the freshly extracted airfields.
//
// Matching is done by proximity, not by name: DCS stores radio callsigns
// ("Kolkhi", "Lochini") where the curated dataset has full airfield names
// ("Senaki Kolkhi", "Tbilisi Lochini"), so names do not line up. Coordinates do:
// the beacons sit within a couple of kilometres of the field they serve.
func enrichFromEmbedded(extracted, embedded []Aerodrome) {
	if len(embedded) == 0 {
		return
	}
	// maxMatchKm accommodates the distance between an airfield centre and the
	// navigational aid that reports its position (a localizer sits at the
	// threshold, an outer marker several kilometres out).
	const maxMatchKm = 6.0

	for i := range extracted {
		e, ok := nearestEmbedded(extracted[i], embedded, maxMatchKm)
		if !ok {
			continue
		}
		// Curated name and identifier win: they are the ones the charts and the
		// UI already refer to, while DCS exposes only an ATC callsign.
		if e.Name != "" {
			extracted[i].Name = e.Name
		}
		if e.ID != "" {
			extracted[i].ID = e.ID
		}
		if extracted[i].ICAOCode == "" || len(extracted[i].ICAOCode) != 4 {
			extracted[i].ICAOCode = e.ID
		}
		if extracted[i].Runway == "" {
			extracted[i].Runway = e.Runway
		}
		if extracted[i].Coalition == "" {
			extracted[i].Coalition = e.Coalition
		}
		if extracted[i].ElevationM == 0 {
			extracted[i].ElevationM = e.ElevationM
		}
		if extracted[i].Charts == nil {
			extracted[i].Charts = e.Charts
		}

		// DCS knows the ILS frequency but not which runway it serves; the
		// embedded dataset does, so label them now that both are known.
		for j := range extracted[i].ILS {
			if extracted[i].ILS[j].Runway == "" {
				extracted[i].ILS[j].Runway = extracted[i].Runway
			}
		}
		for j := range extracted[i].PRMG {
			if extracted[i].PRMG[j].Runway == "" {
				extracted[i].PRMG[j].Runway = extracted[i].Runway
			}
		}
	}
}

// nearestEmbedded returns the embedded airfield closest to a, within maxKm.
func nearestEmbedded(a Aerodrome, embedded []Aerodrome, maxKm float64) (Aerodrome, bool) {
	var best Aerodrome
	bestKm := maxKm
	found := false
	for _, e := range embedded {
		if e.Lat == 0 && e.Lng == 0 {
			continue
		}
		d := haversineApproxKm(a.Lat, a.Lng, e.Lat, e.Lng)
		if d < bestKm {
			best, bestKm, found = e, d, true
		}
	}
	return best, found
}

// haversineApproxKm is the equirectangular approximation, accurate enough for
// comparing airfields a few kilometres apart.
func haversineApproxKm(lat1, lng1, lat2, lng2 float64) float64 {
	const kmPerDeg = 111.32
	dLat := (lat2 - lat1) * kmPerDeg
	dLng := (lng2 - lng1) * kmPerDeg * cosDeg((lat1+lat2)/2)
	return math.Sqrt(dLat*dLat + dLng*dLng)
}

// cosDeg is a small local cosine, to avoid pulling in a radians helper.
func cosDeg(deg float64) float64 {
	return math.Cos(deg * math.Pi / 180)
}

// TerrainReport summarises what was read from one terrain folder, for logging.
type TerrainReport struct {
	Theatre        string `json:"theatre"`
	Airfields      int    `json:"airfields"`
	WithPosition   int    `json:"withPosition"`
	RadioAirfields int    `json:"radioAirfields"`
	BeaconTotal    int    `json:"beaconTotal"`
	Towns          int    `json:"towns"`
	// Dropped lists the navigation aids rejected for an out-of-band frequency,
	// so a data problem in DCS remains visible.
	Dropped []string `json:"dropped,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// Towns returns the settlements of a theatre, read from towns.lua. Empty when
// the theatre was not read from DCS.
func (c *Catalog) Towns(theatre string) []Town {
	out := make([]Town, len(c.towns[theatre]))
	copy(out, c.towns[theatre])
	return out
}

// HasTowns reports whether a theatre has town data.
func (c *Catalog) HasTowns(theatre string) bool { return len(c.towns[theatre]) > 0 }

func (c *Catalog) sortAll() {
	for th := range c.byTheatre {
		sort.Slice(c.byTheatre[th], func(i, j int) bool {
			return c.byTheatre[th][i].Name < c.byTheatre[th][j].Name
		})
	}
}

// ByID returns one airfield.
func (c *Catalog) ByID(id string) (Aerodrome, bool) {
	a, ok := c.byID[id]
	return a, ok
}

// ByTheatre returns the airfields of a theatre, sorted by name.
func (c *Catalog) ByTheatre(theatre string) []Aerodrome {
	out := make([]Aerodrome, len(c.byTheatre[theatre]))
	copy(out, c.byTheatre[theatre])
	return out
}

// Theatres returns the theatre identifiers present in the dataset.
func (c *Catalog) Theatres() []string {
	out := make([]string, 0, len(c.byTheatre))
	for th := range c.byTheatre {
		out = append(out, th)
	}
	sort.Strings(out)
	return out
}

// Count returns the number of airfields.
func (c *Catalog) Count() int { return len(c.byID) }
