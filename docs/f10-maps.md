# DCS F10 map imagery

The F10 map is the one thing DCS does **not** ship as a file: it is composed at
runtime from terrain geometry, ground textures, roads and vector linework, so
there is nothing in `Mods/terrains/` to extract. `RasterCharts/` holds only
overlay linework (roads, rivers, labels) on a black background — not the coloured
map a pilot sees.

Community-made captures of the assembled F10 map exist, are **freeware**, and are
published on Eagle Dynamics' own User Files. This is what the manager displays.

## Where to get them

Eagle Dynamics User Files — each is the assembled F10 map for one terrain, "exact
replicas of the one you see in DCS", licence *Freeware — unlimited distribution*
(author: Baileywa / asherao). These cover **7 of the 14 terrains**:

| Terrain | File |
|---|---|
| Caucasus | `digitalcombatsimulator.com/en/files/3309262/` |
| Persian Gulf | `.../en/files/3309246/` |
| Nevada | `.../en/files/3309261/` |
| Normandy | `.../en/files/3309247/` |
| The Channel | `.../en/files/3310307/` |
| Syria | `.../en/files/3311902/` |
| Marianas | `.../en/files/3317279/` |
| Caucasus + Persian Gulf + Syria (lower res) | `.../en/files/3310608/` |

Newer packs are published as **`.mbtiles`** (a SQLite tile archive), which is the
easiest to import:

