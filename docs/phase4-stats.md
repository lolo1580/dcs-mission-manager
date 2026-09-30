# Advanced statistics (Phase 4)

## Two scopes

All statistics exist in **two scopes**:

- **`career`** (default): aggregation over all missions, **by UCID**, so
  a pilot's history survives their callsign changes;
- **`mission`**: limited to one mission (the current one, or `?missionId=N`).

API side: `?scope=career` or `?scope=mission` (+ `&missionId=N`). Interface side:
the **Career / Mission** selector at the top of the Statistics tab.

## Sources and merging

| Source | What it reliably provides |
|---|---|
| `net.get_stat` (snapshots) | Score, air/ground/ship kills, crashes, landings, ejections, ping |
| `onGameEvent` | Detail of the kills (weapons, targets), friendly-fire, deaths |
| `debrief.log` | Complete timeline of the mission |

The score and the kill counters come from the **DCS snapshots** (authoritative
source). The details (weapon used, target type) come from the
**events**. This is the merging announced from the start.

## Linking an event to a player

DCS event arguments are **numeric player identifiers**, not
names. To be able to attribute a death or a friendly-fire to a pilot, the
backend records the session DCS ID (`dcs_player_id`) next to each statistics
snapshot, then **joins** the event on that ID.

This is what allows, for example, computing the **K/D** per pilot even though DCS
does not directly provide the number of deaths in `net.get_stat`.

## The modules

| Module | Route | Content |
|---|---|---|
| Overview | `/api/stats/overview` | Missions, pilots, kills, deaths, crashes, ejections, friendly-fire, coalitions |
| **4.1 Pilots** | `/api/stats/pilots` | Score, kills, deaths, K/D, landings, ejections, crashes, FF, average ping |
| **4.2 Weapons** | `/api/stats/weapons` | Kills and friendly-fire per weapon, breakdown of targets and platforms |
| **4.4 Balance** | (in overview) | Score/kills per coalition, comparative bars |
| **4.6 Network** | `/api/stats/network` | Average/max ping and number of samples per pilot |
| **4.7 Vehicles** | `/api/stats/engines` | By **exact DCS type**: kills, losses, sorties, K/D |

### Vehicle granularity

In accordance with the plan, vehicles are aggregated by **exact DCS type**
(`F-16C_50`, `Su-27`, `T-72B`, `Mi-24P`…), without grouping by family. The type
is provided by the Lua hook which resolves the player's slot via
`Sim.getAvailableSlots` (the `slotID` alone is only an opaque identifier).

## Accepted limitations

- **4.3 Analytical maps** (heatmaps, tracks) and **4.5 Sortie analysis**
  (telemetry: altitude/speed/max G) require recording positions
  per unit and the ownship export. They will rely on the lat/lng positions already
  received as telemetry rather than on the debrief's internal coordinates, which
  will avoid any per-theatre projection. Coming in a dedicated increment.
- **Friendly-fire** is only counted from the events; the debrief does not
  always detail it.
- **Deaths** (`pilot_death`) are only attributed if the player has a statistics
  snapshot in the same mission.

## Tests

`internal/stats` covers: pilots (career and mission), weapons, vehicles, coalitions,
network, overview, and aircraft type resolution.
