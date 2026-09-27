package dcsvectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Collection is a GeoJSON FeatureCollection.
type Collection struct {
	Type     string     `json:"type"`
	Name     string     `json:"name"`
	Bounds   []float64  `json:"bounds,omitempty"`
	Features []GFeature `json:"features"`
}

// GFeature is one GeoJSON feature.
type GFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Geometry   Geometry       `json:"geometry"`
}

// Geometry is a GeoJSON geometry. Coordinates is nested to the depth the type
// requires: [x,y] for Point, [[x,y]...] for lines, [[[x,y]...]...] for polygons.
type Geometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

// ToGeoJSON converts every shapefile under dir (non-recursive) into a GeoJSON
// FeatureCollection. When simplify is above zero, vertices closer than that many
// degrees to the previous kept one are dropped, which shrinks the output a lot
// for hand-drawn networks without changing their shape at map scale.
//
// Attribute columns listed in drop are omitted from the properties.
func ToGeoJSON(dir string, simplify float64, drop map[string]bool) (map[string]*Collection, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]*Collection{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".shp") {
			continue
		}
		s, err := Open(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[s.baseName] = s.collection(simplify, drop)
	}
	return out, nil
}

// collection renders one shapefile as GeoJSON.
func (s *Shapefile) collection(simplify float64, drop map[string]bool) *Collection {
	c := &Collection{
		Type:   "FeatureCollection",
		Name:   s.baseName,
		Bounds: []float64{s.MinX, s.MinY, s.MaxX, s.MaxY},
	}
	for _, f := range s.features {
		parts := f.Parts
		if simplify > 0 {
			parts = simplifyParts(parts, simplify)
		}
		if len(parts) == 0 {
			continue
		}
		g := Geometry{}
		switch f.Kind {
		case "Point":
			p := parts[0][0]
			g.Type = "Point"
			g.Coordinates = []float64{round(p[0]), round(p[1])}
		case "LineString":
			g.Type = "LineString"
			g.Coordinates = ring(parts[0])
		case "MultiLineString":
			g.Type = "MultiLineString"
			g.Coordinates = rings(parts)
		case "Polygon":
			g.Type = "Polygon"
			g.Coordinates = rings(parts)
		case "MultiPolygon":
			g.Type = "MultiPolygon"
			g.Coordinates = [][][][]float64{rings(parts)}
		default:
			continue
		}
		gf := GFeature{Type: "Feature", Geometry: g}
		if len(f.Props) > 0 {
			props := make(map[string]any, len(f.Props))
			for k, v := range f.Props {
				if drop[k] {
					continue
				}
				props[k] = v
			}
			if len(props) > 0 {
				gf.Properties = props
			}
		}
		c.Features = append(c.Features, gf)
	}
	return c
}

// rings renders a polygon's parts as closed rings: [][][2]float64 -> [][][]float64.
func rings(parts [][][2]float64) [][][]float64 {
	out := make([][][]float64, 0, len(parts))
	for _, p := range parts {
		out = append(out, ring(p))
	}
	return out
}

// ring renders one line: [][2]float64 -> [][]float64 of [lng, lat].
func ring(pts [][2]float64) [][]float64 {
	out := make([][]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, []float64{round(p[0]), round(p[1])})
	}
	return out
}

// round trims coordinates to five decimals (~1 m), which is far finer than any
// map zoom and roughly halves the file size against full float64 text.
func round(v float64) float64 {
	return math.Round(v*1e5) / 1e5
}

// simplifyParts drops points that are within tolerance of the last kept one.
// It never removes the first or last point of a part, so ends stay connected.
func simplifyParts(parts [][][2]float64, tol float64) [][][2]float64 {
	out := make([][][2]float64, 0, len(parts))
	for _, p := range parts {
		if len(p) <= 2 {
			out = append(out, p)
			continue
		}
		kept := make([][2]float64, 0, len(p))
		kept = append(kept, p[0])
		for i := 1; i < len(p)-1; i++ {
			last := kept[len(kept)-1]
			if math.Hypot(p[i][0]-last[0], p[i][1]-last[1]) >= tol {
				kept = append(kept, p[i])
			}
		}
		kept = append(kept, p[len(p)-1])
		if len(kept) >= 2 {
			out = append(out, kept)
		}
	}
	return out
}

// WriteTo writes a collection as compact JSON.
func WriteTo(path string, c *Collection) (int64, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}

// ParseDrop reads a comma-separated list of attribute names to omit.
func ParseDrop(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out[f] = true
		}
	}
	return out
}

// Describe summarises a collection for a log line.
func (c *Collection) Describe() string {
	kinds := map[string]int{}
	points := 0
	for _, f := range c.Features {
		kinds[f.Geometry.Type]++
		points += countPoints(f.Geometry.Coordinates)
	}
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(len(c.Features)))
	sb.WriteString(" features (")
	first := true
	for k, v := range kinds {
		if !first {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s %d", k, v)
		first = false
	}
	fmt.Fprintf(&sb, "), %d vertices", points)
	return sb.String()
}

func countPoints(v any) int {
	switch t := v.(type) {
	case []float64:
		return 1
	case []any:
		n := 0
		for _, e := range t {
			n += countPoints(e)
		}
		return n
	}
	return 0
}
