// Package imgtiles turns a single calibrated map image into the
// tiles/<theatre>/<z>/<x>/<y>.png tree the manager serves.
//
// It complements the MBTiles importer: the freeware DCS F10 map packs (and the
// chart scans in maps_dcs/) are delivered as one large JPG, not as tiles. Given
// the image and the geographic bounds it covers, this slices it into Web
// Mercator tiles, which is what Leaflet asks for.
//
// The mapping from the image to the map is LINEAR over the bounds: the image is
// assumed to be a plate carrée (equirectangular) covering exactly the given
// rectangle. That is accurate for a DCS-sourced image produced with the
// projection in internal/geo, and it is the "indicative basemap" trade-off
// documented in maps_dcs/README.md for real-world charts, whose conic curvature
// a linear fit cannot remove.
package imgtiles

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	// Image decoders for the formats a pack is likely to ship.
	_ "image/jpeg"
	_ "image/png"
)

// Size is the edge of a generated tile, as expected by web maps.
const Size = 256

// Bounds is the geographic rectangle an image covers, in degrees.
type Bounds struct {
	MinLat, MinLng, MaxLat, MaxLng float64
}

// Valid reports whether the rectangle is usable.
func (b Bounds) Valid() bool {
	return b.MinLat < b.MaxLat && b.MinLng < b.MaxLng &&
		b.MinLat >= -90 && b.MaxLat <= 90 &&
		b.MinLng >= -180 && b.MaxLng <= 180
}

// ParseBounds reads "minLat,minLng,maxLat,maxLng".
func ParseBounds(s string) (Bounds, error) {
	var b Bounds
	n, err := fmt.Sscanf(s, "%f,%f,%f,%f", &b.MinLat, &b.MinLng, &b.MaxLat, &b.MaxLng)
	if err != nil || n != 4 {
		return b, fmt.Errorf("expected minLat,minLng,maxLat,maxLng, got %q", s)
	}
	if !b.Valid() {
		return b, fmt.Errorf("invalid bounds %q: min must be below max and inside the world", s)
	}
	return b, nil
}

// Stats reports what a conversion produced.
type Stats struct {
	Written int
	Skipped int
	MinZoom int
	MaxZoom int
}

// deg2num converts a position to fractional Web Mercator tile coordinates.
func deg2num(lat, lng float64, z int) (x, y float64) {
	n := math.Exp2(float64(z))
	latRad := lat * math.Pi / 180
	x = (lng + 180) / 360 * n
	y = (1 - math.Asinh(math.Tan(latRad))/math.Pi) / 2 * n
	return
}

// num2deg converts a tile corner back to latitude/longitude.
func num2deg(x, y float64, z int) (lat, lng float64) {
	n := math.Exp2(float64(z))
	lng = x/n*360 - 180
	lat = math.Atan(math.Sinh(math.Pi*(1-2*y/n))) * 180 / math.Pi
	return
}

// Convert slices the image into tiles under outDir/<theatre>/<z>/<x>/<y>.png.
//
// Zoom levels run from minZ to maxZ inclusive. A tile is written only when the
// image actually covers it. When force is false an existing tile is left alone,
// so an interrupted conversion resumes.
func Convert(imagePath, theatre string, bounds Bounds, minZ, maxZ int, outDir string, force bool) (Stats, error) {
	st := Stats{MinZoom: -1}
	if theatre == "" {
		return st, fmt.Errorf("imgtiles: a theatre id is required")
	}
	if !bounds.Valid() {
		return st, bounds2err(bounds)
	}
	if minZ < 0 || maxZ < minZ || maxZ > 22 {
		return st, fmt.Errorf("imgtiles: invalid zoom range %d..%d", minZ, maxZ)
	}

	f, err := os.Open(imagePath)
	if err != nil {
		return st, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return st, fmt.Errorf("imgtiles: decode %s: %w", imagePath, err)
	}
	st.MinZoom, st.MaxZoom = minZ, maxZ

	for z := minZ; z <= maxZ; z++ {
		// Tile range covering the image's rectangle.
		x0f, y0f := deg2num(bounds.MaxLat, bounds.MinLng, z)
		x1f, y1f := deg2num(bounds.MinLat, bounds.MaxLng, z)
		xStart, xEnd := int(math.Floor(x0f)), int(math.Ceil(x1f))
		yStart, yEnd := int(math.Floor(y0f)), int(math.Ceil(y1f))
		// Web Mercator is periodic in x; keep the range the image covers.
		if xEnd-xStart > 1<<20 {
			return st, fmt.Errorf("imgtiles: refusing an implausible tile range at zoom %d", z)
		}

		for tx := xStart; tx < xEnd; tx++ {
			for ty := yStart; ty < yEnd; ty++ {
				tile := renderTile(src, bounds, tx, ty, z)
				if tile == nil {
					continue // fully outside the image
				}
				dest := filepath.Join(outDir, theatre, strconv.Itoa(z), strconv.Itoa(tx), strconv.Itoa(ty)+".png")
				if !force {
					if _, err := os.Stat(dest); err == nil {
						st.Skipped++
						continue
					}
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return st, err
				}
				out, err := os.Create(dest)
				if err != nil {
					return st, err
				}
				err = png.Encode(out, tile)
				out.Close()
				if err != nil {
					return st, err
				}
				st.Written++
			}
		}
	}
	if st.MinZoom < 0 {
		st.MinZoom = minZ
	}
	return st, nil
}

