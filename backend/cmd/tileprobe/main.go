// Command tileprobe decodes a DCS RasterCharts tile (DDS/DXT5, with mipmaps) to
// PNG, so the F10 imagery DCS ships can be inspected and georeferenced. It is a
// scratch tool for exploration; it is not shipped.
//
// usage: tileprobe <in.dds> <out.png>
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// decodeDXT5 decodes mip level 0 of a DXT5 (BC3) surface into RGBA.
func decodeDXT5(data []byte, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bw, bh := w/4, h/4
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			off := (by*bw + bx) * 16
			if off+16 > len(data) {
				return img
			}
			block := data[off : off+16]
			var alpha [8]uint8
			// Alpha: two endpoints, then 16 3-bit indices (little-endian 48 bits).
			a0, a1 := block[0], block[1]
			alpha[0], alpha[1] = a0, a1
			if a0 > a1 {
				for i := 1; i <= 6; i++ {
					alpha[1+i] = uint8((int(a0)*(7-i) + int(a1)*i) / 7)
				}
			} else {
				for i := 1; i <= 4; i++ {
					alpha[1+i] = uint8((int(a0)*(5-i) + int(a1)*i) / 5)
				}
				alpha[6], alpha[7] = 0, 255
			}
			var aidx uint64
			for i := 0; i < 6; i++ {
				aidx |= uint64(block[2+i]) << (8 * uint(i))
			}
			// Color: RGB565 endpoints then 16 2-bit indices.
			c0 := binary.LittleEndian.Uint16(block[8:])
			c1 := binary.LittleEndian.Uint16(block[10:])
			col := func(c uint16) (uint8, uint8, uint8) {
				r := uint8((c >> 11) & 0x1f)
				g := uint8((c >> 5) & 0x3f)
				b := uint8(c & 0x1f)
				return uint8(r * 255 / 31), uint8(g * 255 / 63), uint8(b * 255 / 31)
			}
			r0, g0, b0 := col(c0)
			r1, g1, b1 := col(c1)
			pal := [4][3]uint8{{r0, g0, b0}, {r1, g1, b1}}
			if c0 > c1 {
				pal[2] = [3]uint8{uint8((2*int(r0) + int(r1)) / 3), uint8((2*int(g0) + int(g1)) / 3), uint8((2*int(b0) + int(b1)) / 3)}
				pal[3] = [3]uint8{uint8((int(r0) + 2*int(r1)) / 3), uint8((int(g0) + 2*int(g1)) / 3), uint8((int(b0) + 2*int(b1)) / 3)}
			} else {
				pal[2] = [3]uint8{uint8((int(r0) + int(r1)) / 2), uint8((int(g0) + int(g1)) / 2), uint8((int(b0) + int(b1)) / 2)}
				pal[3] = [3]uint8{0, 0, 0}
			}
			cidx := binary.LittleEndian.Uint32(block[12:])
			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					i := py*4 + px
					ci := (cidx >> (2 * uint(i))) & 3
					ai := (aidx >> (3 * uint(i))) & 7
					c := pal[ci]
					img.SetRGBA(bx*4+px, by*4+py, color.RGBA{c[0], c[1], c[2], alpha[ai]})
				}
			}
		}
	}
	return img
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: tileprobe <in.dds> <out.png>")
		return
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if string(b[0:4]) != "DDS " {
		fmt.Fprintln(os.Stderr, "not a DDS file")
		os.Exit(1)
	}
	h := int(binary.LittleEndian.Uint32(b[12:]))
	w := int(binary.LittleEndian.Uint32(b[16:]))
	mips := binary.LittleEndian.Uint32(b[28:])
	fourCC := string(b[84:88])
	fmt.Printf("DDS %dx%d mips=%d fourCC=%q\n", w, h, mips, fourCC)
	if fourCC != "DXT5" {
		fmt.Fprintln(os.Stderr, "only DXT5 is handled")
		os.Exit(1)
	}
	img := decodeDXT5(b[128:], w, h)
	// Report how much of the tile is opaque: a colored imagery layer is
	// essentially all-opaque, an overlay layer is mostly transparent.
	var opaque, total int
	var minR, maxR, minG, maxG, minB, maxB uint8 = 255, 0, 255, 0, 255, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			total++
			if c.A > 200 {
				opaque++
				if c.R < minR {
					minR = c.R
				}
				if c.R > maxR {
					maxR = c.R
				}
				if c.G < minG {
					minG = c.G
				}
				if c.G > maxG {
					maxG = c.G
				}
				if c.B < minB {
					minB = c.B
				}
				if c.B > maxB {
					maxB = c.B
				}
			}
		}
	}
	fmt.Printf("opaque %d/%d (%.1f%%)  R[%d..%d] G[%d..%d] B[%d..%d]\n",
		opaque, total, float64(opaque)*100/float64(total), minR, maxR, minG, maxG, minB, maxB)

	f, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", os.Args[2])
}
