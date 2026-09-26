// Package basemap describes the background map layers the web UI can display.
//
// DCS World is rendered by Leaflet, which expects Web Mercator (EPSG:3857)
// raster tiles addressed as {z}/{x}/{y}. Authentic DCS tiles are preferred when
// available (see the tiles directory), and these well-known public basemaps are
// the fallback: they are globally available, require no preprocessing, and are
// accurate enough to overlay unit positions.
package basemap

// Basemap is a tile layer the UI can select.
type Basemap struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Attribution string   `json:"attribution"`
	MaxZoom     int      `json:"maxZoom"`
	Subdomains  []string `json:"subdomains,omitempty"`
	// ClassName is an optional CSS class applied to the tile layer, used for
	// purely client-side effects such as the dark-mode filter.
	ClassName string `json:"className,omitempty"`
}

// Built-in basemaps, ordered by usefulness for tactical overlays.
//
// Every entry must be usable without an API key, so that the map keeps working
// out of the box. Dark mode is therefore implemented as a CSS filter over the
// standard OSM raster tiles rather than a third-party dark tile service.
var builtin = []Basemap{
	{
		ID:          "satellite",
		Name:        "Satellite",
		URL:         "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
		Attribution: "Imagerie © Esri, Maxar, Earthstar Geographics",
		MaxZoom:     19,
	},
	{
		ID:          "topo",
		Name:        "Relief",
		URL:         "https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors, SRTM | © OpenTopoMap (CC-BY-SA)",
		MaxZoom:     17,
		Subdomains:  []string{"a", "b", "c"},
	},
	{
		ID:          "osm",
		Name:        "Routier",
		URL:         "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors",
		MaxZoom:     19,
		Subdomains:  []string{"a", "b", "c"},
	},
	{
		ID:          "dark",
		Name:        "Sombre",
		URL:         "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors",
		MaxZoom:     19,
		Subdomains:  []string{"a", "b", "c"},
		ClassName:   "dcsmm-dark-tiles",
	},
}

// All returns the list of selectable basemaps, including an optional custom one.
func All(customURL string) []Basemap {
	out := make([]Basemap, len(builtin))
	copy(out, builtin)
	if customURL != "" {
		out = append(out, Basemap{
			ID:          "custom",
			Name:        "Personnalisé",
			URL:         customURL,
			Attribution: "Fond de carte personnalisé",
			MaxZoom:     19,
		})
	}
	return out
}

// DefaultID is the basemap used when none is configured.
const DefaultID = "satellite"

// Get returns a built-in basemap by id.
func Get(id string) (Basemap, bool) {
	for _, b := range builtin {
		if b.ID == id {
			return b, true
		}
	}
	return Basemap{}, false
}
