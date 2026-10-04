# Architecture — DCS Manager

## Principle

The manager is a **local companion to DCS World**: it runs on the same Windows
machine as the simulator. That is a deliberate choice, and it pays off twice: the
backend can read DCS's **own terrain data** (airfields, frequencies, beacons of
every installed map), and it reads the **Saved Games** folder directly, so the
Lua installer, the debrief and the track files need no transfer or setup.

1. **The Lua scripts** — collect and send the data.
2. **The manager** (Go backend + Web UI) — receives, stores, aggregates and displays.

## Installing

```powershell
# 1. Build the frontend and the backend
.\build.ps1

# 2. Install the scripts (safe merge into Saved Games)
.\install-dcs.ps1            # add -DryRun to simulate

# 3. Run
.\dcsmanager.exe
```

The manager opens in its own window. `dcsmanager serve` runs it headless instead, with the
web UI at <http://localhost:8080>.

### CLI

```powershell
dcsmanager                 # opens the manager in a native window
dcsmanager serve           # starts the server only; UI at http://localhost:8080
dcsmanager install-lua     # installs/merges the Lua scripts into Saved Games
dcsmanager uninstall-lua   # removes the installed block (keeps the config)
dcsmanager status          # installed / outdated / missing, per file
dcsmanager purge           # deletes recorded sessions (destructive; see README)
dcsmanager version
```

## Data flow

```
                        WINDOWS (one machine)
┌──────────────────────────────────────────────────────────┐
│ DCS World                                                 │
│                                                           │
│  Scripts/Export.lua        Hooks/dcsmanager.lua                │
│   (telemetry)              (events/players)               │
│        │                          │                       │
│        │ UDP/JSON                 │ TCP/JSON              │
└────────┼──────────────────────────┼───────────────────────┘
         │     127.0.0.1            │
         ▼                          ▼
┌──────────────────────────────────────────────────────────┐
│ Manager (dcsmanager.exe)                                       │
│                                                           │
│  internal/udp  ──► internal/state ──► internal/tracker     │
│  internal/tcp  ──► internal/live  ──► internal/ingest      │
│                                        ├─ SQLite (data/)   │
│                                        └─ internal/api     │
│                                            ├─ REST /api/*  │
│                                            ├─ SSE /events  │
│                                            └─ Embedded UI  │
│                                                           │
│  Mods/terrains/  ──► internal/aerodrome (airfields)       │
│  Saved Games/    ──► internal/install, debrief, tracks    │
│  internal/debrief (Phase 3)                               │
│  internal/stats   (Phase 4)                               │
└──────────────────────────────────────────────────────────┘
```

## Backend components

| Package | Role |
|---|---|
| `internal/config` | Configuration via `DCSMANAGER_*` variables |
| `internal/app` | Manager wiring (listeners, state, DB, tracking, HTTP), shared by both entry points |
| `internal/desktop` | Native window (WebView2, pure Go); falls back to headless if unavailable |
| `internal/udp` | Reception and decoding of telemetry datagrams |
| `internal/state` | In-memory store of units, with expiry (TTL) |
| `internal/api` | REST, Server-Sent Events, serving of the embedded UI |
| `internal/aerodrome` | Airfields and frequencies, read from DCS's own terrain files |
| `internal/charts` | Aeronautical chart scans, indexed from `maps_dcs/` |
| `internal/debrief` | `debrief.log` parser (Phase 3) |
| `internal/db` | SQLite persistence (pure Go) |
| `internal/tracker` | Position history and loss detection |
| `internal/stats` | Statistical aggregations (Phase 4) |

## Frontend

Svelte + Vite. The build is written to `backend/internal/api/dist/` (not
versioned), then embedded into the Go binary via `//go:embed`. Result: **a single
executable** contains the backend and the interface. If the frontend has not been
built, a fallback page is served automatically.

The UI is organised in tabs: **Debriefs**, **Missions**, **Career & statistics**,
**Airfields** (reference data, frequencies and charts) and **Settings**, which groups
the installed modules, the DCS-side install (scripts and mods) and the cockpit panels
as sub-tabs. There is no live map, no analysis view and no DCS-configuration view any
more; the unit telemetry is still received and sampled, and it feeds the statistics
and the airfields tab.

## Application window

`dcsmanager.exe` opens the embedded UI in a native window rather than asking the user to
open a browser. That window is a WebView2 control (the runtime Microsoft ships with
Windows 10/11), driven through `github.com/jchv/go-webview2` — a **pure-Go** binding,
so the build needs no C toolchain and `CGO_ENABLED=0` still holds.

The HTTP server is not removed: the window loads `http://<addr>/`, so the page reaches
the API exactly as before, and `dcsmanager serve` still exposes it to a browser. The window
is therefore an addition, not a replacement, and closing it shuts the manager down.

## Real-time choice: SSE rather than WebSocket

Updates are **downstream only** (server → client).
**Server-Sent Events** is enough, integrates natively with the browser (`EventSource`),
and avoids the complexity of a WebSocket. A bidirectional WebSocket could be added
for the command channel (kick, mission change…), but that channel will go through the
dedicated TCP socket on the Lua side, not through the browser.

## Invariants

- **Everything runs locally**, on the same machine as DCS. The Lua scripts talk
  to `127.0.0.1`; there is no remote mode.
- The Lua export is **throttled** (`LuaExportActivityNextEvent`): a simulator
  frame is never blocked.
- The provided Lua scripts are **additive**: never overwrite an existing
  `Export.lua` (Tacview, SRS, DCS-BIOS…).
- Reads from the DCS installation are **read-only** and never execute Lua.
