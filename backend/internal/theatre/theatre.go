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
}

var builtin = []Theatre{
	{ID: "Caucasus", Name: "Caucase", Bounds: Bounds{41.0, 36.5, 45.5, 45.0}},
	{ID: "Syria", Name: "Syrie", Bounds: Bounds{32.0, 34.0, 37.5, 42.5}},
	{ID: "Nevada", Name: "Nevada (NTTR)", Bounds: Bounds{35.0, -117.5, 38.5, -113.0}},
	{ID: "PersianGulf", Name: "Golfe Persique", Bounds: Bounds{22.0, 47.0, 30.5, 57.0}},
	{ID: "Marianas", Name: "Mariannes", Bounds: Bounds{13.0, 144.4, 15.6, 146.1}},
	{ID: "Sinai", Name: "Sinaï", Bounds: Bounds{27.5, 32.0, 31.5, 35.5}},
	{ID: "Kola", Name: "Kola", Bounds: Bounds{65.5, 19.0, 70.5, 34.0}},
	{ID: "Afghanistan", Name: "Afghanistan", Bounds: Bounds{29.5, 60.0, 37.5, 72.0}},
	{ID: "Iraq", Name: "Irak", Bounds: Bounds{29.0, 38.5, 37.5, 49.0}},
	{ID: "Falklands", Name: "Malouines", Bounds: Bounds{-53.0, -62.0, -50.0, -57.0}},
	{ID: "Normandy", Name: "Normandie", Bounds: Bounds{48.5, -2.0, 50.2, 1.5}},
	{ID: "TheChannel", Name: "La Manche", Bounds: Bounds{50.0, -2.0, 51.6, 3.0}},
}

// All returns a copy of the built-in theatres.
func All() []Theatre {
	out := make([]Theatre, len(builtin))
	copy(out, builtin)
	return out
}

// Get returns the theatre with the given id.
func Get(id string) (Theatre, bool) {
	for _, t := range builtin {
		if t.ID == id {
			return t, true
		}
	}
	return Theatre{}, false
}
