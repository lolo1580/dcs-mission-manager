# Changelog

🇬🇧 English | [🇫🇷 Français](CHANGELOG.fr.md)

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [semantic versioning](https://semver.org/).

## [Unreleased]

### Upcoming

- Other features inspired by MizMap / MovingMap: BRA measurement, SAM circles,
  MIL-STD-2525C symbols, kneeboards viewer

## [1.0.0] — 2026-09-26

**Bilingual release.** The whole project is now in English, with French kept as a
first-class option.

### Added

- **Interface internationalisation (`frontend/src/lib/i18n.js`)**
  - Complete EN/FR dictionaries (186 keys each, verified in sync).
  - **English by default**; the language switcher is in the header, the choice is
    persisted in `localStorage` and applied to `<html lang>`.
  - Reactive `$t()` translator in markup, `tNow()` in scripts.
  - Plurals handled explicitly, since French and English do not agree the same way.

- **Bilingual documentation**
  - `README.md` / `CHANGELOG.md` in English (primary) with `README.fr.md` /
    `CHANGELOG.fr.md` preserved, plus cross-language links at the top.

### Changed

- Every French string, comment, log line, CLI message, docstring and documentation
  page has been translated to English: backend (all packages), Lua scripts, tools,
  CI workflow, Docker files, PowerShell scripts, examples.
- **Protocol values were deliberately left untouched** (`dcsmm_*` config keys,
  JSON keys, DCS event names, coalition values, `optview_*`, theatre ids, basemap
  ids), so existing installations keep working.
- The `DCSMM-BEGIN`/`DCSMM-END` markers were translated **on both sides**
  (`install.go` and the Lua scripts) so they still match byte-for-byte.
- The fog-of-war banner is now translated client-side from the machine-readable
  `mode`, instead of displaying the backend's label.

### Fixed

- **Encoding corruption** in `docs/phase2-events.md`: the translation had turned
  every letter `i` into `n` (`internal` -> `nnternal`, `with` -> `wnth`). The page
  was rewritten and a scan confirmed no other file was affected.
- `frontend/index.html` declared `lang="fr"` while the app now defaults to English.
- CI and Docker comments were still French.

## [0.9.0] — 2026-09-26

**Phase 7 — Fog of war.** The manager now respects the mission's visibility
options: it no longer reveals what DCS hides.

### Added

- **`internal/visibility`**
  - Visibility policy based on the mission's `optionsView`, **restrictive
    by default**: never show more than DCS.
  - **Server-side** filtering (the client never receives hidden units).
  - The player's aircraft always remains visible; without it the map would be
    unusable.
  - "Fog of war" mode (`optview_allies`) treated as "allies only":
    sensor contacts are not reproduced, as a safety choice.

- **Backend**
  - The Lua hook transmits `Sim.getMissionOptions()` at mission start.
  - `internal/tcp` reports the options back; the server deduces the policy.
  - `GET /api/visibility` and `visibility` key in every state frame;
    `visibility` SSE event on every change.
  - `internal/config`: `DCSMM_REVEAL_ALL_UNITS` (default `false`).

- **Frontend**
  - Banner under the header indicating the active mode and its limit.

- **Documentation & tests**
  - `docs/fog-of-war.md` (official DCS values, principle, limits).
  - `internal/visibility` tests: value matching, filtering by mode,
    **no leakage of enemy/neutral units**, exemption, absence of aircraft.

### Notes

- The official values come from DCS's `MissionEditor/modules/Options/optionsDb.lua`.
  Along the way, two labels had initially been misinterpreted:
  `optview_allies` is "FOG OF WAR", `optview_onlyallies` is "ALLIES ONLY".

## [0.8.0] — 2026-09-26

**Phase 6 — Aerodromes.** Reference data for Caucasus terrains: coordinates,
radio frequencies and approach charts.

### Added

- **Aerodrome data (`internal/aerodrome`)**
  - 21 Caucasus terrains extracted from the **provided approach charts**:
    coordinates, elevation, runway, **Tower**, **TACAN**, **ILS** per runway, and
    references to the VAD/GND charts.
  - The data is **embedded** (`//go:embed data/*.json`); the format is
    generic, adding a theatre = dropping in a JSON file.
  - Since DCS does not expose frequencies at runtime, this reference is the only
    reliable source.

- **API**
  - `GET /api/aerodromes` (`?theatre=` filter, sort by distance with `?lat=&lng=`)
    and `GET /api/aerodromes/{code}`.

- **Frontend**
  - **Aerodromes** tab: search by name/ICAO code/TACAN, "Near me" button,
    detailed card (Tower, TACAN, ILS, available charts).
  - **"On the map"** checkbox: terrains are displayed as markers, frequencies
    in a tooltip.

- **Documentation & tests**
  - `docs/phase6-aerodromes.md` (source, scope, model, limits).
  - `internal/aerodrome` tests: loading, known terrain, completeness, sorting,
    filtering by theatre.

## [0.7.0] — 2026-09-26

**Phase 5 — Packaging.** CLI, safe Lua injector, `.exe` / Docker deployment.

### Added

- **CLI (`dcsmm`)**
  - `install-lua`: installs/merges the scripts into Saved Games;
  - `uninstall-lua`: removes the block and `Hooks/dcsmm.lua`;
  - `status`: `installed` / `outdated` / `missing` per file;
  - `version` / `help`.
  - Automatic detection of `Saved Games` (`DCS.openbeta` takes priority) and of the
    `dcs-lua` folder; `--saved-games`, `--lua-dir`, `--dry-run`.

- **Lua injector (`internal/install`)**
  - **Never replaces** an existing `Export.lua` (Tacview, SRS, DCS-BIOS…):
    merges a block delimited by `>>> DCSMM-BEGIN >>>` / `<<< DCSMM-END <<<`.
  - **Timestamped backup** before any modification.
  - **Idempotent**: a second run updates the block in place.
  - Markers added in `Export.lua` and `Hooks/dcsmm.lua`.

- **Deployment**
  - `Dockerfile`: `go.sum` copied (reproducible build), version injected via
    `-ldflags`, `ca-certificates`, non-root user, `/data` volume.
  - `docker-compose.yml`: published UDP/TCP ports, configured retention.
  - `Makefile`: `install` and `docker-multiarch` targets; `build.ps1` injects the
    version from `VERSION`.
  - `install-dcs.ps1`: one-click Windows installer with `-DryRun`.

- **Documentation & tests**
  - `docs/deployment.md`: the two modes, the LAN IP pitfall in Docker, ports.
  - `VERSION` (0.7.0).
  - `internal/install` tests: creation, **merge preserving third-party content**,
    idempotence, in-place replacement, backup, `--dry-run`, uninstallation,
    status (missing/installed/outdated).

### Fixed

- `Export.lua` called `toJson()`, which was never defined: the player send failed
  silently. The JSON payload is now built directly, with the
  telemetry fields added conditionally.

## [0.6.0] — 2026-09-26

**Phase 4 bis — Analytical maps & sortie analysis.** Position history,
heatmaps, flight trails and telemetry.

### Added

- **Backend**
  - `internal/tracker`: periodic position sampling (trails) and
    **loss detection** by prolonged disappearance, with its last position.
    Configurable retention and hourly purge. Creates a session mission if none
    has been announced.
  - `track_positions` and `losses` tables.
  - `internal/db`: `SaveSamples` (in batches), `SaveLoss`, `Heatmap` (aggregation into
    a degree grid, **without projection**), `Trails`, `PruneTracking`.
  - `internal/tracker.Analyse`: duration, distance (haversine), altitude/speed/G max.
  - Routes `GET /api/analytics/{heatmap,tracks,sorties}`.
  - Configuration: `DCSMM_TRACK_INTERVAL`, `DCSMM_TRACK_GRACE`,
    `DCSMM_TRACK_RETENTION`.
  - Telemetry fields (`speed`, `g`, `aoa`) on tracked units.

- **DCS scripts**
  - `Export.lua`: conditional ownship telemetry (true/indicated speed, Mach,
    angle of attack, ground altitude, load factor) if the server allows it.

- **Frontend**
  - **Analysis** tab: heatmap source (Traffic / Losses) and sortie
    analysis table per unit.
  - **History** button on the map: overlays the heatmap (cold→warm
    gradient) and the recorded trails.

- **Documentation & tests**
  - `docs/phase4bis-analytics.md`.
  - `internal/tracker` tests: sampling, loss, non-duplication,
    reappearance, haversine, sortie analysis.

## [0.5.0] — 2026-09-26

**Phase 4 — Advanced statistics.** Seven analysis modules, at **career** scope
(all missions, by UCID) or **mission** scope.

### Added

- **Backend**
  - `internal/stats`: overview, pilots (score, kills, K/D, FF, ping),
    weapons (kills, friendly-fire, targets and platforms), engines (**exact DCS type**:
    kills, losses, sorties, K/D), coalitions, network (average/max ping).
  - Joining events to players via `dcs_player_id`, which makes it possible to
    derive **deaths** and **friendly-fire** per pilot (absent from
    `net.get_stat`).
  - `unit_type` column in `player_stats`.
  - Routes `GET /api/stats/{overview,pilots,weapons,engines,network}` with
    `?scope=career|mission` and `?missionId=N`.

- **DCS scripts**
  - `Hooks/dcsmm.lua`: resolution of the **aircraft type** per player via
    `Sim.getAvailableSlots` (cached, refreshed every 30 s); the field
    `unitType` replaces the use of the opaque `slotID`.

- **Frontend**
  - **Statistics** tab: Career/Mission selector, summary cards, and
    five sub-views (Pilots, Weapons, Engines, Balance, Network).
  - Pilot ranking with medals, K/D, friendly-fire and ping.
  - Filters by engine category; comparative coalition bars.

- **Documentation**
  - `docs/phase4-stats.md`.
  - `internal/stats` tests (scopes, weapons, engines, coalitions, network, overview).

### Notes

- The **4.3 Analytical maps** (heatmaps, trails) and **4.5 Sortie analysis**
  (telemetry) modules are postponed to a dedicated increment: they will rely on
  the live map's lat/lng positions, without per-theatre projection.

## [0.4.0] — 2026-09-26

**Phase 3 — Debriefings.** At the end of each mission, `debrief.log` is sent to the
backend, parsed and archived.

### Added

- **Lua parsing (`internal/lua`)**
  - Parser for the subset of Lua used by DCS data files
    (`name = value`, nested tables, strings, comments). **Does not execute
    Lua**: data reading only.

- **Debrief analysis (`internal/debrief`)**
  - Typed extraction: path of the `.miz`, duration, `result`, final state of the world and
    **event timeline** (`takeoff`, `land`, `engine shutdown`,
    `mission end`, `kill`, `crash`, `eject`…).
  - Aggregates (`Summarise`): takeoffs, landings, kills, crashes, ejections,
    pilots, breakdown by event type.

- **Transport and storage**
  - `internal/debriefstore`: reassembly of **multi-chunk** transfers
    (base64 chunks, out-of-order arrival tolerated, concurrent transfers isolated).
  - `Hooks/dcsmm.lua`: reads `debrief.log` at the end of the mission and sends it in 32 KB
    chunks encoded in base64.
  - `debriefs` table (metadata, parsed `parsed` JSON, original `raw`).

- **API**
  - `GET /api/debriefs`, `GET /api/debriefs/{id}` and `?raw=1` for the raw text.

- **Frontend**
  - **Debriefs** tab: list, counters (takeoffs, landings, kills,
    crashes, ejections, duration), pilots and timeline colored by type.

- **Tools & documentation**
  - `tools/send-debrief.mjs`: sends a `debrief.log` to the backend.
  - `docs/phase3-debriefs.md`.
  - Tests: Lua parser (including a real `debrief.log`), analysis, multi-chunk
    reassembly, chunk order, concurrent transfers, invalid base64.

## [0.3.0] — 2026-09-26

**Phase 2 — Events & players.** The backend receives and records the game
events, connected players and chat.

### Added

- **Backend**
  - `internal/model`: exchanged types (messages, players, events, chat, missions).
  - `internal/tcp`: TCP receiver in **line-by-line JSON** format (NDJSON),
    with per-connection reconnection and logging.
  - `internal/live`: in-memory session state (events, players sorted by side,
    chat, current mission) with caps.
  - `internal/db`: **pure Go SQLite persistence** (`modernc.org/sqlite`, no CGO),
    `missions`, `events`, `chat`, `players`, `player_stats`, `meta` tables.
  - `internal/ingest`: bridge between live and the database; opens the current mission,
    resolves players by **UCID** (a career that survives renames) and
    records stats only for active players.
  - `internal/api`: `GET /api/game-events`, `/api/players`, `/api/chat`,
    `/api/mission`, `/api/history/{events,chat,missions}`; SSE `session` frame.
  - `internal/config`: `DCSMM_DB_ENABLED`.

- **DCS scripts**
  - `Hooks/dcsmm.lua`: TCP sending of events (`onGameEvent`), chat
    (`onChatMessage`), players and their statistics (`net.get_player_list`,
    `net.get_player_info`, `net.get_stat`), and mission transitions. Persistent
    connection, automatic reconnection, calls protected by `pcall`, never blocking.
  - `Config/dcsmm.cfg`: `dcsmm_players_interval`.

- **Frontend**
  - **Map** / **Session** tabs; name of the current mission in the header.
  - **Players** panel: name, score, air/ground/naval kills, landings, ping
    (highlighted above 250 ms), color coding by side.
  - **Events** panel: filters by type with counters, timestamp and
    readable description of the DCS arguments.
  - **Chat** panel: history and input area (sending to DCS coming).

- **Tools & documentation**
  - `tools/send-events.mjs`: TCP channel simulator (mission, players, events, chat).
  - `docs/phase2-events.md`.
  - Tests: `internal/live` (caps, sorting, mission) and `internal/db` (missions,
    events, chat, UCID, stats).

## [0.2.0] — 2026-09-25

**Phase 1 — Complete live map.** All mission objects are tracked, classified
and displayed on an interactive map.

### Added

- **Backend**
  - `internal/category`: classification of **exact DCS types** into families
    (aircraft, helicopter, ground, ship, structure), with override via JSON file.
  - `internal/theatre`: DCS theatres (Caucasus, Syria, NTTR, Gulf, Marianas, etc.)
    with their geographic extent.
  - `internal/udp`: support for **`world`** messages (object lists) in addition to
    `ownship`; datagrams up to 1 MB.
  - `internal/state`: stable identifiers, unit cap (`MaxUnits`), age per unit.
  - `internal/api`: filters (`?category=`, `?coalition=`, `?ownship=`, `?q=`), summary
    by category/coalition, `GET /api/units/{id}`, `GET /api/theatres`, and serving of
    **DCS map tiles** (`/api/tiles/<theatre>/<z>/<x>/<y>.png`) with protection
    against path traversal.
  - `internal/config`: new options (`DCSMM_UNIT_TTL`, `DCSMM_TILES_DIR`,
    `DCSMM_BASEMAP_URL`, `DCSMM_CATEGORIES`, `DCSMM_MAX_UNITS`).

- **DCS scripts**
  - `Export.lua`: export of **all world objects** in addition to the player, filterable
    by radius (`dcsmm_world_radius`), capped (`dcsmm_max_objects`), selectable
    coalitions; defensive access to the `LoGet*`/`Export.*` functions.
  - `Config/dcsmm.cfg`: new documented options.

- **Frontend**
  - Map: SVG icons per category, colors per coalition, **flight trails** for
    aircraft and helicopters, hover with tooltip, recenter on the player.
  - **Basemap selector**: Satellite, Relief, Road, Dark — plus the automatic
    "DCS" option if authentic tiles are present. No basemap
    requires an API key (dark mode is a CSS filter on the OSM tiles).
    The choice is remembered in the browser.
  - Side panel: coalition/category filters with live counters, search,
    "my aircraft only", list of selectable units.
  - Unit card: type, category, coalition, country, position, altitude, heading, age.
  - Support for **DCS tiles** with automatic fallback to a real basemap.

- **Tools & documentation**
  - `tools/send-telemetry.mjs`: also emits world objects (ground, ships, AI).
  - `tools/export-tiles.py`: tile extraction from a georeferenced map image.
  - `tools/inspect-maps.py`: inventories map scans and exports the corners.
  - `categories.example.json`, `docs/live-map.md`, `maps_dcs/README.md`.
  - `maps_dcs/` (≈1.2 GB) excluded from git; only its README is tracked.
  - Additional unit tests (categories, theatres, store cap, world messages).

## [0.1.0] — 2026-09-25

First version: **PoC Phase 0** — validation of the DCS → Go → browser chain.

### Added

- **Documentation**
  - `README.md`: vision, architecture, prerequisites, quick start, configuration,
    Lua installation, `.exe`/Docker deployment, structure, roadmap.
  - `CHANGELOG.md`: version tracking.
  - `docs/architecture.md`: data flow, components, technical choices.
  - `docs/dcs-installation.md`: DCS-side installation guide and troubleshooting.

- **Tools**
  - `tools/send-telemetry.mjs`: fake telemetry emitter for testing without DCS.
  - `Makefile` and `build.ps1`: build commands (frontend, backend, Docker).

- **DCS scripts (`dcs-lua/`)**
  - `Config/dcsmm.cfg`: configuration template (backend IP, ports, send interval).
  - `Export.lua`: export of the player's position to the backend over UDP/JSON,
    sampled once per second via `LuaExportActivityNextEvent` (no impact
    on simulator performance).

- **Go backend (`backend/`)**
  - `internal/config`: configuration via environment variables (`DCSMM_*`) with defaults.
  - `internal/udp`: UDP receiver decoding the JSON positions.
  - `internal/state`: in-memory store of units (with expiry).
  - `internal/api`: HTTP server (REST `GET /api/state`, **Server-Sent Events**
    stream `/api/events`) + serving of the embedded web interface.
  - `cmd/dcsmm/main.go`: entry point assembling the building blocks.

- **Frontend (`frontend/`)**
  - Minimal Leaflet interface displaying unit positions in real time via
    **Server-Sent Events**, with a fallback page generated if the frontend is not built.

- **Deployment (`deploy/`)**
  - Multi-stage `Dockerfile` (frontend + backend build → minimal Alpine image, non-root).
  - `docker-compose.yml` with published UDP/TCP ports and SQLite volume.

### Notes

- DCS World stays on Windows and never runs inside the Docker container; the manager
  communicates with it over the network.
- Phase 0's real time uses **SSE** (downstream flow). A bidirectional TCP
  channel (commands to DCS) is planned for Phase 2.
