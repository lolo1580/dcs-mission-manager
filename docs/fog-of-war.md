# Fog of war (respecting mission options)

## The problem

A DCS mission can **limit what the F10 map reveals** (“F10 View Options”).
A live map tool that ignores these options **reveals what the mission hides**:
that is involuntary cheating, especially on a multiplayer server.

It is the only gap we had compared to the reference (Bergison
MovingMap, MizMap), and it is a problem of **integrity**, not comfort. It is
therefore treated as a priority.

## Where the values come from

Directly from **DCS**, in `MissionEditor/modules/Options/optionsDb.lua`:

| `optionsView` value | DCS label | Meaning |
|---|---|---|
| `optview_onlymap` | MAP ONLY | no units |
| `optview_myaircraft` | MY A/C | my aircraft only |
| `optview_allies` | **FOG OF WAR** | allies + contacts detected by sensors |
| `optview_onlyallies` | ALLIES ONLY | allies only |
| `optview_all` | ALL | everything |

The Lua hook transmits `Sim.getMissionOptions()` at mission start; the
backend infers the visibility policy from it.

## Principle: restrictive by default

`internal/visibility` applies a simple rule: **never show more than
what DCS would show**.

- `map_only` / `my_aircraft` → the player's aircraft only;
- `allies` / `only_allies` → the player's aircraft + their **coalition**;
- `all` → everything;
- **options not received** → treated as `allies` (the safe case).

Two explicit choices:

- **Filtering is server-side.** Filtering in the browser would be
  bypassable: the data must never leave the backend if the mission
  hides it.
- **“FOG OF WAR” is approximated as “allies only”.** The contacts
  actually detected depend on each coalition's sensors at time T,
  which the Lua export does not provide. We therefore take the **safe subset**: we
  under-display rather than over-display.

The player's aircraft is **always** kept: showing one's own position is
never a leak, and without it the map would be unusable.

## Configuration

| Variable | Default | Effect |
|---|---|---|
| `DCSMM_REVEAL_ALL_UNITS` | `false` | `true` disables filtering (solo use / mission design) |

When filtering is disabled, the interface clearly indicates it: it is a “god”
mode, accepted and visible.

## API

| Route | Description |
|---|---|
| `GET /api/visibility` | Active policy (mode, label, override, note) |

The policy is also attached to each state frame (`/api/state`, SSE) under the
`visibility` key, and a `visibility` event is broadcast on each change.

## Interface

A banner appears below the header as soon as filtering is active, with the mode and
its limit (for example: “Fog of war (allies) — contacts detected by
sensors are not reproduced (restrictive)”).

## Accepted limitations

- **Real fog of war not reproduced**: without the sensor detections, we cannot
  show the detected enemy contacts. This is a limitation of the Lua export, not a
  design choice.
- Filtering depends only on the **coalition**; it does not take into account the
  per-role layer masks (`visibleUnitLayersMask`), which are finer, that DCS
  also applies.

## Tests

`internal/visibility`: matching of DCS values, filtering by mode,
**non-leakage of enemy and neutral units** in all restrictive modes,
override, absence of player aircraft, description.
