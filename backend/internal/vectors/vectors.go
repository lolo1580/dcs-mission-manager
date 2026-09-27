// Package vectors indexes the GeoJSON terrain layers imported from DCS terrain
// data (roads, railroads, rivers, borders, urban areas…). They are drawn on top
// of the map, which is what makes the map show DCS's own geography instead of a
// real-world approximation.
//
// The files are produced by `dcsmm import-vectors` and sit under
// <vectorsDir>/<theatre>/<layer>.geojson. This package only lists and resolves
// them; a single layer is served as-is, since GeoJSON is exactly what Leaflet
// consumes.
package vectors

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Layer is one GeoJSON file on disk.
type Layer struct {
	// Name is the file's base name, e.g. "DCS_Caucasus_Roads_oct2019".
	Name string `json:"name"`
	// File is the path relative to the vectors folder, for serving.
	File string `json:"file"`
	// Bounds is the layer's extent, when the collection declares one:
	// [minLng, minLat, maxLng, maxLat].
	Bounds []float64 `json:"bounds,omitempty"`
	// Size is the file size in bytes.
	Size int64 `json:"size,omitempty"`
}

// Catalog is the index of vector layers, per theatre.
type Catalog struct {
	byTheatre map[string][]Layer
	dir       string
}

// Load indexes every .geojson under dir, one folder per theatre.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{byTheatre: map[string][]Layer{}, dir: dir}
	if dir == "" {
		return c, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing folder is normal: the feature is optional.
		return c, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		theatre := e.Name()
		files, err := os.ReadDir(filepath.Join(dir, theatre))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.EqualFold(filepath.Ext(f.Name()), ".geojson") {
				continue
			}
			info, _ := f.Info()
			layer := Layer{
				Name: strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())),
				File: theatre + "/" + f.Name(),
			}
			if info != nil {
				layer.Size = info.Size()
			}
			layer.Bounds = readBounds(filepath.Join(dir, theatre, f.Name()))
			c.byTheatre[theatre] = append(c.byTheatre[theatre], layer)
		}
		sort.Slice(c.byTheatre[theatre], func(i, j int) bool {
			return c.byTheatre[theatre][i].Name < c.byTheatre[theatre][j].Name
		})
	}
	return c, nil
}

// Count returns how many layers are indexed in total.
func (c *Catalog) Count() int {
	n := 0
	for _, l := range c.byTheatre {
		n += len(l)
	}
	return n
}

// ByTheatre returns the layers of one theatre.
func (c *Catalog) ByTheatre(theatre string) []Layer {
	out := make([]Layer, len(c.byTheatre[theatre]))
	copy(out, c.byTheatre[theatre])
	return out
}

// Theatres lists the theatres that have vectors.
func (c *Catalog) Theatres() []string {
	out := make([]string, 0, len(c.byTheatre))
	for th := range c.byTheatre {
		out = append(out, th)
	}
	sort.Strings(out)
	return out
}

// Resolve returns the absolute path of a layer, refusing anything outside the
// indexed theatre folders. Only a path this catalogue produced resolves, which
// is what keeps the HTTP handler from serving an arbitrary file.
func (c *Catalog) Resolve(theatre, name string) (string, bool) {
	for _, l := range c.byTheatre[theatre] {
		if l.Name == name {
			return filepath.Join(c.dir, l.File), true
		}
	}
	return "", false
}

// readBounds reads just enough of a GeoJSON file to find its declared "bounds"
// array. Reading the whole file would mean parsing megabytes to show a list.
func readBounds(path string) []float64 {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	// The bounds field is written near the start by the importer
	// ({"type":...,"name":...,"bounds":[...]...}).
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n <= 0 {
		return nil
	}
	s := string(buf[:n])
	i := strings.Index(s, `"bounds":[`)
	if i < 0 {
		return nil
	}
	rest := s[i+len(`"bounds":[`):]
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return nil
	}
	parts := strings.Split(rest[:end], ",")
	if len(parts) != 4 {
		return nil
	}
	out := make([]float64, 0, 4)
	for _, p := range parts {
		var v float64
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%g", &v); err != nil {
			return nil
		}
		out = append(out, v)
	}
	return out
}
