# Live map

## What is displayed

The map receives the full state of units via **Server-Sent Events** (`GET /api/events`),
refreshed server-side once per second. Each unit carries:

| Field | Description |
|---|---|
| `id` | DCS runtime identifier (`ownship` for the player) |
| `type` | **Exact DCS type** (`F-16C_50`, `T-72B`, `SA-10`, `USS_Arleigh_Burke`…) |
| `label` | Optional name (pilot's callsign for the player's aircraft) |
| `category` | Family: `plane`, `heli`, `ground`, `ship`, `structure`, `other` |
| `coalition` | `blue`, `red`, `neutral` |
| `country` | DCS country |
| `lat`, `lng`, `alt` | Position |
| `heading` | Heading in degrees |
| `ownship` | `true` for the player's aircraft |
| `ageMs` | Age of the last update |

## Vehicle classification

The family (`category`) is inferred from the **DCS type** by heuristic rules
(`backend/internal/category`). These rules cover the essentials, but can be
**overridden** precisely, via a JSON file:

```jsonc
// categories.json (path via DCSMM_CATEGORIES, default ./categories.json)
{
  "F-16C_50": "plane",
  "SA-10": "ground",
  "USS_Arleigh_Burke": "ship"
}
```

A template is provided: `categories.example.json`.

## Filters and search

- **Coalitions**: Blue / Red / Neutral, with live counters.
- **Categories**: Planes / Helicopters / Ground / Ships / Structures / Others.
- **Search**: covers the type, the name and the country.
- **My aircraft only**: keeps only the player.
- **Flight trails**: polylines for planes and helicopters.

All filters are applied **in the browser**; the API also exposes the same
criteria as query parameters (`?category=`, `?coalition=`, `?ownship=true`, `?q=`).

## Flight trails

Trails are kept client-side (120 rolling points per aircraft), so
they fill up over the session and do not survive a reload.
Persistent (replayable) history belongs to Phase 3/4.

## Map tiles

In priority order:

1. **Authentic DCS tiles** — if a `<tilesDir>/<theatre>/` folder exists, the
   backend serves it via `/api/tiles/<theatre>/<z>/<x>/<y>.png` and the map uses it.
2. **Real basemap** — otherwise, an OSM basemap (or the URL from `DCSMM_BASEMAP_URL`) serves
   as a fallback.

The `tools/export-tiles.py` extractor slices a georeferenced map image into
tiles in the expected format. Direct export from DCS (multi-zoom F10 captures) is
planned as an evolution.

```bash
python tools/export-tiles.py \
  --image caucasus.png --theatre Caucasus \
  --bounds 41.0,36.5,45.5,45.0 \
  --min-zoom 2 --max-zoom 6 --out ./tiles
```

## Choice: SSE rather than WebSocket

Phase 1 updates are **downstream only**. SSE integrates
natively with the browser (`EventSource`), reconnects on its own and avoids the
complexity of a WebSocket. The **bidirectional** channel (commands to DCS) will arrive
in Phase 2 via a dedicated TCP socket on the Lua side.
