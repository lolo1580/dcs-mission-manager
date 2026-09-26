# Reference charts (scans)

This folder contains **scans of aeronautical charts** (JNC/ONC) of several
DCS theatres, at different resolutions. They are **not versioned** in git
(~1.3 GB) and remain local.

## Contents

### General charts (full-page scans)

| Folder | Theme |
|---|---|
| `DCS Caucasus Maps/` | Caucasus (the main scan actually covers a much larger area) |
| `DCS Nevada Maps/` | Nevada / NTTR |
| `DCS Normandy Maps/` | Normandy |
| `DCS PersianGulfMaps/` | Persian Gulf |
| `DCS_Syria_High_Detail_Maps/` | Syria |
| `DCS_The_Channel_High_Detail_Map/` | The Channel |
| `Marianas_High_Detail_Maps/` | Marianas (also contains `The Channel 8M.jpg`) |

### Aerodrome and procedure charts (Caucasus)

`DCS Caucasus Maps/` also contains a series of charts **per aerodrome**:

- `00_*`: general chart and legends.
- `NN_GND_*`: ground plans (*ground movement*).
- `NN_VAD_*`: visual approach/departure charts (*Visual Operation Chart*).
- `NN_PAR_*`: procedures.

Each chart indicates the coordinates (CRP), frequencies (Tower, Radar, TACAN, ILS)
and the runway. Useful for a future **briefings / charts** section — these are not
basemaps.

## Important: these scans are not usable as is

1. **Not georeferenced** — no calibration metadata (no `.jgw`, no
   GeoTIFF). The coordinates of the corners are unknown to the software.
2. **Conic projection** (Lambert type) — the edges are curved, whereas
   Leaflet expects **Web Mercator (EPSG:3857)**. A direct overlay is
   impossible without reprojection.
3. **Real-world charts** — these are not the F10 tiles of DCS. The rendering
   matches approximately, but not pixel for pixel.

### Aerodrome and procedure charts

The aerodrome charts (`NN_VAD_*`, `NN_GND_*`) are **documents** to consult,
not georeferenced basemaps. A future “briefings / charts” section
could display them as is. `tools/inspect-maps.py` can list them.

## Avenues of use

- **A. GeoTIFF** — if you obtain these charts as georeferenced GeoTIFF (EPSG:4326 or
  3857), they turn into tiles cleanly (gdal2tiles or a dedicated script).
- **B. Image + calibration** — providing **4 corners (lat/lng)** per image allows a
  linear mapping via `tools/export-tiles.py`. Enough for an indicative basemap,
  not enough to correct the conic curvature.
- **C. Real basemap** — this is what is currently active in the UI
  (Satellite, Relief, Road, Dark): no pre-processing, accurate enough
  to overlay the units.

## Rights

The scanned aeronautical charts may be subject to copyright or
terms of use depending on their source. They remain **local** and are
neither redistributed nor integrated into the binary. Check the licence before any sharing.
