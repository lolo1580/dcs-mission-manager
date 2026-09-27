# DCS terrain projection (research)

This document records a measured result, so the work does not have to be redone:
**DCS's terrain coordinates are a transverse Mercator projection, and its
parameters can be recovered per map from DCS's own data — to a few tens of
metres over an entire theatre.**

It is the missing piece for placing the F10 raster charts (`RasterCharts/`) on
the Leaflet map, which is what "authentic DCS tiles" needs.

## Why this is possible at all

DCS gives us ground truth for free. Every beacon in
`Mods/terrains/<map>/beacons.lua` carries **both** representations of the same
point:

```lua
position     = { -1321.801758, 12.340623, 246748.500000 };  -- terrain: x, altitude, z (metres)
positionGeo  = { latitude = 45.039907, longitude = 37.396436 };  -- the real position
```

So the mapping `(lat, lng) -> (x, z)` can be **measured** instead of guessed, over
hundreds of points spread across the whole map. (The third component of
`position` is the altitude, which is how the aerodrome elevation is already read.)

## Method

For a candidate projection `P(lat,lng)` (TM or Lambert conformal conic), fit the
best 2-D **linear** map from `P` to the terrain `(x,z)`, and report the residual
in metres. A linear map can only absorb scale, rotation and offset; if the
projection is right, the residual is small. Worst outliers are trimmed
iteratively (90 % kept per round, 3 rounds) so a few bad beacon coordinates
cannot hide a good fit.

The tool is `backend/cmd/geoexplore`:

```
cd backend
go run ./cmd/geoexplore "E:\Games\DCS World\Mods\terrains" Caucasus Kola PersianGulf MarianaIslands
```

## Result

Spherical **transverse Mercator** wins by an order of magnitude. Lambert
conformal conic, and a plain affine in lat/lng, are both far worse.

| Map | pairs | TM RMS | TM robust | affine (lat/lng) RMS |
|---|---|---|---|---|
| Caucasus | 164 | **34 m** | 10 m | 11 568 m |
| Kola | 69 | **21 m** | 11 m | 16 982 m |
| Persian Gulf | 101 | **75 m** | 17 m | 7 029 m |
| Marianas | 19 | **1 m** | 0 m | 28 m |

For scale, these maps span 600-1300 km: 34 m over 600 km is ~0.006 %.

The large affine error was the red herring: beacons far from the map (Odessa,
Yalta, Melitopol for the Caucasus) carry coordinates that do not follow the same
projection, and they alone accounted for the ~12 km affine residual. Trimming
them drops the affine error to ~3 km and the TM error to ~10 m.

### Fitted parameters

Idealised spherical TM with `R = 6 371 000 m`, then a 2-D affine (the affine is
what absorbs the real scale, rotation and false origin):

| Map | lat0 | lng0 | k0 |
|---|---|---|---|
| Caucasus | 44.890 | 33.288 | 1.0537 |
| Kola | 70.068 | 20.880 | 0.9906 |
| Persian Gulf | 29.824 | 57.288 | 1.0370 |
| Marianas | 16.286 | 146.771 | 0.9766 |

`k0` is near 1 for every map, which is the expected shape of a real TM. Each map
has its own central meridian, so the parameters must be fitted per terrain —
which is exactly what the tool does, from the installed data.

## Decoded DDS tiles are overlay linework, not imagery

`Mods/terrains/<map>/RasterCharts/` holds DDS/DXT5 tiles, 1024×1024, **with
mipmaps** (so decodable in pure Go, no C toolchain — `backend/cmd/tileprobe`
does it) on a grid (`x<col>_z<row>`), indexed by `rasterCharts.sup5`. Four maps
ship one big `rasterCharts.zip` (Kola 1.0 GB, Persian Gulf 0.9 GB, Marianas
0.5-0.7 GB).

Decoding tiles from **four different maps** shows the same thing: **almost-black
linework**, not coloured imagery. Every pixel of an opaque tile sits between 0 and
8 on each RGB channel:

