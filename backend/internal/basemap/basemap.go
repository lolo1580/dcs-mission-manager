// Package basemap describes the background map layers the web UI can display.
//
// DCS World is rendered by Leaflet, which expects Web Mercator (EPSG:3857)
// raster tiles addressed as {z}/{x}/{y}. Authentic DCS tiles are preferred when
// available (see the tiles directory), and these well-known public basemaps are
// the fallback: they are globally available, require no preprocessing, and are
// accurate enough to overlay unit positions.
//
// About real aeronautical charts, since the question keeps coming up:
//
//   - OpenAIP (openaip.net) serves aeronautical tiles but requires an API key,
//     which would break the "works out of the box" rule.
//   - open flightmaps has no public tile endpoint.
//   - VFRMAP serves FAA charts, which cover the United States only: a request
//     for the Caucasus returns an empty placeholder tile.
//   - DCS itself ships the F10 map imagery (Mods/terrains/<map>/RasterCharts).
//     The pixels are DDS/DXT5 tiles of 1024x1024, decodable in pure Go, but they
//     turn out to be overlay layers (roads, rivers, labels) drawn on black, not
//     the coloured terrain the F10 map shows. The projection problem is solved —
//     DCS's terrain coordinates are a transverse Mercator, fitted per map from
//     beacons.lua to a few tens of metres (see docs/terrain-projection.md) — but
//     the base imagery it would sit on is Eagle Dynamics' copyrighted work, so
//     re-serving it is a different matter from reading factual data such as
//     frequencies.
//
// The honest conclusion is the "aero" style below: a chart-like rendering of a
// free topographic base, with the aeronautical content (airfields, navaids,
// towns) drawn from DCS's own data. That content is factual, ours to read, and
// it is what a pilot actually needs.
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
		Attribution: "Imagery © Esri, Maxar, Earthstar Geographics",
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
		Name:        "Road",
		URL:         "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors",
		MaxZoom:     19,
		Subdomains:  []string{"a", "b", "c"},
	},
	{
		// Aeronautical look: the relief base, which already carries contour
		// lines, shaded relief and land use — close to what a chart shows — with
		// the airfields drawn as permanent callouts by the app.
		//
		// A pale wash was tried first and made the map unreadable; the relief
		// tiles need almost no filtering, only a slight calming so the labels
		// stay legible on top.
		ID:          "aero",
		Name:        "Aeronautical",
		URL:         "https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors, SRTM | © OpenTopoMap (CC-BY-SA)",
		MaxZoom:     17,
		Subdomains:  []string{"a", "b", "c"},
		ClassName:   "dcsmm-aero-tiles",
	},
	{
		ID:          "dark",
		Name:        "Dark",
		URL:         "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
		Attribution: "© OpenStreetMap contributors",
		MaxZoom:     19,
		Subdomains:  []string{"a", "b", "c"},
		ClassName:   "dcsmm-dark-tiles",
	},
}

// AeroID is the aeronautical basemap. The UI uses it to switch to chart-style
// labels and to reveal the airfields, since the style exists to show them.
const AeroID = "aero"

// All returns the list of selectable basemaps, including an optional custom one.
func All(customURL string) []Basemap {
	out := make([]Basemap, len(builtin))
	copy(out, builtin)
	if customURL != "" {
		out = append(out, Basemap{
			ID:          "custom",
			Name:        "Custom",
			URL:         customURL,
			Attribution: "Custom basemap",
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
