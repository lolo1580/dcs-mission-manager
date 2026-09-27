// Package mbtiles reads MBTiles archives (the SQLite format used by offline map
// packs) and turns them into the tiles/<theatre>/<z>/<x>/<y>.png tree the
// manager serves.
//
// It exists because the DCS F10 map is not shipped as an image: DCS composes it
// at runtime. Community packs of the assembled F10 map are published as MBTiles,
// and this package is what lets the manager display them as a basemap.
//
// MBTiles stores rows with a bottom-left origin (TMS), while Leaflet addresses
// tiles from the top-left (XYZ), so the row index is flipped on the way out —
// without that, every map would be mirrored vertically.
package mbtiles

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// Image decoders: a pack may store JPEG or PNG tiles.
	_ "image/jpeg"
	_ "image/png"

	_ "modernc.org/sqlite"
)

// Metadata is the archive's descriptive table.
type Metadata struct {
	Name        string
	Format      string
	MinZoom     int
	MaxZoom     int
	Bounds      [4]float64 // minlng, minlat, maxlng, maxlat
	Attribution string
}

// DB is an opened MBTiles archive.
type DB struct {
	db *sql.DB
}

// Open opens an MBTiles archive read-only.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("mbtiles: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mbtiles: %s is not a readable SQLite database: %w", path, err)
	}
	return &DB{db: db}, nil
}

// Close releases the archive.
func (d *DB) Close() error { return d.db.Close() }

// Metadata reads the descriptive table. Missing keys are left at their zero
// value rather than treated as errors: packs vary in what they declare.
func (d *DB) Metadata() (Metadata, error) {
	var md Metadata
	rows, err := d.db.Query(`SELECT name, value FROM metadata`)
	if err != nil {
		return md, fmt.Errorf("mbtiles: no metadata table: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return md, err
		}
		switch strings.ToLower(k) {
		case "name":
			md.Name = v
		case "format":
			md.Format = strings.ToLower(v)
		case "attribution":
			md.Attribution = v
		case "minzoom":
			md.MinZoom, _ = strconv.Atoi(v)
		case "maxzoom":
			md.MaxZoom, _ = strconv.Atoi(v)
		case "bounds":
			parts := strings.Split(v, ",")
			for i := 0; i < 4 && i < len(parts); i++ {
				md.Bounds[i], _ = strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
			}
		}
	}
	return md, rows.Err()
}

// TileSource returns the SQL that lists every tile as (z, column, tmsRow, data),
// following whichever schema the archive uses: a flat `tiles` table, or the
// normalised `map` + `images` pair (the standard `tiles` view joins them, but
// some packs ship the tables without the view).
func (d *DB) TileSource() (string, error) {
	var kind string
	err := d.db.QueryRow(`SELECT type FROM sqlite_master WHERE name = 'tiles'`).Scan(&kind)
	if err == nil {
		return `SELECT zoom_level, tile_column, tile_row, tile_data FROM tiles`, nil
	}
	err = d.db.QueryRow(`SELECT type FROM sqlite_master WHERE name = 'map'`).Scan(&kind)
	if err == nil {
		return `SELECT m.zoom_level, m.tile_column, m.tile_row, i.tile_data
		        FROM map m JOIN images i ON i.tile_id = m.tile_id`, nil
	}
	return "", fmt.Errorf("mbtiles: neither a tiles table nor a map/images pair was found")
}

// Stats reports what a conversion did.
type Stats struct {
	Tiles      int
	Written    int
	Skipped    int
	MinZoom    int
	MaxZoom    int
	Transcoded int // tiles that came in as JPEG and were re-encoded to PNG
}

// Convert writes the archive into dir/<theatre>/<z>/<x>/<y>.png.
//
// Tiles are always written as PNG, so the backend's tile handler needs no
// change: it serves <theatre>/<z>/<x>/<y>.png and nothing else. A JPEG tile is
// therefore re-encoded, which is lossless on the way in but larger on disk; that
// trade keeps one code path in the server.
//
// When skipExisting is set, a tile already on disk is left alone, so an
// interrupted conversion can be resumed.
func (d *DB) Convert(dir, theatre string, skipExisting bool) (Stats, error) {
	var st Stats
	if theatre == "" {
		return st, fmt.Errorf("mbtiles: a theatre id is required")
	}

	query, err := d.TileSource()
	if err != nil {
		return st, err
	}
	rows, err := d.db.Query(query)
	if err != nil {
		return st, fmt.Errorf("mbtiles: read tiles: %w", err)
	}
	defer rows.Close()

	st.MinZoom = -1
	for rows.Next() {
		var z, col, tmsRow int
		var data []byte
		if err := rows.Scan(&z, &col, &tmsRow, &data); err != nil {
			return st, err
		}
		st.Tiles++
		if st.MinZoom < 0 || z < st.MinZoom {
			st.MinZoom = z
		}
		if z > st.MaxZoom {
			st.MaxZoom = z
		}

		// TMS -> XYZ: the row index is counted from the bottom in MBTiles and
		// from the top in the tile URL scheme.
		row := (1 << z) - 1 - tmsRow

		dest := filepath.Join(dir, theatre, strconv.Itoa(z), strconv.Itoa(col), strconv.Itoa(row)+".png")
		if skipExisting {
			if _, err := os.Stat(dest); err == nil {
				st.Skipped++
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return st, err
		}

		out, transcoded, err := toPNG(data)
		if err != nil {
			// One unreadable tile must not abort a whole pack; report and go on.
			fmt.Fprintf(os.Stderr, "mbtiles: z%d x%d y%d: %v\n", z, col, row, err)
			continue
		}
		if transcoded {
			st.Transcoded++
		}
		if err := os.WriteFile(dest, out, 0o644); err != nil {
			return st, err
		}
		st.Written++
	}
	return st, rows.Err()
}

// toPNG returns PNG bytes for a tile, converting only when needed.
func toPNG(data []byte) (out []byte, transcoded bool, err error) {
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return data, false, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false, fmt.Errorf("decode tile: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}
