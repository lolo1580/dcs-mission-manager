// Package dcsvectors reads ESRI shapefiles and writes GeoJSON, so the vector
// data that describes DCS's terrain can be drawn on the map.
//
// DCS ships no image of its terrain features, and its own binary formats
// (roads/*.rn4, Map/*.sup5) are proprietary, quantized and undocumented. The
// geometry that DCS actually uses has however been published as shapefiles by
// the author of a DCS-accurate map (Flappie, dcsmaps.com): roads, railroads,
// rivers, water bodies, borders, urban areas, airfields and towns, all in
// WGS84. This package turns those into GeoJSON, which Leaflet draws directly.
//
// The reader implements the ESRI shapefile specification for the geometry types
// the data uses (Point, PolyLine, Polygon) and the dBase III/IV attribute table
// that travels with it. Nothing else is needed, and no external dependency is
// pulled in for it.
package dcsvectors

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Geometry kinds, as declared in the shapefile header.
const (
	shapePoint    = 1
	shapePolyLine = 3
	shapePolygon  = 5
)

// Feature is one shape with its attributes.
type Feature struct {
	// Kind is "Point", "LineString", "MultiLineString", "Polygon" or
	// "MultiPolygon" — the GeoJSON type this feature maps to.
	Kind string
	// Coordinates is the raw ring/line structure: a list of parts, each a list
	// of [lng, lat] pairs. For a Point there is a single part with one pair.
	Parts [][][2]float64
	// Props holds the attribute table row.
	Props map[string]string
}

// Shapefile is one opened .shp/.dbf pair.
type Shapefile struct {
	baseName string
	shapeTyp int
	features []Feature
	// Fields is the attribute column order, as declared in the .dbf header.
	Fields []string
	// Bounds is the shapes' extent, from the .shp header.
	MinX, MinY, MaxX, MaxY float64
}

// Bounds returns the file's extent as "minLng,minLat,maxLng,maxLat".
func (s *Shapefile) Bounds() string {
	return fmt.Sprintf("%.5f,%.5f,%.5f,%.5f", s.MinX, s.MinY, s.MaxX, s.MaxY)
}

// Open reads a shapefile. path may be the .shp itself or the base name; the
// matching .dbf is loaded when present.
func Open(path string) (*Shapefile, error) {
	shpPath := path
	if !strings.EqualFold(filepath.Ext(shpPath), ".shp") {
		shpPath += ".shp"
	}
	shp, err := os.ReadFile(shpPath)
	if err != nil {
		return nil, err
	}
	if len(shp) < 100 {
		return nil, fmt.Errorf("dcsvectors: %s is too short to be a shapefile", shpPath)
	}
	// The file code is a big-endian 9994; the rest of the header is little-endian.
	if binary.BigEndian.Uint32(shp[0:4]) != 9994 {
		return nil, fmt.Errorf("dcsvectors: %s is not a shapefile (bad file code)", shpPath)
	}
	s := &Shapefile{
		baseName: strings.TrimSuffix(filepath.Base(shpPath), filepath.Ext(shpPath)),
		shapeTyp: int(binary.LittleEndian.Uint32(shp[28:32])),
		MinX:     math.Float64frombits(binary.LittleEndian.Uint64(shp[36:44])),
		MinY:     math.Float64frombits(binary.LittleEndian.Uint64(shp[44:52])),
		MaxX:     math.Float64frombits(binary.LittleEndian.Uint64(shp[52:60])),
		MaxY:     math.Float64frombits(binary.LittleEndian.Uint64(shp[60:68])),
	}

	// Records: an 8-byte header (record number, content length in 16-bit words),
	// both big-endian, then the content (little-endian).
	off := 100
	for off+8 <= len(shp) {
		contentLen := int(binary.BigEndian.Uint32(shp[off+4:off+8])) * 2
		body := shp[off+8 : min(off+8+contentLen, len(shp))]
		if f, ok := parseShape(body); ok {
			s.features = append(s.features, f)
		}
		off += 8 + contentLen
	}

	// Attributes travel in the .dbf of the same name.
	dbfPath := strings.TrimSuffix(shpPath, filepath.Ext(shpPath)) + ".dbf"
	if rows, fields, err := readDBF(dbfPath); err == nil {
		s.Fields = fields
		for i := range s.features {
			if i < len(rows) {
				s.features[i].Props = rows[i]
			}
		}
	}
	return s, nil
}

// Features returns the parsed shapes.
func (s *Shapefile) Features() []Feature { return s.features }

