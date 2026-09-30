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
and the runway. They are indexed by `internal/charts` and shown **as documents**
in the manager's chart viewer — not georeferenced, so they are never overlaid on a
map.

## How they are used

The scans are **not georeferenced**: no calibration metadata (no `.jgw`, no
GeoTIFF), and the projection is a conic (Lambert type), not Web Mercator. They are
therefore displayed whole, as documents, matched to an airfield by name or ICAO
code. `tools/inspect-maps.py` lists what is present.

## Rights

The scanned aeronautical charts may be subject to copyright or
terms of use depending on their source. They remain **local** and are
neither redistributed nor integrated into the binary. Check the licence before any sharing.