- **Persian Gulf** — free, released on the ED forums in December 2025 ("DCS
  Persian Gulf F10 Map for CombatFlite").
- **Cold War Germany**, **Afghanistan** — F10 resolution `.mbtiles` published by
  graveyard4DCS (Patreon; some packs free).

### Coverage of all 14 terrains

| Terrain | F10 pack |
|---|---|
| Caucasus | **installed** — Flappie's DCS-accurate map, zooms 8-13 (`fetch-tiles` below) |
| Cold War Germany, Afghanistan | `.mbtiles` (graveyard4DCS) |
| Persian Gulf | `.mbtiles`, free (ED forums, December 2025) |
| Nevada, Normandy, The Channel, Syria, Marianas | freeware image packs on ED User Files |
| Marianas WWII | none published; the Marianas pack covers the area |
| Sinai, Kola, Iraq, South Atlantic (Falklands) | none found so far |

A terrain without a pack still works: it uses the live basemaps (satellite,
relief, road, dark) and simply has no **DCS (official)** entry.

The manager knows all 14 DCS terrains (`internal/theatre`), using DCS's own ids —
`SinaiMap`, `GermanyCW`, and `Falklands` for the South Atlantic map.

### Caucasus, step by step

Flappie's map is the reference for the Caucasus: it shows what exists **in the
game**, and is served as tiles over zooms 8-13. The bounds and zoom range come
from its viewer page.

```powershell
# The whole thing (zooms 8..13). Run the levels in order; it resumes, so a
# second run only fills what is missing.
dcsmm fetch-tiles --url "http://dcsmaps.com/caucasus/{z}/{x}/{y}.png" `
    --theatre Caucasus --tms --bounds 40.8151520679,36.55,45.8109913793,45.5349433511 `
    --min-zoom 8 --max-zoom 10 --delay 60

dcsmm fetch-tiles --url "http://dcsmaps.com/caucasus/{z}/{x}/{y}.png" `
    --theatre Caucasus --tms --bounds 40.8151520679,36.55,45.8109913793,45.5349433511 `
    --min-zoom 11 --max-zoom 13 --delay 60
```

Then show the author on the map:

```powershell
$env:DCSMM_TILES_ATTRIBUTION = "Map by Flappie (dcsmaps.com)"
dcsmm
```

Pick **DCS (official)** in the basemap selector. The manager frames the map to the
pack's extent and limits zooming to the levels actually present, so the map never
shows the empty space beyond the tiles.

> These are captures of Eagle Dynamics' map, redistributed by their authors for
> DCS use. They are **not** shipped with this project and must not be
> redistributed with it. Keep them local.

### A published tile set (no download needed here)

Some DCS-accurate maps are published as a **tile server**, which the manager can
bring local. The reference is Flappie's DCS 2.5 Caucasus map: it shows what
exists **in the game** (roads, railroads, rivers, coastlines, forests, bridges —
extracted from mission-editor screenshots), not the real world, and it is served
as tiles at `http://dcsmaps.com/caucasus/{z}/{x}/{y}.png` (TMS rows, zoom 8-13).

```powershell
dcsmm fetch-tiles --url "http://dcsmaps.com/caucasus/{z}/{x}/{y}.png" `
    --theatre Caucasus --tms --min-zoom 8 --max-zoom 12 `
    --bounds 40.8151520679,36.55,45.8109913793,45.5349433511
```

| Flag | Meaning |
|---|---|
| `--url <template>` | tile URL with `{z}`, `{x}`, `{y}` (required) |
| `--theatre <id>` | store it under this terrain (required) |
| `--bounds <b>` | `minLat,minLng,maxLat,maxLng` to download (required) |
| `--min-zoom` / `--max-zoom` | zoom levels (defaults 8 and 12) |
| `--tms` | the source counts rows from the bottom (TMS, not XYZ) |
| `--delay <ms>` | minimum pause between requests (default 120) |
| `--concurrency <n>` | parallel requests (default 4, kept low on purpose) |
| `--out`, `--force` | as above |

The bounds and zoom range for Flappie's map come from its own viewer page. The
download is paced and retried on purpose: these are community servers, not CDNs.
An interrupted run resumes, so it can be re-run safely.

**Credit the author.** Flappie's map is free to use with attribution. Set
`DCSMM_TILES_ATTRIBUTION` and the manager shows it on the map:

```powershell
$env:DCSMM_TILES_ATTRIBUTION = "Map by Flappie (dcsmaps.com)"
```

## Importing

### MBTiles packs (tiles, the easiest)

```powershell
dcsmm import-tiles --mbtiles "PersianGulf-F10.mbtiles" --theatre PersianGulf
```

| Flag | Meaning |
|---|---|
| `--mbtiles <file>` | the archive to import (required) |
| `--theatre <id>` | store it under this terrain, e.g. `PersianGulf` (required) |
| `--out <dir>` | tiles directory, defaults to `DCSMM_TILES_DIR` (`./tiles`) |
| `--force` | overwrite tiles that already exist (resume-safe without it) |

The importer writes `tiles/<theatre>/<z>/<x>/<y>.png`. Two details matter and are
handled for you:

- **TMS → XYZ.** MBTiles counts rows from the bottom, the tile URL scheme from
  the top. Without the flip every map is mirrored vertically.
- **PNG output.** JPEG tiles are re-encoded, so the server keeps one code path
  (`/api/tiles/.../<y>.png`). A JPEG pack is decoded once and stored as PNG.

### Images (the freeware packs are one big JPG)

Those packs ship a single large image, not tiles. Given the rectangle it covers,
the manager slices it:

```powershell
dcsmm import-image --image "DCS Caucasus Map 5M.jpg" --theatre Caucasus `
    --bounds 41.0,36.5,45.5,45.0 --min-zoom 2 --max-zoom 5
```

| Flag | Meaning |
|---|---|
| `--image <file>` | the map image, JPG or PNG (required) |
| `--theatre <id>` | store it under this terrain (required) |
| `--bounds <b>` | `minLat,minLng,maxLat,maxLng` the image covers (required) |
| `--min-zoom` / `--max-zoom` | zoom levels to generate (defaults 2 and 6) |
| `--out <dir>` | tiles directory, defaults to `DCSMM_TILES_DIR` |
| `--force` | overwrite tiles that already exist |

**Bounds** are the rectangle the image covers, in degrees, in the same
`minLat,minLng,maxLat,maxLng` order `tools/export-tiles.py` uses. The manager's
per-theatre bounds (`internal/theatre`) are a starting point, but a real chart
rarely covers exactly that rectangle: adjust until the map lines up.

**Alignment caveat.** The image is treated as a plate carrée covering the bounds
exactly (a linear mapping). That is the trade-off documented in
`maps_dcs/README.md`: a real chart is drawn on a conic projection, so a linear fit
cannot remove its curvature and the edges may be off by a few kilometres. For a
DCS-sourced image — anything produced with the projection in `internal/geo` — the
mapping is exact.

Restart the manager afterwards: the theatre then reports `tiles: true` and the UI
offers a **DCS (official)** basemap at the zoom levels present on disk.

## Terrain vectors (roads, rivers, urban areas)

The F10 map is a rendering, but the *geography* behind it is data — and a
DCS-accurate copy of it was published as shapefiles together with the map above
(Flappie, dcsmaps.com, free to reuse as long as derivative work is free). Those
layers are what makes the map match the simulator: roads, railroads, rivers,
water bodies, urban areas, borders, airfields and towns, all in WGS84.

```powershell
dcsmm import-vectors --in <folder with .shp/.dbf> --theatre Caucasus --simplify 0.0002
```

| Flag | Meaning |
|---|---|
| `--in <dir>` | folder holding the `.shp`/`.dbf` files (required) |
| `--theatre <id>` | store them under this terrain (required) |
| `--out <dir>` | GeoJSON root, defaults to `DCSMM_VECTORS_DIR` (`./vectors`) |
| `--simplify <deg>` | drop vertices closer than this; `0.0002` (~20 m) is a good default |
| `--drop <names>` | attribute columns to omit, comma-separated |

Then press **Terrain** in the header. The layers are drawn over whatever basemap
is active, which is the useful combination: a satellite or relief background with
DCS's own roads, towns and borders on top.

Measured on the Caucasus: 8 layers, 2 726 roads, 1 650 rivers, 1 723 urban areas,
1 718 towns — about 20 MB. Layers are fetched once and cached in the browser, so
toggling the overlay does not re-download them.

### Where the shapefiles come from

The vector data for Flappie's DCS Caucasus map, published on Google Drive under
"Data for Flappie's DCS 2.5 Caucasus map":

| Layer | File |
|---|---|
| Roads | `DCS_Caucasus_Roads_oct2019` |
| Railroads | `DCS_Caucasus_Railroads_may2019` |
| Rivers | `DCS_Caucasus_Rivers_10avril2019` |
| Water bodies | `DCS_Caucasus_Waterbeds_may2019` |
| Urban areas | `DCS_Caucasus_Urban_june2019` |
| Borders | `DCS_Caucasus_Borders` |
| Airfields | `DCS_Caucasus_Airbases_may2019` |
| Towns | `DCS_Caucasus_Towns_oct2019` |

Each layer is a `.shp` with its `.shx`, `.dbf` and `.prj` alongside; the `.prj`
declares `GCS_WGS_1984`, so no reprojection is needed. Download each part under
its own file id (Drive returns the `.shp` bytes for any id if you reuse one).

## If a pack is misaligned

The projection used by DCS is recoverable per map from DCS's own `beacons.lua`
(see `docs/terrain-projection.md` and `internal/geo`). It converts terrain
coordinates — from `roads/`, `beacons.lua`, anything — to latitude/longitude and
back, to a few tens of metres, which is what any calibration or offset correction
would be measured against.

## Why not extract DCS's own tiles

- The coloured F10 imagery **does not exist on disk**; DCS renders it at runtime.
- `RasterCharts/` decodes (DDS/DXT5, pure Go — `cmd/tileprobe`) but is black
  linework, not imagery.
- The vector formats (`roads/*.rn4`, `Map/*.sup5`) hold the real geometry, but
  they are proprietary, quantized and grid-indexed; their headers and layer names
  are decoded, the vertex encoding is not.

Community packs avoid all of this: they are simply the map as DCS draws it.
