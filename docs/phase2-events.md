# Events, players and chat (Phase 2)

## Principle

In addition to the **UDP** channel (positions, Phase 1), the backend listens on a
**TCP** channel on which the Lua hooks push **JSON messages, one line per
message**.

```
DCS (Windows)                                    Backend
Scripts/Hooks/dcsmanager.lua
  onGameEvent ─┐
  onChatMessage ┼─► TCP 7779, JSON one line per message ─► internal/tcp
  net.get_*   ─┘                                           │
                                                           ├─► internal/live   (in-memory, real-time UI)
                                                           └─► internal/ingest ─► internal/db (SQLite)
```

The choice of **line-by-line JSON** (NDJSON) rather than a binary frame is
deliberate: it is debuggable by eye, the parser fits in a few lines, and the
protocol can evolve without breaking older clients.

## Messages

| `type` | Content | Emitted by |
|---|---|---|
| `mission` | `phase` (`start`/`end`), `name`, `theatre`, `winner` | `onSimulationStart` / `onSimulationStop` |
| `event` | `event`, `args[]`, `t` | `onGameEvent` |
| `players` | `players[]` (id, UCID, name, side, slot, stats) | `net.get_player_list` + `net.get_stat` |
| `chat` | `from`, `message` | `onChatMessage` |

### Captured events

`kill`, `friendly_fire`, `mission_end`, `self_kill`, `change_slot`, `connect`,
`disconnect`, `crash`, `eject`, `takeoff`, `landing`, `pilot_death`.

### Per-player statistics

Exactly those of the `net.get_stat` API: ping, crashes, vehicle / aircraft /
ship kills, score, landings, ejections. Plus the **UCID**, essential for career
statistics that survive callsign changes.

## Refresh

- **Events** and **chat**: sent as soon as they occur.
- **Players**: refreshed every `dcsmanager_players_interval` seconds (default 5), and
  immediately on a `change_slot`, `connect`, `disconnect` or `mission_end`.
- The periodic refresh goes through `onSimulationFrame`, so it never blocks a
  frame: no blocking network call is made in a callback.

## Robustness on the Lua side

- **Persistent** TCP connection with automatic reconnection if the backend
  restarts or is absent.
- `settimeout(0.5)`: an unreachable backend does not freeze the simulator.
- Each `net.*`/`Sim.*` call is protected by `pcall`.
- `tcp-nodelay` enabled for immediate sending of events.

## Web API

| Route | Description |
|---|---|
| `GET /api/game-events` | Recent events (in memory) |
| `GET /api/players` | Connected players |
| `GET /api/chat` | Recent chat; `POST` reserved for sending to DCS (coming soon) |
| `GET /api/mission` | Current mission |
| `GET /api/history/events` | Persisted events (SQLite). `?sinceId=N` switches to incremental mode: only events with `id > N`, oldest first, plus `nextSinceId` |
| `GET /api/history/chat` | Persisted chat. `?sinceId=N` works the same way |
| `GET /api/history/missions` | Past missions. `?sinceId=N` works the same way |

The `?sinceId=` form exists so an **external consumer** (the statistics plugin, or
any script) can mirror the whole history without gaps or duplicates: it stores the
largest id it has seen and asks for everything after it. Without `sinceId`, the
endpoints keep their original meaning — the most recent rows, newest first. A
non-numeric `sinceId` is treated as `0` (the whole history) rather than an error.

The same data arrives in real time via **SSE** (`/api/events`) as a
`{"type":"session", ...}` frame emitted every second.

## Persistence

SQLite via **modernc.org/sqlite** (pure Go, no CGO): the binary stays single and
needs no C toolchain.

Tables: `missions`, `events`, `chat`, `players`, `player_stats`, `meta`.
Persistence can be disabled with `DCSMANAGER_DB_ENABLED=false` (everything stays in
memory).

## Command channel

The TCP channel is **bidirectional**: the backend pushes commands down the same
connection the hook uses to report, as one JSON line per command. The hook reads
them from `onSimulationFrame` (non-blocking, so a frame is never stalled) and
executes them.

```
Backend                                   DCS (Windows)
POST /api/chat ─► internal/tcp.SendCommand ─► Hooks/dcsmanager.lua readCommands()
                                                 └─► net.send_chat("...")
```

| Command | Effect |
|---|---|
| `{"type":"command","command":"chat","message":"...","from":"Server"}` | Injects a chat message into DCS via `net.send_chat` |

`POST /api/chat` returns `200` once the command is written, `503` when no hook is
connected (game not running, or scripts not installed) — a normal state, shown as
such in the UI, not an error.

The hook ignores an unknown command and logs it, so a newer backend cannot break
an older hook.
