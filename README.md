# DCS Mission Manager

🇬🇧 English | [🇫🇷 Français](README.fr.md)

An all-in-one manager for **DCS World**: real-time live map, debriefing reading, and advanced statistics. It runs **locally, on the same Windows machine as DCS**: one `dcsmm.exe`, no server, no container, nothing to configure.

Because it is local, it can read DCS's **own terrain data** — the airfields, frequencies and beacons of every installed map — instead of relying on a hand-maintained dataset.

---

## Table of contents

- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick start (PoC Phase 0)](#quick-start-poc-phase-0)
- [Configuration](#configuration)
- [Installing the Lua scripts into DCS](#installing-the-lua-scripts-into-dcs)
- [Deployment](#deployment)
- [Project structure](#project-structure)
- [Roadmap](#roadmap)
- [License](#license)

---

## Features

| Feature | Status | Details |
|---|---|---|
| Real-time live map | ✅ Phase 1 | All objects, categories, filters, trails, search |
| Basemaps | ✅ Phase 8 | Satellite, Relief, Road, **Aeronautical**, Dark — all key-free |
| Theatre & extent | ✅ Phase 8 | 12 DCS maps: framing, extent outline, per-theatre airfields |
| Authentic DCS tiles | 📋 Planned | F10 tile exporter (`tiles/` folder) |
| Events & players | ✅ Phase 2 | Kills, crashes, chat, players, SQLite history |
| Debriefings | ✅ Phase 3 | Network transfer of `debrief.log`, Lua parser, history |
| Server control | 🚧 Partial | Chat to DCS (command channel) coming |
| Advanced stats | ✅ Phase 4 | Pilots, weapons, engines, balance, network (career + mission) |
| Analytical maps & sortie | ✅ Phase 4 bis | Heatmaps, trails, sortie analysis, ownship telemetry |
| Aerodromes | ✅ Phase 6 | Read from DCS's own terrain files: 101 airfields across 5 maps, with Tower/TACAN/ILS/VOR/RSBN/NDB, shown on the map with a click-through data card |
| Fog of war | ✅ Phase 7 | Respects F10 mission options (server-side filtering) |

### Advanced statistics (planned)

- **Pilot & career profile** — kills/deaths/KD, ejections, crashes, flight time, by **UCID**
- **Weapon analysis** — effectiveness per weapon, kill matrix, friendly-fire
- **Analytical maps** — kill/death heatmaps, replayable flight trails
- **Balance & meta** — coalition balance, aircraft flown, mission timeline
- **Sortie analysis** — duration, distance, altitude/speed/G max (telemetry)
- **Network quality** — ping, disconnections, error codes
- **Analysis by engine** — exact **DCS type** granularity (`F-16C_50`, `T-72B`, `SA-10`…), platform + target + matchups

---

## Architecture

```
                      WINDOWS (one machine)
┌──────────────────────────────────────────────────────────┐
│ DCS World                                                 │
│  Scripts/Export.lua      → positions (UDP/JSON)           │
│  Scripts/Hooks/dcsmm.lua → events/players/chat (TCP/JSON) │
└──────────────┬───────────────────────────────────────────┘
               │ 127.0.0.1 — nothing to configure
               ▼
┌──────────────────────────────────────────────────────────┐
│ dcsmm.exe — Go backend + embedded Web UI                  │
│  • UDP + TCP listeners, in-memory state, SQLite (data/)   │
│  • debrief parser, statistics, airfield reference         │
│  • reads Mods/terrains/ and Saved Games/ directly         │
│  • REST + SSE on http://localhost:8080                    │
└──────────────────────────────────────────────────────────┘
```

### Local by design

```
   dcsmm.exe runs ON THE SAME WINDOWS MACHINE AS DCS
   → no LAN address to set, no firewall rule, no container
   → direct read access to Mods/terrains/ and Saved Games/
```

**Stack:** Go (backend, single binary + embedded UI) · Svelte + Vite + Leaflet (frontend) · SQLite (persistence).

---

## Prerequisites

### DCS side (machine A, Windows)

- DCS World (latest stable version or open beta)
- Access to `%USERPROFILE%\Saved Games\DCS\` (or `DCS.openbeta`)
- LuaSocket — bundled with DCS, no installation required

### Development / manager side

- **Go 1.22+** — <https://go.dev/dl/> (`winget install GoLang.Go`)
- **Node.js 20+** — <https://nodejs.org/> (only to build the frontend)
- **Git**

---

## Quick start (PoC Phase 0)

The PoC validates the whole chain: **DCS → UDP → Go → SSE → browser**.

### 1. Start the manager

```powershell
# Backend only (also serves a placeholder if the frontend is not built)
go run ./backend/cmd/dcsmm
```

By default, the backend listens on:

- `127.0.0.1:7778` over **UDP** (positions)
- `0.0.0.0:8080` over **HTTP** (Web UI + real-time stream `GET /api/events` via SSE)

Then open <http://localhost:8080>.

### 2. Install the Lua scripts into DCS

Copy the files from `dcs-lua/` into your Saved Games folder — see
[Installing the Lua scripts into DCS](#installing-the-lua-scripts-into-dcs). The PoC only needs
`Export.lua` and `Config/dcsmm.cfg`.

### 3. Launch DCS and a mission

Your aircraft appears as a point on the map, updated once per second.

---

## Configuration

All configuration is done through **environment variables** (prefix `DCSMM_`) with sensible
defaults. None of them is required for a normal install.

| Variable | Default | Description |
|---|---|---|
| `DCSMM_HTTP_ADDR` | `0.0.0.0:8080` | HTTP listening address (Web UI + SSE) |
| `DCSMM_UDP_ADDR` | `127.0.0.1:7778` | UDP listening address (Live map telemetry) |
| `DCSMM_TCP_ADDR` | `127.0.0.1:7779` | TCP listening address (events + commands) |
| `DCSMM_DB_PATH` | `./data/dcsmm.db` | SQLite database path |
| `DCSMM_DB_ENABLED` | `true` | Enable persistence (otherwise everything in memory) |
| `DCSMM_THEATRE` | `Caucasus` | Default theatre |
| `DCSMM_UNIT_TTL` | `5` (seconds) | Delay before a silent unit disappears |
| `DCSMM_TILES_DIR` | `./tiles` | DCS map tiles folder |
| `DCSMM_BASEMAP` | `satellite` | Default basemap: `satellite`, `topo`, `osm`, `dark` |
| `DCSMM_BASEMAP_URL` | *(empty)* | Optional custom basemap (template `{z}/{x}/{y}`) |
| `DCSMM_CATEGORIES` | `./categories.json` | Override for engine classification |
| `DCSMM_MAX_UNITS` | `5000` | Maximum number of tracked units |
| `DCSMM_TRACK_INTERVAL` | `3` (seconds) | Position sampling frequency |
| `DCSMM_TRACK_GRACE` | `15` (seconds) | Absence before a unit counts as lost |
| `DCSMM_TRACK_RETENTION` | `86400` (seconds) | History retention duration |
| `DCSMM_SAVED_GAMES` | *(auto)* | DCS Saved Games folder, when auto-detection fails |
| `DCSMM_CHARTS_DIR` | `./maps_dcs` | Aeronautical chart scans (approach plates, ground plans) |
| `DCSMM_REVEAL_ALL_UNITS` | `false` | Disables fog of war (broadcast everything; solo/design) |
| `DCSMM_SOURCE` | *(auto)* | Force the session source: `live` or `test` (see below) |
| `DCSMM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### DCS side — `Saved Games\DCS\Config\dcsmm.cfg`

```lua
-- Backend address (the manager runs locally)
dcsmm_host = "127.0.0.1"
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779

-- Live map
dcsmm_send_interval = 1.0    -- player position (seconds)
dcsmm_world_enabled = true   -- export all objects
dcsmm_world_interval = 2.0   -- object list (seconds)
dcsmm_world_radius = 0       -- radius filter in km (0 = all)
```

---

## Installing the Lua scripts into DCS

> ⚠️ **Always merge, never overwrite.** `Export.lua` is very often already modified by
> Tacview, SRS, DCS-BIOS, etc. Back up the existing file before any modification.

The scripts go into DCS's *Saved Games* folder:

```
%USERPROFILE%\Saved Games\DCS\          (or DCS.openbeta)
├─ Config\
│   └─ dcsmm.cfg          ← backend address configuration
└─ Scripts\
    ├─ Export.lua         ← positions (live map); to MERGE with the existing one
    └─ Hooks\
        └─ dcsmm.lua      ← events, players, chat (Phases 2+)
```

1. Copy `dcs-lua/Config/dcsmm.cfg` into `Saved Games\DCS\Config\`.
2. If `Saved Games\DCS\Scripts\Export.lua` already exists: make a copy of it
   (`Export.lua.bak-YYYYMMDD`), then add the `do ... end` block provided in
   `dcs-lua/Export.lua` **at the end** of the existing file.
3. Otherwise, simply copy `dcs-lua/Export.lua`.
4. Restart DCS.

---

## Deployment

### Windows `.exe`

```powershell
# 1. Build the frontend and the backend
.\build.ps1

# 2. Install the scripts on the DCS side (safe merge into Saved Games)
.\install-dcs.ps1            # add -DryRun to simulate

# 3. Run
.\dcsmm.exe
```

The UI opens at <http://localhost:8080>.

### CLI

```powershell
dcsmm                 # starts the manager (web interface + DCS receive)
dcsmm install-lua     # installs/merges the Lua scripts into Saved Games
dcsmm uninstall-lua   # removes the installed block (keeps the config)
dcsmm status          # installed / outdated / missing, per file
dcsmm purge           # deletes recorded sessions (destructive)
dcsmm version
```

### Test sessions and `purge`

The test tools (`tools/send-telemetry.mjs`, …) speak exactly the same protocol as
DCS, so a session recorded while they are running is indistinguishable from a real
flight. Every session is therefore tagged with a **source**:

- `live` — recorded from DCS. Statistics count these.
- `test` — recorded from the test tools. Kept on disk, but **excluded from
  statistics, analytics and the dashboard** unless explicitly requested.

Detection is automatic: the backend recognises the fixture callsigns used by
`tools/send-telemetry.mjs`. Set `DCSMM_SOURCE=test` (or `live`) to force the
verdict when you use your own fixtures.

```powershell
dcsmm purge --source test          # delete simulated sessions only
dcsmm purge --mission-id 3         # delete one mission and everything linked to it
dcsmm purge --all                  # delete every recorded session
dcsmm purge --source test --dry-run  # show what would be deleted, delete nothing
```

Statistics accept `?includeTest=1` to include simulated sessions deliberately.

> Upgrading from an older version tags **existing** missions as `live` (the
> migration cannot know they were simulated). To clear data recorded before this
> feature, use `dcsmm purge --mission-id <n>` or `--all`.

---

## Project structure

```
DCS mission manager/
├─ README.md                 # English (primary)
├─ README.fr.md              # French
├─ CHANGELOG.md / .fr.md
├─ VERSION
├─ Makefile / build.ps1       # build commands
├─ install-dcs.ps1           # installs the Lua scripts into Saved Games
├─ dcs-lua/                  # scripts to install on the DCS side
│   ├─ Config/dcsmm.cfg      # configuration template
│   ├─ Export.lua            # positions → UDP (live map)
│   └─ Hooks/dcsmm.lua       # events / players / chat
├─ backend/                  # Go
│   ├─ go.mod
│   ├─ cmd/dcsmm/main.go     # server + CLI (install/uninstall/status)
│   └─ internal/
│       ├─ config/           # env loading + defaults
│       ├─ install/          # Lua injector (merge via markers)
│       ├─ aerodrome/        # aerodromes and frequencies (embedded data)
│       ├─ category/         # engine classification (DCS type → family)
│       ├─ theatre/          # DCS theatres and their extents
│       ├─ basemap/          # basemaps (satellite, relief, osm, dark)
│       ├─ model/            # types exchanged DCS ↔ backend
│       ├─ lua/              # Lua data parser (debrief.log)
│       ├─ debrief/          # debrief analysis
│       ├─ debriefstore/     # reassembly of debrief transfers
│       ├─ udp/              # position receiver (live map)
│       ├─ tcp/              # event / player / chat receiver
│       ├─ live/             # in-memory session state
│       ├─ ingest/           # live → database bridge
│       ├─ tracker/          # position history + loss detection
│       ├─ db/               # SQLite persistence (pure Go)
│       ├─ state/            # unit store (in memory)
│       ├─ stats/            # statistical aggregations
│       └─ api/              # REST + SSE + tiles + embedded UI (dist/)
├─ frontend/                 # Svelte + Vite + Leaflet
│   └─ src/
│       ├─ App.svelte
│       └─ lib/              # map, panels, stores
├─ tiles/                    # DCS tiles per theatre
├─ tools/                    # test telemetry emitter, tile extractor
└─ docs/                     # documentation
```

---

## Roadmap

- [x] **Phase 0 — PoC**: `Export.lua` (player position) → Go → Leaflet map
- [x] **Phase 1 — Live map**: all objects, categories, filters, trails, search, DCS tiles
- [x] **Phase 2 — Events & players**: `onGameEvent`, chat, `net.get_stat`, SQLite history
- [x] **Phase 3 — Debriefings**: network transfer of `debrief.log`, Lua parser, history
- [x] **Phase 4 — Advanced stats**: overview, pilots, weapons, engines, balance, network
- [x] **Phase 4 bis — Analytical maps & sortie**: heatmaps, trails, telemetry
- [x] **Phase 5 — Packaging**: CLI, safe Lua injector, single self-contained binary
- [x] **Phase 6 — Aerodromes**: 21 Caucasus terrains (frequencies, charts)
- [x] **Phase 7 — Fog of war**: respects the mission's F10 options (server filtering)

The full, detailed plan is available in the project's plan file.

---

## License

[MIT](LICENSE) © 2026 Laurent (lolo1580)

This is a community project, not affiliated with or endorsed by Eagle Dynamics.
"DCS World" and its terrains are trademarks of Eagle Dynamics SA.
