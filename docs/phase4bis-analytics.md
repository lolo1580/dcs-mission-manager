# Analytical maps & sortie analysis (Phase 4 bis)

> **The Analysis view has been removed.** The tab, its plot and the
> `/api/analytics/*` routes are gone, and the sortie computation with them. What
> remains is the collection described below: `internal/tracker` still samples
> positions and records losses, so the history is kept in the database. This
> document is kept as the reference for that collection (and for the day the view
> comes back); the endpoints and the sortie statistics it describes no longer exist.

## Principle: no per-theatre projection

Modules 4.3 (maps) and 4.5 (sortie) rely on the **position history**.
Rather than using the debriefs' internal `x/y` coordinates
(which require a projection specific to each theatre), the backend samples the
**lat/lng positions** already received as telemetry. Result: **no geographic
calibration**, all theatres work the same way.

## Collection: `internal/tracker`

A periodic sampler (`DCSMANAGER_TRACK_INTERVAL`, default 3 s) records
for each unit: position, altitude, heading, speed, load factor.

- **Tracks**: the movements, to reconstruct the trajectories.
- **Losses**: when a unit **disappears** from the world for longer than
  `DCSMANAGER_TRACK_GRACE` (default 15 s), it is recorded as a loss with its
  last known position. This is an honest approximation of “destroyed or
  deactivated” — DCS does not always send an explicit event.
- **Reappearance**: if the unit returns, it is no longer counted as a loss.

If no mission has been announced (only `Export.lua` installed), the tracker
automatically creates a session mission so nothing is lost.

## Retention

`DCSMANAGER_TRACK_RETENTION` (default 24 h): an hourly pass deletes data
older than that, so the database does not grow indefinitely on a server that
runs continuously.

## Advanced telemetry (ownship)

`Export.lua` enriches the player's message with, **if the server allows it**
(`allow_ownship_export`): true and indicated airspeed, Mach, angle of attack, ground
altitude, load factor (`LoGetAccelerationUnits`). Otherwise, only the positions
are recorded — the speed/G columns stay empty rather than wrong.

## API

| Route | Description |
|---|---|
| `GET /api/analytics/heatmap?source=positions\|losses&grid=0.05` | Aggregated heat points (grid in degrees, no projection) |
| `GET /api/analytics/tracks` | Tracks of the most active aircraft + sortie stats |
| `GET /api/analytics/sorties` | Sortie stats only |

## Interface

- New **Analysis** tab: heat map source (Traffic / Losses) and
  a **sortie analysis** table per unit (duration, distance, max altitude,
  max speed, max G, number of points).
- A **top-down plot** (plain SVG, no map library) draws the heat map as coloured
  grid cells and the recorded flight paths as polylines, over a shared bounding
  box with corner coordinates. Longitude is scaled by `cos(latitude)` so the shape
  keeps its true proportions. Each layer can be toggled; the choice is remembered.
  The sortie table also shows a distance bar per row, scaled to the longest sortie.

The distance is computed with the **haversine** formula (great circle), on the
points actually recorded.

## Accepted limitations

- Loss detection is **heuristic** (prolonged absence), not a DCS
  event. Coupling with the Phase 2 `crash`/`pilot_death`/`kill` events
  is possible but deliberately left aside to avoid double counting.
- Telemetry (G, speed, angle of attack) is only available for **one's own
  aircraft** in multiplayer; the other units only have positions.
- Sampling at 3 s is enough for a readable trajectory; it is
  configurable if more precision is needed.

## Tests

`internal/tracker` covers: sampling, loss detection (and non-duplication),
reappearance, haversine, and sortie analysis (extremes and distance).
