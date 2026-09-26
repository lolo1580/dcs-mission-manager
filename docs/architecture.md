# Architecture — DCS Mission Manager

## Principle

DCS World cannot run in Docker (Windows only, GPU + rendering required).
The project is therefore designed as **two components** that communicate over the network:

1. **The Lua scripts on the DCS side** (Windows machine) — collect and send the data.
2. **The manager** (Go backend + Web UI) — receives, stores, aggregates and displays.

The same manager deploys as a Windows `.exe` or a Linux Docker image, **with no
changes to the code**.

## Data flow

```
                          WINDOWS (machine A)
┌──────────────────────────────────────────────────────────┐
│ DCS World                                                 │
│                                                           │
│  Scripts/Export.lua        Hooks/dcsmm.lua                │
│   (live map)               (events/players)           │
│        │                          │                       │
│        │ UDP/JSON                 │ TCP/JSON              │
└────────┼──────────────────────────┼───────────────────────┘
         │                          │
         ▼                          ▼
┌──────────────────────────────────────────────────────────┐
│ Manager (machine A en .exe, ou machine B en Docker)       │
│                                                           │
│  internal/udp  ──► internal/state ──► internal/api        │
│                                        ├─ REST /api/*      │
│                                        ├─ SSE  /api/events │
│                                        └─ Embedded web UI │
│                                                           │
│  internal/debrief (Phase 3)                               │
│  internal/stats   (Phase 4)                               │
│  internal/theatre (Phase 1, projection)                   │
└──────────────────────────────────────────────────────────┘
```

## Backend components

| Package | Role |
|---|---|
| `internal/config` | Configuration via `DCSMM_*` variables |
| `internal/udp` | Reception and decoding of telemetry datagrams |
| `internal/state` | In-memory store of units, with expiry (TTL) |
| `internal/api` | REST, Server-Sent Events, serving of the embedded UI |
| `internal/theatre` | `lat/lng ↔ DCS coordinates` projection (Phase 1) |
| `internal/debrief` | `debrief.log` parser (Phase 3) |
| `internal/stats` | Statistical aggregations (Phase 4) |

## Frontend

Svelte + Vite + Leaflet. The build is written to
`backend/internal/api/dist/` (not versioned), then embedded into the Go binary via
`//go:embed`. Result: **a single executable** contains the backend and the interface.
If the frontend has not been built, a fallback page is served automatically.

## Real-time choice: SSE rather than WebSocket

For Phase 0/1, updates are **downstream only** (server → client).
**Server-Sent Events** is enough, integrates natively with the browser (`EventSource`),
and avoids the complexity of a WebSocket. A bidirectional WebSocket could be added
in Phase 2 for the command channel (kick, mission change…), but that channel
will go through the dedicated TCP socket on the Lua side, not through the browser.

## Invariants

- DCS stays on Windows; the container only contains the manager.
- In Docker on another machine, the Lua scripts target the container's **LAN IP**,
  never `127.0.0.1` (which would loop back to the Windows host).
- The Lua export is **throttled** (`LuaExportActivityNextEvent`): a simulator
  frame is never blocked.
- The provided Lua scripts are **additive**: never overwrite an existing
  `Export.lua` (Tacview, SRS, DCS-BIOS…).
