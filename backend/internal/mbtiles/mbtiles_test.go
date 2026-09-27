package mbtiles

import (
	"bytes"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// makeTile returns a small PNG of a solid colour, standing in for a map tile.
func makeTile(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeArchive builds an MBTiles file with the flat `tiles` schema.
func writeArchive(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE metadata (name TEXT, value TEXT)`,
		`CREATE TABLE tiles (zoom_level INTEGER, tile_column INTEGER, tile_row INTEGER, tile_data BLOB)`,
		`CREATE UNIQUE INDEX tile_index ON tiles (zoom_level, tile_column, tile_row)`,
		`INSERT INTO metadata VALUES ('name', 'Test Pack'), ('format', 'png'), ('minzoom', '0'), ('maxzoom', '1')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	// At z=1 the grid is 2x2. TMS rows are counted from the bottom, so TMS row 0
	// is the *lower* half of the map — the one XYZ calls row 1.
	insert := `INSERT INTO tiles VALUES (?, ?, ?, ?)`
	for _, tc := range []struct {
		z, col, tmsRow int
		col2           color.Color
	}{
		{0, 0, 0, color.RGBA{10, 0, 0, 255}},
		{1, 0, 0, color.RGBA{0, 20, 0, 255}}, // TMS bottom-left
		{1, 0, 1, color.RGBA{0, 0, 30, 255}}, // TMS top-left
		{1, 1, 1, color.RGBA{40, 40, 40, 255}},
	} {
		if _, err := db.Exec(insert, tc.z, tc.col, tc.tmsRow, makeTile(t, tc.col2)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestConvertFlipsTMSRows is the load-bearing test: MBTiles counts rows from the
// bottom, the tile URL scheme from the top. Getting this wrong mirrors every map
// vertically, which is easy to miss and hard to diagnose.
func TestConvertFlipsTMSRows(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "pack.mbtiles")
	writeArchive(t, archive)

	db, err := Open(archive)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	out := filepath.Join(dir, "tiles")
	st, err := db.Convert(out, "Testland", false)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if st.Tiles != 4 || st.Written != 4 {
		t.Fatalf("stats = %+v, want 4 read / 4 written", st)
	}
	if st.MinZoom != 0 || st.MaxZoom != 1 {
		t.Errorf("zoom range = %d..%d, want 0..1", st.MinZoom, st.MaxZoom)
	}

	// The tile stored at TMS row 0 must land at XYZ row 1 (the green one), and
	// TMS row 1 at XYZ row 0 (the blue one).
	green := filepath.Join(out, "Testland", "1", "0", "1.png")
	blue := filepath.Join(out, "Testland", "1", "0", "0.png")
	if _, err := os.Stat(green); err != nil {
		t.Fatalf("expected the TMS row 0 tile at XYZ row 1: %v", err)
	}
	if _, err := os.Stat(blue); err != nil {
		t.Fatalf("expected the TMS row 1 tile at XYZ row 0: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "Testland", "0", "0", "0.png")); err != nil {
		t.Errorf("the z0 tile is missing: %v", err)
	}

	// Verify the colours to be sure the rows were not just renamed.
	got := readPixel(t, green)
	if got.G < 15 || got.R > 5 {
		t.Errorf("XYZ row 1 should hold the green (TMS row 0) tile, got %+v", got)
	}
	got = readPixel(t, blue)
	if got.B < 25 || got.R > 5 {
		t.Errorf("XYZ row 0 should hold the blue (TMS row 1) tile, got %+v", got)
	}
}

// TestConvertSkipsExisting covers resuming an interrupted import.
func TestConvertSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "pack.mbtiles")
	writeArchive(t, archive)

	db, err := Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	out := filepath.Join(dir, "tiles")
	if _, err := db.Convert(out, "Testland", false); err != nil {
		t.Fatal(err)
	}
	st, err := db.Convert(out, "Testland", true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Written != 0 || st.Skipped != 4 {
		t.Errorf("second run = %+v, want 0 written / 4 skipped", st)
	}
}

// TestMetadata reads the descriptive table.
func TestMetadata(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "pack.mbtiles")
	writeArchive(t, archive)

	db, err := Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	md, err := db.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Name != "Test Pack" || md.Format != "png" || md.MaxZoom != 1 {
		t.Errorf("metadata = %+v", md)
	}
}

// TestOpenRejectsNonSQLite guards the error path a user hits by pointing the
// command at the wrong file.
func TestOpenRejectsNonSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.db")
	if err := os.WriteFile(path, []byte("definitely not sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("opening a non-SQLite file should fail")
	}
}

func readPixel(t *testing.T, path string) color.RGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := img.At(4, 4).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}
