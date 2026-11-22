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
| `internal/checklist` | Checklist model and validation |
| `internal/idgen` | Short random identifiers for stored documents |
| `internal/auth` | Password hashing (Argon2id), session and API token primitives |
| `internal/syncer` | Optional desktop → server mission and checklist sync |

## Frontend

Svelte + Vite. The build is written to `backend/internal/api/dist/` (not
versioned), then embedded into the Go binary via `//go:embed`. Result: **a single
executable** contains the backend and the interface. If the frontend has not been
built, a fallback page is served automatically.

The UI is organised in addressable pages: **Overview**, **Flights** (missions
and debriefs), **Career**, **Statistics**, **Checklists**, **Mods & skins**,
**Airfields** (reference data, frequencies and charts) and **Settings**, which
groups the installed modules, the DCS-side install (scripts and mods), the
profile backup and the debug log as sub-tabs. A small history router uses
`history.pushState`; the Go handler already falls back to `index.html`, so a
reload on `/checklists` works. The server profile shows **Squadron**, **Flights**,
**Statistics**, **Checklists**, **Airfields**, **My profile**
and, for administrators, **Administration**.
There is no live map and no analysis view any more. Unit telemetry is still
received and sampled, and current position feeds the airfields tab.

## Server mode

The same binary exposes a second profile, `dcsmanager server` (or
`DCSMANAGER_MODE=server`). It opens the same SQLite schema but:

- never opens the UDP/TCP telemetry ports and never reads Saved Games;
- requires a session for every API call except health, login and the session
  probe; desktop-only routes return 404;
- boots the first administrator from `DCSMANAGER_ADMIN_USER` and
  `DCSMANAGER_ADMIN_PASSWORD` when the users table is empty;
- stores Argon2id password hashes, sessions and personal API tokens, and
  records account changes in an audit log;
- can send **single-use password-reset links** when SMTP is configured
  (`DCSMANAGER_SMTP_*`); without SMTP, an administrator resets passwords;
- exposes an administrator **database backup** (`VACUUM INTO` + zip) and a
  **conflict merge** screen (restore the losing version or choose fields);
- accepts `POST /api/sync/upload` from a desktop token, upserting missions by
  `(owner, client, desktop id)` so a retry does not duplicate data.

The desktop uploads finished live missions from **Settings → Server sync**. The
server serves the shared flights, statistics, plans and checklists through the
same Svelte application, with a server-specific navigation and login page.
Plans and checklists carry an owner, a private/squadron visibility and a
server-wide revision. The desktop pulls changes, then pushes the documents
changed locally (tracked by a per-document dirty flag), using last-write-wins
for concurrent edits and tombstones for deletions. The losing version is kept in
`conflict_copies` and shown under **Administration**.
`Dockerfile` and `docker-compose.yml` package the profile for Linux; see
[docs/docker.md](docker.md).

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
