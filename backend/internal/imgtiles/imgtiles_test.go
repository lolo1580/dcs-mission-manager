package imgtiles

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestParseBounds covers the CLI input.
func TestParseBounds(t *testing.T) {
	b, err := ParseBounds("41.0,36.5,45.5,45.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if b.MinLat != 41 || b.MinLng != 36.5 || b.MaxLat != 45.5 || b.MaxLng != 45 {
		t.Errorf("bounds = %+v", b)
	}
	for _, bad := range []string{"", "1,2,3", "45,36,41,45", "0,0,0,10", "91,0,92,10"} {
		if _, err := ParseBounds(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

// writeSolidPNG writes a solid-colour image.
func writeSolidPNG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestConvertPlacesImageGeographically is the load-bearing test: it checks that
// the image lands at the right place on the map, not merely that tiles appear.
// The image covers lat 0..10, lng 0..10; in the single zoom-0 tile (the whole
// world, 256 px) that rectangle sits at a known pixel, and the image's centre
// must be there and nowhere else.
func TestConvertPlacesImageGeographically(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "map.png")
	const r, g, b = 200, 30, 60
	writeSolidPNG(t, src, 256, 256, color.RGBA{r, g, b, 255})

	out := filepath.Join(dir, "tiles")
	st, err := Convert(src, "Testland", Bounds{0, 0, 10, 10}, 0, 0, out, false)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if st.Written != 1 {
		t.Fatalf("z0 should produce exactly one tile, got %d", st.Written)
	}

	tilePath := filepath.Join(out, "Testland", "0", "0", "0.png")
	f, err := os.Open(tilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tile, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	// Where the image centre (lat 5, lng 5) should fall inside the z0 tile.
	n := math.Exp2(0)
	x := (5.0 + 180.0) / 360.0 * n * 256
	latRad := 5 * math.Pi / 180
	y := (1 - math.Asinh(math.Tan(latRad))/math.Pi) / 2 * n * 256
	got := tile.At(int(x), int(y))
	gr, gg, gb, _ := got.RGBA()
	if uint8(gr>>8) != r || uint8(gg>>8) != g || uint8(gb>>8) != b {
		t.Errorf("image centre should be at (%d,%d), got colour %v", int(x), int(y), got)
	}

	// And it must not cover the rest of the world: a far corner stays empty.
	// (lng -100 is far to the west of the image.)
	far := tile.At(40, 128)
	if fr, _, _, fa := far.RGBA(); fr>>8 > 10 || fa>>8 > 10 {
		t.Errorf("the world away from the bounds should be empty, got %v", far)
	}
}

// TestConvertRespectsZoomRange checks the level bounds are honoured.
func TestConvertRespectsZoomRange(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "map.png")
	writeSolidPNG(t, src, 64, 64, color.RGBA{1, 2, 3, 255})

	out := filepath.Join(dir, "tiles")
	st, err := Convert(src, "Testland", Bounds{41, 36.5, 45.5, 45}, 3, 4, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.MinZoom != 3 || st.MaxZoom != 4 {
		t.Errorf("zoom range = %d..%d, want 3..4", st.MinZoom, st.MaxZoom)
	}
	// There must be no tile outside the requested range.
	for _, z := range []string{"2", "5"} {
		if _, err := os.Stat(filepath.Join(out, "Testland", z)); err == nil {
			t.Errorf("zoom %s should not have been generated", z)
		}
	}
}

// TestConvertSkipsExisting covers resuming, and --force.
func TestConvertSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "map.png")
	writeSolidPNG(t, src, 64, 64, color.RGBA{9, 9, 9, 255})
	out := filepath.Join(dir, "tiles")

	if _, err := Convert(src, "Testland", Bounds{41, 36.5, 45.5, 45}, 0, 2, out, false); err != nil {
		t.Fatal(err)
	}
	st, err := Convert(src, "Testland", Bounds{41, 36.5, 45.5, 45}, 0, 2, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Written != 0 || st.Skipped == 0 {
		t.Errorf("second pass = %+v, want everything skipped", st)
	}
	st, err = Convert(src, "Testland", Bounds{41, 36.5, 45.5, 45}, 0, 2, out, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Skipped != 0 || st.Written == 0 {
		t.Errorf("force pass = %+v, want everything rewritten", st)
	}
}
