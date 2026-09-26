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
	// Charts are file names under maps_dcs/ (not shipped with the binary).
	Charts []string `json:"charts,omitempty"`

	// DistanceKm is filled in by the API when a reference position is given.
	DistanceKm float64 `json:"distanceKm,omitempty"`
}

// Catalog is the whole airfield dataset.
type Catalog struct {
	byID      map[string]Aerodrome
	byTheatre map[string][]Aerodrome
}

// Load reads the embedded dataset.
func Load() (*Catalog, error) {
	c := &Catalog{
		byID:      make(map[string]Aerodrome),
		byTheatre: make(map[string][]Aerodrome),
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
			c.byID[a.ID] = a
			c.byTheatre[a.Theatre] = append(c.byTheatre[a.Theatre], a)
		}
	}

	for th := range c.byTheatre {
		sort.Slice(c.byTheatre[th], func(i, j int) bool {
			return c.byTheatre[th][i].Name < c.byTheatre[th][j].Name
		})
	}
	return c, nil
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
