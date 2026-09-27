// Package theatre describes DCS maps: their identifiers, human names and
// approximate geographic bounds. Bounds are used by the web UI to frame the map
// when no unit position is known yet.
package theatre

// Bounds is a geographic bounding box.
type Bounds struct {
	MinLat float64 `json:"minLat"`
	MinLng float64 `json:"minLng"`
	MaxLat float64 `json:"maxLat"`
	MaxLng float64 `json:"maxLng"`
}

// Theatre is a DCS map.
type Theatre struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Bounds Bounds `json:"bounds"`
	// Tiles is true when map tiles are available for this theatre. It is
	// filled in at runtime by the API layer, which checks the tiles directory.
	Tiles bool `json:"tiles"`
	// TileMaxZoom is the highest zoom level present on disk for this theatre,
	// filled in at runtime. The UI uses it so a detailed tile pack is shown at
	// its full resolution instead of being capped at an arbitrary level.
	TileMaxZoom int `json:"tileMaxZoom,omitempty"`
	// TileMinZoom is the lowest zoom level present on disk. A pack whose tiles
	// start at zoom 8 shows nothing below that, so the UI must not frame the map
	// at a wider zoom or the map appears empty.
	TileMinZoom int `json:"tileMinZoom,omitempty"`
}

// builtin lists the DCS maps. The IDs are the ones DCS itself uses (declared in
// each terrain's entry.lua, and visible in the mission file's `theatre` field),
// so a theatre sent by the simulator always matches a known entry. This is the
// full set of terrains DCS sells, plus Marianas WWII.
//
// Getting these wrong is not cosmetic: airfields are indexed by theatre, so a
// mismatch makes them unreachable even though they were read successfully.
// "MarianaIslands" and "SinaiMap" are DCS's spellings, not "Marianas" and
// "Sinai", and the South Atlantic map is "Falklands".
//
// Bounds are a FALLBACK only. The real extent is measured from the map's own
// airfields and settlements at startup (see aerodrome.Catalog.Extent), which is
// exact and follows DCS. The literals below are used when a map is not installed
// and there is therefore nothing to measure; they are approximate by nature.
var builtin = []Theatre{
	{ID: "Caucasus", Name: "Caucasus", Bounds: Bounds{41.0, 36.5, 45.5, 45.0}},
	{ID: "Syria", Name: "Syria", Bounds: Bounds{32.0, 34.0, 37.5, 42.5}},
	{ID: "Nevada", Name: "Nevada (NTTR)", Bounds: Bounds{35.0, -117.5, 38.5, -113.0}},
	{ID: "PersianGulf", Name: "Persian Gulf", Bounds: Bounds{22.0, 47.0, 30.5, 57.0}},
	{ID: "MarianaIslands", Name: "Marianas", Bounds: Bounds{13.0, 144.4, 15.6, 146.1}},
	{ID: "MarianaIslandsWWII", Name: "Marianas (WWII)", Bounds: Bounds{13.0, 144.4, 15.6, 146.1}},
	{ID: "SinaiMap", Name: "Sinai", Bounds: Bounds{27.5, 32.0, 31.5, 35.5}},
	{ID: "Kola", Name: "Kola", Bounds: Bounds{65.5, 19.0, 70.5, 34.0}},
	{ID: "Afghanistan", Name: "Afghanistan", Bounds: Bounds{29.5, 60.0, 37.5, 72.0}},
	{ID: "Iraq", Name: "Iraq", Bounds: Bounds{29.0, 38.5, 37.5, 49.0}},
	{ID: "Falklands", Name: "Falklands", Bounds: Bounds{-53.0, -62.0, -50.0, -57.0}},
	{ID: "Normandy", Name: "Normandy", Bounds: Bounds{48.5, -2.0, 50.2, 1.5}},
	{ID: "TheChannel", Name: "The Channel", Bounds: Bounds{50.0, -2.0, 51.6, 3.0}},
	{ID: "GermanyCW", Name: "Cold War Germany", Bounds: Bounds{47.0, 5.5, 55.5, 15.5}},
}

// aliases maps the spellings that are not DCS's own onto the real theatre id.
// They exist because the wrong ids ("Marianas", "Sinai") were published in
// earlier releases and may still be in a browser's saved preference.
var aliases = map[string]string{
	"Marianas": "MarianaIslands",
	"Sinai":    "SinaiMap",
}

// Resolve returns the real theatre id for a query value, following the published
// aliases so an older saved preference keeps working. An unknown value is
// returned unchanged, letting the caller decide what to do with it.
func Resolve(id string) string {
	if real, ok := aliases[id]; ok {
		return real
	}
	return id
}

// All returns a copy of the built-in theatres.
func All() []Theatre {
	out := make([]Theatre, len(builtin))
	copy(out, builtin)
	return out
}

// Get returns the theatre with the given id, following the published aliases.
func Get(id string) (Theatre, bool) {
	if real, ok := aliases[id]; ok {
		id = real
	}
	for _, t := range builtin {
		if t.ID == id {
			return t, true
		}
	}
	return Theatre{}, false
}
