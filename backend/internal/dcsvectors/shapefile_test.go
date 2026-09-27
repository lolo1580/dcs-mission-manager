package dcsvectors

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeShapefile emits a minimal but valid shapefile with the given geometry
// type and one record, plus a matching dBase table. The header layout is the
// real one, so the reader is exercised against the actual format.
func writeShapefile(t *testing.T, base string, shapeType int, content []byte, fields []struct {
	Name string
	Size int
}, values []string) {
	t.Helper()

	// Content length is expressed in 16-bit words in the record header.
	clen := (len(content) + 1) / 2
	fileLen := 50 + 4 + clen // header (50 words) + record header (4 words) + content

	shp := make([]byte, 100+8+len(content))
	binary.BigEndian.PutUint32(shp[0:], 9994)
	binary.BigEndian.PutUint32(shp[24:], uint32(fileLen))
	binary.LittleEndian.PutUint32(shp[28:], 1000)
	binary.LittleEndian.PutUint32(shp[32:], uint32(shapeType))
	// Bounds: 0,0 .. 30,30
	for i, v := range []float64{0, 0, 30, 30} {
		binary.LittleEndian.PutUint64(shp[36+i*8:], math.Float64bits(v))
	}
	binary.BigEndian.PutUint32(shp[100:], 1)
	binary.BigEndian.PutUint32(shp[104:], uint32(clen))
	copy(shp[108:], content)
	if err := os.WriteFile(base+".shp", shp, 0o644); err != nil {
		t.Fatal(err)
	}

	// dBase: 32-byte header, 32 bytes per field descriptor, 0x0D terminator,
	// then fixed-width records.
	recLen := 1
	for _, f := range fields {
		recLen += f.Size
	}
	dbf := make([]byte, 32+32*len(fields)+1+recLen)
	dbf[0] = 0x03
	binary.LittleEndian.PutUint32(dbf[4:], 1) // one record
	binary.LittleEndian.PutUint16(dbf[8:], uint16(32+32*len(fields)+1))
	binary.LittleEndian.PutUint16(dbf[10:], uint16(recLen))
	off := 32
	for _, f := range fields {
		copy(dbf[off:off+11], f.Name)
		dbf[off+16] = byte(f.Size)
		off += 32
	}
	dbf[off] = 0x0D
	rec := off + 1
	dbf[rec] = ' ' // not deleted
	p := rec + 1
	for i, f := range fields {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		for len(v) < f.Size {
			v += " "
		}
		copy(dbf[p:p+f.Size], []byte(v[:f.Size]))
		p += f.Size
	}
	if err := os.WriteFile(base+".dbf", dbf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// polyLineContent builds a PolyLine record with one part and the given points.
func polyLineContent(pts [][2]float64) []byte {
	b := make([]byte, 48+16*len(pts))
	binary.LittleEndian.PutUint32(b[0:], 3)  // shape type: PolyLine
	binary.LittleEndian.PutUint32(b[36:], 1) // one part
	binary.LittleEndian.PutUint32(b[40:], uint32(len(pts)))
	// Record layout: type(4) bbox(32) nParts(4) nPoints(4) [partStarts][points]
	binary.LittleEndian.PutUint32(b[44:], 0) // the single part starts at point 0
	for i, p := range pts {
		binary.LittleEndian.PutUint64(b[48+i*16:], math.Float64bits(p[0]))
		binary.LittleEndian.PutUint64(b[48+i*16+8:], math.Float64bits(p[1]))
	}
	return b
}

// TestReadPolyLine checks the reader against a hand-built shapefile.
func TestReadPolyLine(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "roads")
	writeShapefile(t, base, 3, polyLineContent([][2]float64{{39.5, 43.1}, {39.6, 43.2}, {39.7, 43.3}}),
		[]struct {
			Name string
			Size int
		}{{"NAME", 10}},
		[]string{"Test road"})

	s, err := Open(base)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(s.Features()) != 1 {
		t.Fatalf("want 1 feature, got %d", len(s.Features()))
	}
	f := s.Features()[0]
	if f.Kind != "LineString" {
		t.Errorf("kind = %q, want LineString", f.Kind)
	}
	if len(f.Parts) != 1 || len(f.Parts[0]) != 3 {
		t.Fatalf("parts = %d with %d points, want 1 part of 3", len(f.Parts), len(f.Parts[0]))
	}
	if f.Parts[0][0][0] != 39.5 || f.Parts[0][0][1] != 43.1 {
		t.Errorf("first point = %v, want [39.5 43.1]", f.Parts[0][0])
	}
}

// TestAttributesAreRead checks the dBase table is joined to the shapes.
func TestAttributesAreRead(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "borders")
	writeShapefile(t, base, 3, polyLineContent([][2]float64{{40, 42}, {41, 43}}),
		[]struct {
			Name string
			Size int
		}{{"NAME", 12}},
		[]string{"Frontier"})

	s, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Fields) != 1 || s.Fields[0] != "NAME" {
		t.Fatalf("fields = %v, want [NAME]", s.Fields)
	}
	if got := s.Features()[0].Props["NAME"]; got != "Frontier" {
		t.Errorf("NAME = %q, want Frontier", got)
	}
}

