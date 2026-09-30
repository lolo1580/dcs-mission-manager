# Live telemetry

## What the manager receives

DCS pushes unit telemetry (positions, the player's aircraft, the world objects) to
the manager, which samples it for **statistics and analysis**. The live map has
been removed, so this feed is no longer drawn: it is what feeds the flight trails,
the heatmaps and the sortie analysis (Phase 4 bis), and the airfields tab's
"nearest field" lookup.

Each unit carries:

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

## Server-side filtering

The unit feed is also available as a plain JSON snapshot at `GET /api/state`, with
the same criteria as query parameters: `?category=`, `?coalition=`, `?ownship=true`
and `?q=` (a substring of the type or label). The airfields tab uses it to find the
player's ownship and sort fields by distance.

## Choice: SSE rather than WebSocket

Updates are **downstream only**. SSE integrates
natively with the browser (`EventSource`), reconnects on its own and avoids the
complexity of a WebSocket. A **bidirectional** channel (commands to DCS) goes
through the dedicated TCP socket on the Lua side, not through the browser.