```
Kola            R[0..8] G[0..4] B[0..8]   100 % opaque
PersianGulf     R[0..8] G[0..4] B[0..8]   100 % opaque
MarianaIslands  R[1..8] G[1..3] B[2..8]   43-95 % opaque
```

So the raster charts are **overlay layers** (roads, rivers, labels), and the
coloured base the F10 map shows is produced elsewhere. The imagery is also Eagle
Dynamics' copyrighted work: reading a frequency is one thing, re-serving the
artwork is another, and this project reads factual data only.

## Vector geometry does exist in DCS

The F10 map is not shipped as an image, but its **vectors** are shipped as data,
and they are structured:

- `roads/<map>.rn4` (`landscape4::lRoadNetwork`) and
  `AirfieldsTaxiways/<field>.rn4` are the same container. Its **header is
  decoded**:
  - `[u32][u32][u32]` header, then a length-prefixed type string
    (`landscape4::lRoadNetwork`), then the **layer table**: a count followed by
    `[u32 len][name]` records — Caucasus: `road`, `rdb`, `tunnel_hose`, `rw`,
    `roki_tunnel_road`, `vpp_center`, `vpp_border`, `vpp_border_big`, `taxiway`,
    `taxiway_border`, `stopline`, `stand_border`, `vpp_border_vbig`; Kola's
    `AirfieldsTaxiways` files name their surfaces by width (`taxiway_23m`,
    `runway_60m`…).
  - After the layer table come **16.16 fixed-point** values (multiples of
    65536 = 1.0), i.e. metadata and indices, and a node graph of small integers.

  The **geometry itself is not readable as plain float32 vertices**. Genuine
  terrain coordinates (terrain-scale, not fixed-point multiples) do appear — 662
  in an 188 KB taxiway file, 1 078 084 in the 200 MB Caucasus road network — but
  they are **scattered, never in dense vertex arrays**, and their neighbourhoods
  are high-entropy garbage rather than coordinate sequences. So the vertex data is
  **quantized relative to the spatial grid and/or compressed in a proprietary
  way**, and no zlib/deflate stream is present to inflate.

  Conclusion: decoding these containers into polylines is a genuine
  reverse-engineering project. The header and the layer semantics are known; the
  vertex encoding is not.
- `Map/<map>.sup5` (`landscape5::sup5File`, 48-200 MB) holds a `LINELIST` with a
  `colorIndex` and a `VIEW_IN_MAPTEX` shader define: this is literally the linework
  the F10 map draws (coastlines, borders, rivers, and the **MGRS grid**).

Both are proprietary binary containers with a layer table and a spatial grid
index, and neither format is documented. Decoding them is a real reverse-engineering
project, not a conversion — but it is a **licence-clean** path to a DCS-shaped
vector map, built from DCS's own factual geometry.

## What this does NOT unlock

The projection solves where things go on a map. It does **not** supply map
imagery, and there is none to take from DCS. A tempting shortcut is to cache
third-party tiles (OpenStreetMap, Esri) over DCS's extent, using the projection to
frame them — but caching those tiles in bulk is **not permitted by their terms of
use**: OSM's tile usage policy forbids bulk downloading, and Esri's World Imagery
is licensed, not free to store.

The realistic, licence-clean options are therefore:

1. **Vector map from DCS's own geometry** (`roads/*.rn4`, `Map/*.sup5`), drawn
   with the projection. Stylised, not pixel-identical, no DCS needed at runtime.
   Requires decoding the two containers above.
2. **A georeferenced image** the user has the right to use (scan, personal capture,
   purchase): `tools/export-tiles.py` turns it into tiles, and the projection makes
   that exact rather than approximate.
3. **Live third-party basemaps**, as today, for anything that need not be
   DCS-accurate.

## What a consumer would implement

1. Fit `(lat0, lng0, k0)` and the 2-D affine per terrain, once, from
   `beacons.lua` (cache it: it never changes for a given map build).
2. Invert: terrain `(x,z)` -> TM plane -> `(lat,lng)`, then Web Mercator for
   Leaflet.
3. For imagery: decode `roads/*.rn4` (and `Map/*.sup5`) into polylines and draw
   them, **or** place a user-supplied calibrated image by the same projection.