// renderTile samples the source image over one tile's geographic footprint.
// It returns nil when the tile lies entirely outside the image.
func renderTile(src image.Image, bounds Bounds, tx, ty, z int) *image.RGBA {
	// The tile's geographic corners.
	tileLatTop, tileLngLeft := num2deg(float64(tx), float64(ty), z)
	tileLatBottom, tileLngRight := num2deg(float64(tx+1), float64(ty+1), z)

	// Overlap of the tile with the image, in degrees.
	minLat := math.Max(tileLatBottom, bounds.MinLat)
	maxLat := math.Min(tileLatTop, bounds.MaxLat)
	minLng := math.Max(tileLngLeft, bounds.MinLng)
	maxLng := math.Min(tileLngRight, bounds.MaxLng)
	if minLat >= maxLat || minLng >= maxLng {
		return nil
	}

	// Map degrees to source pixels (linear, as documented).
	b := src.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	toX := func(lng float64) float64 {
		return (lng - bounds.MinLng) / (bounds.MaxLng - bounds.MinLng) * w
	}
	toY := func(lat float64) float64 {
		return (bounds.MaxLat - lat) / (bounds.MaxLat - bounds.MinLat) * h
	}

	// The destination tile covers the full tile; the source covers only the
	// overlapping part, so the two are sampled proportionally.
	dst := image.NewRGBA(image.Rect(0, 0, Size, Size))
	for oy := 0; oy < Size; oy++ {
		// Latitude at this destination row, then the source row it corresponds to.
		fy := float64(oy) / Size
		lat := tileLatTop + (tileLatBottom-tileLatTop)*fy
		sy := toY(lat)
		for ox := 0; ox < Size; ox++ {
			fx := float64(ox) / Size
			lng := tileLngLeft + (tileLngRight-tileLngLeft)*fx
			sx := toX(lng)
			dst.SetRGBA(ox, oy, bilinear(src, sx, sy))
		}
	}
	return dst
}

// bilinear samples the image at fractional pixel coordinates.
func bilinear(src image.Image, x, y float64) color.RGBA {
	b := src.Bounds()
	// Coordinates are relative to the image origin.
	xi, yi := math.Floor(x), math.Floor(y)
	fx, fy := x-xi, y-yi

	at := func(px, py int) (float64, float64, float64, float64) {
		// Clamp to the image; outside pixels are transparent, so a tile edge
		// against the image border fades rather than smears.
		if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
			return 0, 0, 0, 0
		}
		r, g, bl, a := src.At(px, py).RGBA()
		return float64(r >> 8), float64(g >> 8), float64(bl >> 8), float64(a >> 8)
	}
	r00, g00, b00, a00 := at(int(xi), int(yi))
	r10, g10, b10, a10 := at(int(xi)+1, int(yi))
	r01, g01, b01, a01 := at(int(xi), int(yi)+1)
	r11, g11, b11, a11 := at(int(xi)+1, int(yi)+1)

	mix := func(v00, v10, v01, v11 float64) uint8 {
		top := v00*(1-fx) + v10*fx
		bot := v01*(1-fx) + v11*fx
		v := top*(1-fy) + bot*fy
		if v < 0 {
			v = 0
		} else if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	return color.RGBA{
		R: mix(r00, r10, r01, r11),
		G: mix(g00, g10, g01, g11),
		B: mix(b00, b10, b01, b11),
		A: mix(a00, a10, a01, a11),
	}
}

func bounds2err(b Bounds) error {
	return fmt.Errorf("imgtiles: incoherent bounds %+v", b)
}
