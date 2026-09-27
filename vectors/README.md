# Terrain vectors (GeoJSON)

This folder holds the **GeoJSON terrain layers** the map draws over its basemap:
roads, railroads, rivers, water bodies, urban areas, borders, airfields and towns,
one subfolder per theatre.

They are **not versioned** (about 20 MB for the Caucasus) and are not shipped with
the binary. Regenerate them with:

```powershell
dcsmm import-vectors --in <folder with .shp/.dbf> --theatre Caucasus --simplify 0.0002
```

Or set `DCSMM_VECTORS_DIR` to point the manager at another location.

## Where the data comes from

The shapefiles published with **Flappie's DCS 2.5 Caucasus map** (dcsmaps.com) —
the geometry that exists *in the game*, extracted from DCS's own mission editor
and terrain, not a real-world map. They are distributed on Google Drive under
"Data for Flappie's DCS 2.5 Caucasus map", with the author's permission to reuse
them **as long as derivative work is distributed for free**.

| Layer | Shapefile |
|---|---|
| Roads | `DCS_Caucasus_Roads_oct2019` |
| Railroads | `DCS_Caucasus_Railroads_may2019` |
| Rivers | `DCS_Caucasus_Rivers_10avril2019` |
| Water bodies | `DCS_Caucasus_Waterbeds_may2019` |
| Urban areas | `DCS_Caucasus_Urban_june2019` |
| Borders | `DCS_Caucasus_Borders` |
| Airfields | `DCS_Caucasus_Airbases_may2019` |
| Towns | `DCS_Caucasus_Towns_oct2019` |

Each `.shp` needs its `.shx`, `.dbf` and `.prj` alongside it. The `.prj` declares
`GCS_WGS_1984`, so no reprojection is needed: the coordinates are already
longitude/latitude.

See `docs/f10-maps.md` for the full procedure.

## Rights

The individual values (a coordinate, a road segment) are facts and are not
themselves copyrightable, but the source is a community work, redistributed under
its author's terms. Keep these files local and credit the author — the manager
shows whatever `DCSMM_TILES_ATTRIBUTION` names, for instance
`Map by Flappie (dcsmaps.com)`.
