# Events, players and chat (Phase 2)

## Principle

In addition to the **UDP** channel (positions, Phase 1), the backend listens on a
**TCP** channel on which the Lua hooks push **JSON messages, one line per
message**.

```
DCS (Windows)                                    Backend
Scripts/Hooks/dcsmm.lua
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
- **Players**: refreshed every `dcsmm_players_interval` seconds (default 5), and
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
| `GET /api/history/events` | Persisted events (SQLite) |
| `GET /api/history/chat` | Persisted chat |
| `GET /api/history/missions` | Past missions |

The same data arrives in real time via **SSE** (`/api/events`) as a
`{"type":"session", ...}` frame emitted every second.

## Persistence

SQLite via **modernc.org/sqlite** (pure Go, no CGO): the binary stays single and
needs no C toolchain.

Tables: `missions`, `events`, `chat`, `players`, `player_stats`, `meta`.
Persistence can be disabled with `DCSMM_DB_ENABLED=false` (everything stays in
memory).

## Command channel (coming soon)

The TCP channel is **bidirectional by construction**: the backend will be able to
push commands (kick, mission change, chat message) that the Lua hook will
execute. The `POST /api/chat` endpoint already exists and returns `501`
explicitly as long as this channel is not wired up.
