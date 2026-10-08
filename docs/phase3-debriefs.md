# Debriefs (Phase 3)

## The `debrief.log` file

At the end of each mission, DCS writes
`%USERPROFILE%\Saved Games\DCS\Logs\debrief.log`. It is a **Lua table dump**,
not JSON. It notably contains:

| Key | Content |
|---|---|
| `mission_file_path` | Path of the `.miz` played |
| `mission_time` | Mission duration (seconds) |
| `result` | Result |
| `world_state` | **Final state** of all units (type, coalition, x/y, alt, dead or not) |
| `events` | **Ordered timeline**: `mission start`, `takeoff`, `land`, `engine shutdown`, `mission end`, and depending on the mission `kill`, `crash`, `eject`, `pilot dead`… |

Example event:

```lua
[4] =
{
    type = "takeoff",
    initiatorPilotName = "Cellar",
    place = "Mineralnye Vody",
    t = 43.42,
    initiator_unit_type = "M-2000C",
    event_id = 36,
    initiator_coalition = 2,
    initiatorMissionID = "25",
},
```

### Important clarification

DCS writes this file **at the end of the mission**, and it is **overwritten** on each
new mission. That is why the Lua hook captures it and sends it
immediately: otherwise the history would be lost.

## Parsing

`debrief.log` is not JSON, so rather than a fragile regular expression,
the backend embeds a **minimal Lua parser** (`internal/lua`): it reads the
`name = value` assignments and the nested tables, **without executing any Lua**.
It is safe (no code evaluation) and sufficient for this format.

The `internal/debrief` package then turns the raw table into a typed
structure (events, world state, aggregates) via `ToModel()`.

## Network transport

The file can exceed 1 MB. It is sent in **chunks**:

```
Hooks/dcsmanager.lua                      Backend
  lit debrief.log
  → splits into 32 KB chunks
  → base64 par morceau
  → {"type":"debrief", transferId, chunk, chunks, size, data}
                                    → internal/debriefstore reassembles
                                    → internal/debrief parse
                                    → SQLite (internal/db)
```

- **base64**: guarantees that any byte crosses the NDJSON channel intact.
- **`transferId`**: several transfers can be in progress without getting mixed up.
- **Reassembly in chunk order**, even if they arrive out of order.
- An invalid chunk is logged and ignored: it never makes the backend crash.

## Storage

Table `debriefs`: metadata + `parsed` (structured JSON) + `raw` (original
text, to re-parse later if the format evolves).

## Web API

| Route | Description |
|---|---|
| `GET /api/debriefs` | List of debriefs (metadata) |
| `GET /api/debriefs/{id}` | Full debrief (structured) |
| `GET /api/debriefs/{id}?raw=1` | Adds the original text |

## Interface

The **Debriefs** tab is currently hidden. Its retained view shows the list on the
left and, for the selected debrief:

- counters (takeoffs, landings, kills, crashes, ejections, duration);
- the list of pilots;
- the minute-by-minute **timeline**, colored by event type.

## Test without DCS

```bash
node tools/send-debrief.mjs
# ou avec un fichier et une cible explicites :
node tools/send-debrief.mjs "%USERPROFILE%\Saved Games\DCS\Logs\debrief.log" 127.0.0.1 7779
```

## Known limitations

- The `debrief.log` does not always contain the kills (it depends on the mission
  and the version); Phase 2 real-time events fill this gap,
  which justifies the **merging of the two sources** planned for the statistics.
- The file is written only at the end of the mission: no “in progress” debrief.