// parseShape decodes one record body.
func parseShape(b []byte) (Feature, bool) {
	if len(b) < 4 {
		return Feature{}, false
	}
	kind := int(binary.LittleEndian.Uint32(b[0:4]))

	if kind == shapePoint {
		if len(b) < 20 {
			return Feature{}, false
		}
		x := math.Float64frombits(binary.LittleEndian.Uint64(b[4:12]))
		y := math.Float64frombits(binary.LittleEndian.Uint64(b[12:20]))
		return Feature{Kind: "Point", Parts: [][][2]float64{{{x, y}}}}, true
	}
	if kind != shapePolyLine && kind != shapePolygon {
		// Null shapes and unsupported types are skipped rather than guessed at.
		return Feature{}, false
	}
	if len(b) < 44 {
		return Feature{}, false
	}
	nParts := int(binary.LittleEndian.Uint32(b[36:40]))
	nPoints := int(binary.LittleEndian.Uint32(b[40:44]))
	if nParts <= 0 || nPoints <= 0 {
		return Feature{}, false
	}
	partsOff := 44
	pointsOff := partsOff + nParts*4
	if pointsOff+16 > len(b) {
		return Feature{}, false
	}
	// Part start indices, then the point array.
	starts := make([]int, nParts)
	for i := 0; i < nParts; i++ {
		starts[i] = int(binary.LittleEndian.Uint32(b[partsOff+i*4 : partsOff+i*4+4]))
	}
	pts := make([][2]float64, nPoints)
	for i := 0; i < nPoints; i++ {
		o := pointsOff + i*16
		if o+16 > len(b) {
			break
		}
		pts[i] = [2]float64{
			math.Float64frombits(binary.LittleEndian.Uint64(b[o : o+8])),
			math.Float64frombits(binary.LittleEndian.Uint64(b[o+8 : o+16])),
		}
	}

	parts := make([][][2]float64, 0, nParts)
	for i := 0; i < nParts; i++ {
		start := starts[i]
		end := nPoints
		if i+1 < nParts {
			end = starts[i+1]
		}
		if start < 0 || end > nPoints || start >= end {
			continue
		}
		parts = append(parts, pts[start:end])
	}
	if len(parts) == 0 {
		return Feature{}, false
	}

	f := Feature{Parts: parts}
	switch kind {
	case shapePolyLine:
		if len(parts) == 1 {
			f.Kind = "LineString"
		} else {
			f.Kind = "MultiLineString"
		}
	case shapePolygon:
		if len(parts) == 1 {
			f.Kind = "Polygon"
		} else {
			f.Kind = "MultiPolygon"
		}
	}
	return f, true
}

// readDBF parses a dBase III/IV attribute table, returning the rows and the
// field names in order.
func readDBF(path string) ([]map[string]string, []string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if len(b) < 32 {
		return nil, nil, fmt.Errorf("dcsvectors: %s is too short to be a dbf", path)
	}
	nRecords := int(binary.LittleEndian.Uint32(b[4:8]))
	headerLen := int(binary.LittleEndian.Uint16(b[8:10]))
	recordLen := int(binary.LittleEndian.Uint16(b[10:12]))
	if headerLen <= 32 || recordLen <= 0 || headerLen > len(b) {
		return nil, nil, fmt.Errorf("dcsvectors: %s has an implausible header", path)
	}

	// Field descriptors run from offset 32 to the header terminator (0x0D).
	type field struct {
		name string
		size int
	}
	var fields []field
	for off := 32; off+32 <= headerLen && b[off] != 0x0D; off += 32 {
		name := strings.TrimRight(string(b[off:off+11]), "\x00")
		fields = append(fields, field{name: name, size: int(b[off+16])})
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.name
	}

	rows := make([]map[string]string, 0, nRecords)
	for r := 0; r < nRecords; r++ {
		start := headerLen + r*recordLen
		if start+recordLen > len(b) {
			break
		}
		// Byte 0 is the deletion flag; 0x2A means deleted.
		if b[start] == 0x2A {
			rows = append(rows, map[string]string{})
			continue
		}
		row := make(map[string]string, len(fields))
		pos := start + 1
		for _, f := range fields {
			if pos+f.size > start+recordLen || pos+f.size > len(b) {
				break
			}
			v := strings.TrimSpace(string(b[pos : pos+f.size]))
			if v != "" {
				row[f.name] = v
			}
			pos += f.size
		}
		rows = append(rows, row)
	}
	return rows, names, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
