# Live telemetry

## What the manager receives

DCS pushes unit telemetry (positions, the player's aircraft, the world objects) to
the manager, which stores position samples and uses the current ownship position
for the airfields tab's "nearest field" lookup. The live map and analytical map
views have been removed; stored samples remain available for future analysis.

`Export.lua` also sends a small `heartbeat` every export cycle. It contains no
position and creates no map unit. This keeps the feed active when DCS is running
but the player is a spectator or the server denies ownship/world export. If a
previously active feed becomes silent, the interface reports an interrupted
export; silence alone cannot prove that DCS is paused.
The export callback always schedules a future model time and keeps the earlier
valid deadline returned by another export tool. Returning the current time
caused a single heartbeat followed by an interrupted feed after the 5-second
unit TTL. The manager sends its own heartbeat at the configured interval even
when another tool requests more frequent callbacks.

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
// categories.json (path via DCSMANAGER_CATEGORIES, default ./categories.json)
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