// TestToGeoJSONShape checks the GeoJSON structure, which Leaflet consumes: a
// FeatureCollection whose coordinates are [lng, lat].
func TestToGeoJSONShape(t *testing.T) {
	dir := t.TempDir()
	writeShapefile(t, filepath.Join(dir, "roads"), 3,
		polyLineContent([][2]float64{{39.5, 43.1}, {39.7, 43.3}}),
		[]struct {
			Name string
			Size int
		}{{"NAME", 6}},
		[]string{"Road"})

	cols, err := ToGeoJSON(dir, 0, nil)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	c, ok := cols["roads"]
	if !ok {
		t.Fatalf("no 'roads' layer, got %v", len(cols))
	}
	if c.Type != "FeatureCollection" || len(c.Features) != 1 {
		t.Fatalf("collection = %s with %d features", c.Type, len(c.Features))
	}
	g := c.Features[0].Geometry
	if g.Type != "LineString" {
		t.Errorf("geometry type = %q", g.Type)
	}
	coords, ok := g.Coordinates.([][]float64)
	if !ok || len(coords) != 2 {
		t.Fatalf("coordinates shape is wrong: %T %v", g.Coordinates, g.Coordinates)
	}
	// GeoJSON is [lng, lat]; the shapefile stored lng first too, so they match.
	if coords[0][0] != 39.5 || coords[0][1] != 43.1 {
		t.Errorf("first coordinate = %v, want [39.5 43.1]", coords[0])
	}
	if c.Bounds == nil || len(c.Bounds) != 4 {
		t.Errorf("bounds = %v, want four values", c.Bounds)
	}
}

// TestSimplifyDropsNearbyPoints checks the vertex reduction, which is what keeps
// a hand-drawn road network from being tens of megabytes of JSON.
func TestSimplifyDropsNearbyPoints(t *testing.T) {
	// A line of 100 points, 0.00001 deg apart: far finer than any map zoom.
	var pts [][2]float64
	for i := 0; i < 100; i++ {
		pts = append(pts, [2]float64{39.0 + float64(i)*0.00001, 43.0})
	}
	dir := t.TempDir()
	writeShapefile(t, filepath.Join(dir, "rivers"), 3, polyLineContent(pts), nil, nil)

	plain, err := ToGeoJSON(dir, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	simplified, err := ToGeoJSON(dir, 0.001, nil)
	if err != nil {
		t.Fatal(err)
	}
	plainPts := plain["rivers"].Features[0].Geometry.Coordinates.([][]float64)
	simplifiedPts := simplified["rivers"].Features[0].Geometry.Coordinates.([][]float64)
	if len(simplifiedPts) >= len(plainPts) {
		t.Errorf("simplify kept %d of %d points, expected fewer", len(simplifiedPts), len(plainPts))
	}
	// The ends must survive: a road that stops short is worse than a coarse one.
	if simplifiedPts[0][0] != plainPts[0][0] || simplifiedPts[len(simplifiedPts)-1][0] != plainPts[len(plainPts)-1][0] {
		t.Error("simplify must keep the first and last point")
	}
}

// TestOpenRejectsGarbage guards the error path a user hits by pointing the
// command at the wrong file.
func TestOpenRejectsGarbage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not.shp")
	if err := os.WriteFile(p, []byte("definitely not a shapefile"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(p); err == nil {
		t.Fatal("a non-shapefile should be rejected")
	}
}
