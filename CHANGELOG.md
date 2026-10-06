# Changelog

🇬🇧 English | [🇫🇷 Français](CHANGELOG.fr.md)

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [semantic versioning](https://semver.org/).

## [Unreleased]

### Fixed

- **A restore no longer proceeds when its safety copy fails.** Restoring replaces
  live DCS files, so the pre-restore `*.prerestore.zip` is now mandatory: if it
  cannot be written (or its categories cannot be resolved), the restore is refused
  with an error and nothing is overwritten.
- **Panel encoders now honour their direction.** Both detents of a wheel report the
  same "active" edge, so the old mapping sent the same argument either way and a
  trim wheel or the PZ70 LCD wheel could only turn one direction. The sign now comes
  from the direction (`variable_step`: ±step, `fixed_step`: INC/DEC).
- **The airfields tab no longer shows charts of a previously selected field.** A
  slow chart response for an airfield picked earlier is ignored when it arrives
  after the one now shown.
- **The nearest-field lookup is filtered by the selected theatre.** It used to
  search every theatre, so a field absent from the current map could appear first.

### Added

- **The panels' LEDs are driven from DCS-BIOS.** The PZ55's landing-gear lights
  and the PZ70's autopilot button lights now reflect the cockpit, read from the
  values DCS-BIOS exports (the gear lights are bicolour: green for down-and-
  locked, red for unsafe). This is the missing half of the panel bridge —
  `mapping` already sent a switch move into the cockpit — and the Go counterpart
  of the original profiles' `outputBindings`. Output bindings live in the same
  per-aircraft profiles as the input ones and are edited in the Panels tab; the
  starter profiles carry the gear lights (F-16C, F-5E-3, F/A-18C, M-2000C) and the
  M-2000C's autopilot lights. They have their **own switch** (see below),
  independent of command sending, and a profile file written before outputs
  existed is backfilled in memory from the starters (an explicit empty list is
  respected).

- **A live mapping test in the Panels tab.** Turn it on, move a switch or press a
  button, and each change is shown with the DCS-BIOS command the binding would
  send — so a mapping can be checked without flying. The test can also send the
  input to DCS-BIOS while it is on (so the cockpit reacts), independently of the
  command-sending switch; like that switch it resets on restart.

- **A debug mode in Settings.** A switch turns on a live log of the manager's
  actions: every API request, panel input and the command it produces, DCS-BIOS
  state change, mission transition and debrief transfer. The lines are streamed to
  a **Debug** sub-tab of Settings (newest first, colour-coded by level) and echoed
  to `data/dcsmanager.log` / the console. It is off by default and can be turned on
  at startup with `DCSMANAGER_DEBUG=1`; the switch is `GET/POST /api/debug` and the
  log is `GET /api/log`.

- **The aircraft can now be chosen by hand in the Panels tab.** Bindings are per
  aircraft and used to appear only once DCS-BIOS reported the active one — which
  meant nothing was editable or testable with DCS closed, exactly when a mapping is
  set up. A picker lists every aircraft with a profile (`GET /api/aircraft`) and
  loads its bindings without a mission; DCS-BIOS still overrides it while an
  aircraft is active.

- **Panel outputs now have their own switch, and the panel driver builds one
  complete report.** Driving the LEDs no longer requires arming command sending
  (which writes the pilot's inputs into the aircraft): the outputs switch
  (`POST /api/mappings/outputs`) only reads DCS-BIOS and writes the panels. The
  driver is ticked on **every** DCS-BIOS frame (not only on a state change), merges
  the PZ55's gear lights and the PZ70's light byte and (to come) LCD lines into a
  single report, writes only when it changes, and drops its cache on a panel
  reconnect or an aircraft change so the panel is redrawn. The profile also gained a
  validated `displays` section (LCD: mode, line, source, export index, scale/offset,
  unit), unused by the UI yet.

- **A PZ70 display editor in the Panels tab.** A "PZ70 display" section binds one
  LCD line to a value DCS-BIOS exports, per selector mode (ALT/VS/IAS/HDG/CRS): pick
  the source control, the export index, a scale/offset and a unit label. A **Preview**
  resolves the binding against the live DCS-BIOS memory and shows the raw value, the
  converted value and the exact LCD text (`POST /api/display/preview`); a **hardware
  test** writes chosen numbers to the two lines so every cell and the sign can be
  confirmed on the real panel (`POST /api/display/test`). Both are independent of
  command sending.

- **Starter LCD profiles for the M-2000C and the F-5E-3.** Each shows the selected
  heading (HDG) and course (CRS) — the references those aircraft actually export as
  integers (the HSI angles, normalised 0..1 and so scaled by 360/65535 degrees). The
  F-16 and F/A-18 export their selected values as text, which this version does not
  read, so they get no display; the mode is left to the operator to fill once the
  panel is validated.

### Changed

- **The interface is a sidebar + topbar shell.** Navigation moved from a header
  tab bar to a sidebar rail with grouped views (Analysis / Reference /
  Configuration) and a titled topbar, and the statistics overview cards were
  reworked (a coloured accent per metric, a larger value). The panels themselves
  are unchanged; only the surrounding shell and the KPI cards moved.
- **The install, modules and panels tabs are now one "Settings" tab with
  sub-tabs.** The three had become a cluster of configuration-focused tabs in the
  main bar, so they are gathered behind **Settings** — sub-tabs *DCS install*,
  *Modules* and *Panels* — leaving the bar to the flight-facing views (Debriefs,
  Career & statistics, Airfields). The panels are unchanged; only their
  place in the bar moved.
- **Statistics and Career are now one tab, "Career & statistics".** The two
  answer different questions — "what does DCS say I have done?" (the logbook in
  `MissionEditor/logbook.lua`: rank, squadron, awards, hours, per-airframe kills)
  and "what did the manager record?" (the aggregates built from its own sessions:
  pilots, weapons, airframes, balance, network) — so they are stacked rather than
  blended, the logbook on top and the statistics below, under a single Refresh
  and a single scroll. `StatsPanel.svelte` was merged into `CareerPanel.svelte`;
  the separate Career tab is gone.
- **The project is renamed to DCS Manager.** It outgrew the mission-only scope,
  so the name, the binary and every identifier that carried the old one changed:
  `dcsmm.exe` → `dcsmanager.exe`, the Go module `dcsmm` → `dcsmanager`, the CLI
  name, the environment variables `DCSMM_*` → `DCSMANAGER_*`, the Lua config
  `dcsmm.cfg` → `dcsmanager.cfg` and the hook `Hooks/dcsmm.lua` →
  `Hooks/dcsmanager.lua`. The install markers in `Export.lua` became
  `DCSMANAGER-BEGIN` / `-END`.
  - A fresh install: nothing is carried over from the previous name, and no
    migration path is provided. `dcsmanager install-lua` writes the new files.

### Removed

- **The manager is local-only: PostgreSQL, `migrate-db` and the statistics plugin
  are removed.** Persistence is SQLite only — a file in `data/` beside the binary
  — so there is no server to run, no container and nothing to configure. Gone with
  the optional engine: `internal/db/postgres`, the `DCSMANAGER_DB_DRIVER` /
  `DCSMANAGER_DB_DSN` settings and the engine switch in `openStore`, the
  `dcsmanager migrate-db` command and `internal/migrate`, the read-only `v_stats_*`
  views, the `pgx` dependency, the root `docker-compose.yml`, the whole
  `stats-plugin/` service (Go code, web UI, `Dockerfile` and `docker-compose.yml`),
  the `docs/database-backends.md` and `docs/stats-plugin.md` guides, the
  PostgreSQL/plugin sections of `docs/installation.md`, the CI `postgres` service
  containers and the stats-plugin job, and the stats-plugin mockup. The binary
  dropped from ~17.6 MB to ~13.6 MB.
- **The Missions tab has been removed.** It listed the `.miz` files under
  `Saved Games\DCS\Missions` (theatre, date, weather, size) and opened a mission's
  detail. The tab, its store and the `GET /api/missions` route are gone
  (`MissionsPanel.svelte`, `missions.js`, `internal/api/missions.go`), along with
  the `internal/dcsdata` mission reader and the `missions` section of the state.
  The manager reads no `.miz` any more; the debriefs remain the durable record of
  a flight. The bundle dropped from ~156 KB to ~149 KB.
- **The Configuration tab has been removed.** It listed DCS's own settings read
  from `Saved Games\DCS\Config` (`options.lua` groups, the `pluginsEnabled.lua`
  toggles, the UI language). The tab, its store, the `GET /api/config` route and the
  `internal/dcsdata` config reader it alone used are gone, and the bundle dropped
  from ~166 KB to ~156 KB. The manager no longer reads that folder.
- **The Analysis tab has been removed.** It drew a heatmap and flight paths, and a
  per-unit sortie table, from the recorded positions. The tab, its plot and its
  store are gone (`AnalyticsPanel.svelte`, `AnalyticsPlot.svelte`, `analytics.js`),
  along with the `/api/analytics/{heatmap,tracks,sorties}` routes and the sortie
  computation they alone used (`tracker.Analyse` and its helpers), and the bundle
  dropped from ~187 KB to ~166 KB. **Positions and losses are still sampled and
  stored**: the tracker is untouched, so the history is kept and a future analysis
  view would have something to draw on. Only the view is gone.
- **The Session tab has been removed.** It showed the connected players, the live
  game events and the chat. The manager now opens on **Debriefs**, the durable
  record of a flight. The backend still receives and stores events, players and
  chat: they feed the statistics and the debriefs, so nothing is lost — only the
  live view is gone. `ChatPanel.svelte`, `PlayerPanel.svelte`,
  `EventPanel.svelte` and the now-unused session stores were deleted, and the
  bundle dropped from ~204 KB to ~187 KB.
- **The compatibility shims for our own former versions are gone.** Now that the
  project starts fresh, code that only existed to read artefacts an older build of
  ours had produced is dead weight:
  - **The theatre aliases** (`Marianas` → `MarianaIslands`, `Sinai` → `SinaiMap`)
    were a fix for ids we once published wrong. `theatre.Resolve` is removed and
    only DCS's own ids are accepted.
  - **The "leftovers from the previous name" report** in the DCS install tab, and
    the `legacy` list behind it, are gone: there is no old install to clean up.
  - The README paragraph about upgrading from a release predating the session
    `source` column is gone too.
  - What is kept is **robustness against external input**, not against our past: a
    malformed DCS file, an unreadable archive or a hand-edited `options.lua` still
    degrade quietly, and the additive DB migrations still run.
- **The live map has been removed.** The manager now focuses on the debriefs,
  the statistics, the analysis and the airfields. Everything that only existed to
  serve the map went with it: the map
  view and its filters, the basemaps (satellite, relief, road, aeronautical,
  dark), the imported DCS F10 imagery (MBTiles/image/tile-set importers and the
  `tiles/` folder), the DCS terrain vectors (`vectors/`), the map extent outline,
  the map-tile and vector API routes, and the fog-of-war filtering.
  - The unit telemetry is **still received and sampled**: it feeds the flight
    trails, the heatmaps and the sortie analysis, and the airfields tab's
    "nearest field" lookup. Only the drawing is gone.
  - Airfields, their frequencies and their **aeronautical charts** (read from
    `maps_dcs/`) are unchanged.
  - `dcsmanager import-tiles`, `import-image`, `fetch-tiles` and `import-vectors` are
    gone, along with the `DCSMANAGER_TILES_DIR`, `DCSMANAGER_TILES_ATTRIBUTION`,
    `DCSMANAGER_VECTORS_DIR`, `DCSMANAGER_BASEMAP`, `DCSMANAGER_BASEMAP_URL` and
    `DCSMANAGER_REVEAL_ALL_UNITS` variables. Leaflet is no longer a frontend
    dependency.

### Added

- **An optional PostgreSQL backend, and a storage interface to make it
  possible.** The manager's persistence used to be SQLite through a concrete
  `*db.DB`. It now depends on a **`db.Store`** interface, implemented by the
  existing pure-Go SQLite store and by a new **`internal/db/postgres`** one, so
  the two engines are interchangeable wherever a database is used (the API, the
  ingest writer, the tracker, the statistics, the debrief store). The engine is
  chosen with `DCSMANAGER_DB_DRIVER` (`sqlite`, the default, or `postgres`) and
  `DCSMANAGER_DB_DSN`. SQLite behaviour is unchanged. The PostgreSQL translation
  needed real dialect work — `RETURNING id` instead of `LastInsertId`,
  `?` → `$n` placeholders, a sub-query with `round(…::numeric)` for the heatmap
  (no `GROUP BY` on an alias, no `round(double, int)`), and an expression index
  for "a single open mission". A cross-engine **conformance suite** runs the
  same assertions against both stores (`DCSMANAGER_TEST_POSTGRES_DSN` enables
  the PostgreSQL half in CI).
- **`dcsmanager migrate-db`: copy a SQLite database into PostgreSQL.** The
  PostgreSQL backend has no in-place upgrade path, so a one-shot command copies
  the existing history over: every table at the SQL level (the `Store` interface
  hides raw columns like `player_stats` and the tracking), in dependency order,
  preserving ids (`OVERRIDING SYSTEM VALUE`) and advancing each identity
  sequence, and idempotent (`ON CONFLICT DO NOTHING`) so it can be re-run.
- **Read-only `v_stats_*` views: the statistics interface.** The manager now
  exposes the data a reader needs through views (`v_stats_missions`, `_players`,
  `_player_stats`, `_events`, `_debriefs`, `_track_positions` and `_config`),
  each with a `source` column so a reader applies the live/test policy without
  knowing the internal tables. Exposing views rather than tables lets the schema
  change without breaking a reader.
- **A separate statistics plugin (`stats-plugin/`).** An optional service that
  reads the manager's PostgreSQL database directly (read-only, opened with
  `default_transaction_read_only=on`, and best pointed at a SELECT-only role) and
  serves its own dashboard: overview, pilots, weapons, airframes, network,
  missions and trends, plus CSV/JSON exports. It never calls the manager over
  HTTP, so the manager can stay on loopback while the plugin runs elsewhere. It
  ships a Dockerfile and a `docker-compose.yml`.
- **Incremental history endpoints.** `GET /api/history/events`, `…/chat` and
  `…/missions` accept `?sinceId=N` and return, oldest first, only the rows with
  an id greater than N plus a `nextSinceId` cursor — a consumer can mirror the
  whole history without gaps or duplicates. Without `sinceId` the endpoints keep
  their original "most recent" behaviour.
- **Player-profile backup.** Save and restore the parts of `Saved Games` that are
  painful to lose — the logbook, the control bindings, the options, the scripts,
  and optionally the kneeboard, missions and mods — as a portable zip with a
  manifest, with the categories chosen per call. Available three ways: the CLI
  (`dcsmanager backup` / `restore`, with `--categories`, `--out`,
  `--list-categories`, `--dry-run`), the API (`/api/backup`,
  `/api/backup/categories`, `/api/backup/restore`, `/api/backup/download/{name}`)
  and a **Settings → Player profile backup** sub-tab. Restore is safe by design:
  a dry run reports what it would write, a **pre-restore safety copy** of the
  current state is kept beside the archive, and entries that would escape Saved
  Games (`../`) are refused.
- **An optional API token.** `DCSMANAGER_API_TOKEN` protects the API when the
  manager is deliberately exposed beyond the loopback interface: non-local
  callers must present it (`Authorization: Bearer`, or `?token=`, which sets a
  cookie). Loopback callers stay exempt, so the normal local setup needs no
  configuration, and the comparison is constant-time.
- **Ready-made panel profiles for four aircraft.** The first time the binding
  file is absent, the Cockpit panels tab is seeded with working bindings for the
  F/A-18C, the F-16C, the F-5E-3 and the Mirage 2000C — the aircraft the original
  DCS Panel Manager shipped profiles for — so a new cockpit has something that
  works without mapping every switch by hand. They are adapted to this manager's
  one-command-per-control model: the battery switch maps to a single `set_state`
  binding, and the gear lever is two controls (`GEAR_UP`/`GEAR_DOWN`) driving the
  two positions of one command. Output bindings (gear lights, the PZ70 LCD and
  button LEDs) are not part of the model yet, so they are not reproduced.
  Seeding is one-way and only fills a missing file: an existing binding file,
  even one emptied by the operator, is never touched, and seeding never arms
  sending. A test validates every shipped command and interface against the
  machine's real DCS-BIOS metadata.
- **A logo.** The header now shows the DCS Manager emblem beside the title, and
  the README opens on it. The source art lives in `Logo/`; build-friendly copies
  are `frontend/src/assets/logo.png` (the transparent emblem, 128 px, resized and
  bundled into the UI) and `docs/logo.png` (the dark-background emblem for the
  README). The originals are 1.2 MB each, so they are never shipped or served
  directly, and `Logo/` is git-ignored (only the optimized derivatives are
  committed).
- **The executable now carries an icon and version metadata.** Windows takes a
  program's icon (and the FileDescription / ProductName / version shown in the
  taskbar, Alt-Tab and the file's Properties) from a resource object linked into
  the binary. `backend/cmd/dcsmanager/rsrc_windows_amd64.syso` provides it: the
  emblem at 16/32/48/64/128/256 px plus the version block, so `dcsmanager.exe`
  looks like an application rather than a generic binary. The `.syso` is
  committed, so an ordinary `go build` embeds it with no extra tool; regenerate
  it with `.\build.ps1 -Target winres` (or `make winres`) after changing the icon
  or `winres/winres.json`, which needs `go install github.com/tc-hib/go-winres@latest`.
- **The cockpit hardware side of the DCS Panel Manager is now part of the
  manager.** A Logitech/Saitek PZ55 Switch Panel and PZ70 Multi Panel can be
  driven directly, and DCS-BIOS is spoken rather than reimplemented, so a cockpit
  already running it needs no second tool. This absorbed a separate C#/Avalonia
  application into this one, in Go, keeping the single-binary `CGO_ENABLED=0`
  build.
  - **`internal/hid`** reads Windows HID devices with no cgo:
    enumerate, open, read the caps, read reports with a timeout, write output
    reports. Four traps had to be found on real hardware — the interface path is
    at offset 4 (not 8) in `SP_DEVICE_INTERFACE_DETAIL_DATA_W`,
    `SetupDiEnumDeviceInterfaces` takes five arguments, `HidP_GetCaps` wants the
    preparsed data rather than the device handle (passing the handle crashes the
    process), and the handle must be opened with `FILE_FLAG_OVERLAPPED` or
    `ReadFile` blocks forever on an idle panel.
  - **`internal/panel`** decodes and encodes the PZ55/PZ70 protocol: switches,
    buttons and encoders (reporting on the rising edge, so one detent is one
    event), gear LEDs, the autopilot LCD and its button lights.
  - **`internal/panelservice`** keeps a reader per panel, handles hot-plug, and
    publishes the inputs as events. A panel another tool holds, or a read that
    keeps failing, is reported rather than fatal.
  - **`internal/dcsbios`** decodes DCS-BIOS' export stream (the `0x55` sync and its
    address/length/data blocks), resynchronises after a lost datagram, lifts the
    active aircraft out of the memory image, and sends commands back.
  - **`internal/biosmeta`** reads the control catalogue DCS-BIOS publishes per
    aircraft, and **`internal/mapping`** binds a panel control to one of those
    commands. Sending is **off by default and off again on every restart**: driving
    a live cockpit must be deliberate, and must not survive a restart unnoticed.
  - A **Cockpit panels** tab shows the DCS-BIOS link, one card per panel, a live
    monitor of the inputs, and a mapping editor (286 bindable controls on the
    F-16C, read from its own metadata).
  - The manager's own telemetry moved to **UDP 7776** so it can run alongside
    DCS-BIOS, which owns 7778.

- **A DCS install tab shows what lives on the game's side of the fence.** It reports
  the mods installed under `Saved Games\DCS\Mods` and the state of the scripts, so a
  broken or half-finished install is visible instead of silent.
  - **Mods**: category, name, file count and size, and whether the mod ships an
    `entry.lua` (its absence flags an incomplete mod). Total size is summed.
  - **Manager scripts**: each managed file as up to date / outdated / not installed.
  - **Export.lua**: which other tools share it (Tacview, DCS-BIOS, SRS, LotAtc,
    VAICOM, BattleHub…), detected from the `dofile`/`require` lines.
  - A summary banner says when a managed file needs attention.
  - `internal/dcsdata` reads the Mods tree and inspects the Scripts folder; it
    reuses `internal/install`'s own status for the managed files rather than
    duplicating the comparison. `GET /api/mods` and `GET /api/scripts` expose it.
  - Covered by tests building a Mods tree and a Scripts folder in a temp dir
    (including backups, and our own files being excluded from the third-party
    list), plus the API's empty and populated cases.

- **A Missions tab is a library of the `.miz` saved in Saved Games.** A `.miz` is a
  ZIP; the manager reads the editor data inside it and lists each mission with its
  **theatre**, **in-game date and start time**, **weather and temperature**,
  **size** and last modification — without launching the game.
  - `internal/dcsdata` opens the archive, reads only the `mission` entry (bounded,
    so a hostile archive cannot exhaust memory) and parses the metadata with the
    existing Lua parser. A broken or empty archive is still listed; a missing
    folder yields an empty list, never an error.
  - `GET /api/missions` returns the library; `?theatre=` filters it.
  - Tested by building a `.miz` in memory, against the machine's real missions,
    and on the missing-folder and empty-API cases.

- **A Career tab shows the player's own logbook.** DCS keeps a career record in
  `Saved Games\DCS\MissionEditor\logbook.lua`; the manager now reads it and shows
  the pilot's **rank**, **squadron**, **awards** and **invulnerability**, the
  career totals (flight hours, missions, landings, score), and a **per-airframe
  breakdown**: flight hours, landings, deaths, ejections and air-to-air /
  air-to-ground kills, with a bar scaled to the aircraft flown the most.
  - `GET /api/career` returns the profiles; the tab switches between them when a
    machine has more than one.
  - This is the pilot's record as DCS itself keeps it, complementing the
    statistics the manager builds from its own recorded sessions.
  - Covered by a test against the machine's real `logbook.lua` when present, a
    synthetic document, and the API's empty and populated cases.

- **A Modules tab lists what DCS itself reports as installed.** The manager reads
  DCS's own inventory (`Saved Games\DCS\MissionEditor\modules.lua`) instead of a
  hand-maintained catalogue: terrains, aircraft, navigation systems, tech packs,
  campaigns and bundles, each with its **type**, developer, DCS id and the
  **installed versions**, and whether the player **owns** it.
  - `internal/dcsdata` parses the file with the existing Lua parser (no new
    dependency) and normalises the sections DCS writes as either slices or
    numbered maps. It degrades gracefully: a missing file yields an empty
    inventory, not an error.
  - `GET /api/modules` returns the list, with `?owned=1` for the owned subset.
  - The tab filters by **search** (title, developer, id), by **category** and by
    **owned only**, and shows the owned/total count. On the test machine it finds
    183 entries, 22 owned, 17 terrains of which 5 owned.
  - Covered by a test against the machine's real `modules.lua` when one is
    present, plus a synthetic document and the API's empty and filtered cases.

- **Chat messages can now be sent into DCS (command channel).** The TCP link was
  already bidirectional by construction; the backend now writes commands down the
  same connection the hook uses to report, and the hook executes them.
  - `POST /api/chat` no longer answers `501`: it hands a
    `{"type":"command","command":"chat",…}` line to the TCP listener, which fans
    it out to every connected hook. It answers `503` when no hook is connected
    (game not running, or scripts not installed) — a normal state, shown as such
    in the UI, not an error.
  - `Hooks/dcsmanager.lua` reads pending commands from `onSimulationFrame`, with a
    **zero timeout**, so a frame is never stalled, and injects the message with
    `net.send_chat`. An unknown command is logged and ignored, so a newer backend
    cannot break an older hook.
  - `internal/tcp` tracks its connections and exposes `Connected()` /
    `SendCommand()`, behind a small `api.Commander` interface so the HTTP layer
    stays free of the transport.
  - Covered by tests on both sides: the backend fan-out and the "no hook" case,
    and the `503`/`200`/`400` answers of `POST /api/chat`.

- **The analysis tab draws its data instead of only listing numbers.** With the
  map gone, the heatmap had become a count and the sortie stats a table. The
  analysis now has a **top-down plot** built in plain SVG (no map library): the
  heatmap as coloured grid cells (blue to red, square-root scaled so a few dense
  cells do not hide the rest) and the recorded flight paths as polylines, over a
  shared bounding box with corner coordinates.
  - The plot is **projection-free but not distorted**: one degree of longitude is
    scaled by `cos(latitude)`, so the picture keeps its true proportions — the
    difference is large on Kola or The Channel, where a naive square plot
    stretches the map horizontally.
  - Two toggles (density, flight paths) turn each layer on and off; they are
    remembered across restarts. Switching between "Traffic" and "Losses"
    re-fetches only the heatmap, leaving the paths in place.
  - The sortie table gained a **distance bar** per row, scaled to the longest
    sortie, so the ranking reads at a glance.
  - The projection helpers live in `frontend/src/lib/plots.js` and handle the
    degenerate cases explicitly: a single point, a perfectly straight track, and
    an empty result all plot without a division by zero.

- **The manager now opens in its own window instead of a browser tab.** Starting
  `dcsmanager.exe` shows the embedded UI in a native WebView2 window: no browser to
  launch, no `localhost:8080` to remember, and closing the window shuts the
  manager down. The window is driven by `github.com/jchv/go-webview2`, a pure-Go
  binding, so the build still needs no C toolchain (`CGO_ENABLED=0` unchanged).
  - The HTTP server is kept, not replaced: the window loads `http://<addr>/`, so
    the page reaches the API exactly as before, and `dcsmanager serve` still exposes
    the UI to a browser (second monitor, tablet). The manager falls back to that
    headless mode on its own when the WebView2 runtime is unavailable.
  - `DCSMANAGER_HTTP_ADDR` now accepts a port of `0`, in which case the OS picks a
    free one and the window is pointed at the resolved address.
  - The manager's wiring moved to `internal/app` so the window and the headless
    mode share one implementation and cannot drift apart.

- **Cold War Germany now ships a bundled airfield dataset.** The manager already
  reads every installed map's airfields from DCS's own files, and Cold War
  Germany is no exception (119 fields, with Tower/TACAN/ILS/NDB). What was
  missing was the offline fallback: with no DCS found, the binary fell back to
  the Caucasus dataset alone. `internal/aerodrome/data/germanycw.json` now covers
  this theatre too, so the Airfields tab stays useful without an installation.
  - The file is **generated from DCS**, not typed by hand:
    `go run ./cmd/gen-aerodrome <Mods/terrains> <Theatre> <out.json>` extracts
    `radio.lua` and `beacons.lua` and writes the dataset for any theatre.
  - DCS gives every field a Tower frequency but a position only when a navigation
    aid exists, so **77 of the 119 fields carry coordinates and 42 do not.** The
    UI now says "not positioned by DCS" instead of rendering `0.0000°` as if it
    were a location, and an empty coalition reads "unknown".
  - A dedicated test locks the dataset in (119 fields, Frankfurt's TACAN and
    position, 70+ placed); the integrity test no longer demands coordinates from
    every field, which DCS cannot provide.

### Fixed

- **A player's totals were multiplied by the number of samples.** The hook resends
  the same cumulative counters every few seconds (and at each connect, disconnect
  and slot change), and statistics summed every row: three snapshots of "1 kill,
  100 points" read 3 kills and 300 points, for the player and for their coalition.
  Per-player and per-coalition totals now come from the **latest snapshot per
  mission**, the final counters DCS reported, so one sortie counts once.
- **A DCS player id reused in a later mission attributed deaths to the wrong
  player.** Event arguments carry the *DCS* player id, which is only meaningful
  within its mission: a career-wide `id → name` map gave one player's deaths to
  another whenever an id was reused. Events are now resolved by
  `(mission_id, dcs_player_id)` before being aggregated by identity.
- **A name-only player never adopted their UCID.** A player first recorded without
  a UCID kept an empty one even after DCS reported it, so a later rename created a
  second identity. The UCID is now recorded when an anonymous row is matched.
- **Several missions could be open at once, or a new flight merged into a stale
  one.** The open-mission get-or-create is race-prone, and a mission start arriving
  while a previous mission was still open (manager interrupted, end message lost)
  was merged into it. The get-or-create is now serialised and backed by a unique
  index, and an explicit start closes the stale session first while an identical
  repeated start stays the same flight.
- **Purged missions left the ingestion with a dead id.** After a purge, events
  failed on the foreign key and were lost until a restart. The writer revalidates
  the mission before writing and opens a fresh one when it is gone.
- **A corrupt binding file no longer hides a stale mission's samples.** The tracker
  now releases a mission that has ended instead of pinning samples to it, so a new
  flight is never recorded into the previous one.
- **A failed profile save no longer changed the active profile.** Bindings were
  published in memory before being written, so a refused save reported an error
  while the manager kept the new state (and a panel could act on it). The new state
  is persisted first and published only on success; a failed delete restores the
  previous profile too.
- **A corrupt debrief transfer is recovered, not lost.** The TCP sender declares
  the file size in every chunk, but the assembled content can come out longer
  (observed in the field: the valid file followed by `2 × base64(fragment)`, a
  transport corruption between DCS's LuaSocket and the backend). The assembler now
  compares the assembled length to the declared size: when it is **longer**, the
  real file is its first `size` bytes and that prefix is stored once it parses;
  when it is **shorter**, data is genuinely missing and the transfer is dropped and
  written to `data/rejected/` for inspection.
- **An empty player roster was dropped, taking the whole message with it.** When
  no player was connected, the Lua serializer sent the roster as `{}`: its
  `isArray = #v > 0` test cannot tell an empty table from an empty object, so an
  empty table came out as an object, and the backend's `[]Player` field refused
  it. `json.Unmarshal` failed on the **whole line**, so the roster — and any event
  it travelled with, since the hook resends the roster on every connect, disconnect
  and slot change — was lost in silence. The serializer now emits `[]` for an empty
  table, and the backend accepts both shapes: an empty roster, a roster keyed by
  player id, and an empty `[]` where the mission options are a map. A roster that
  is neither an array nor an object is still reported rather than hidden.
- **Career flight hours were wildly inflated.** The Career tab showed thousands
  of hours for a handful of flights (5 477 h for 6 sorties), because DCS stores
  the `flightHours`, `daytime` and `nighttime` fields in **seconds** despite
  their name — a well-known DCS quirk (the game counts time in seconds, so a raw
  value reads as thousands of "hours"). The manager displayed the raw number as
  if it were hours.
  - The logbook parser now divides those three fields by 3 600, for the
    per-airframe rows and the flat career totals alike, so the whole payload is
    consistently in hours. The same player now reads **1.5 h** (M-2000C 1.1 h),
    which matches the flights.
  - A test on the machine's real `logbook.lua` rejects any single airframe above
    5 000 h (a tell-tale of unconverted seconds) and checks the total equals the
    sum of the airframes; the synthetic fixture now uses seconds too.
- **The Modules tab confused "owned" with "installed".** DCS's
  `MissionEditor/modules.lua` is the store catalogue: its `have="1"` means the
  player *bought* the module, not that it is present on disk. A map purchased and
  then **uninstalled to free space keeps `have="1"` forever**, so the tab kept
  showing it as installed — Kola and the Persian Gulf, for instance, after
  uninstalling them to make room for Cold War Germany.
  - The manager now also reads **`autoupdate.cfg`** at the game root, DCS's own
    list of the modules present in the installation (`GERMANYCW_terrain`,
    `CAUCASUS_terrain`…), and matches each module on all of its identifiers
    (`modulId`, `update_id`, `code`). The two facts are shown separately:
    **Owned** and **Installed**.
  - Only content units (terrains, aircraft) carry an install state; a campaign or
    a bundle ships with a module and shows "n/a" rather than a guessed value. When
    no installation can be found, the state is left unknown instead of assumed.
  - A new **"Installed only"** filter joins "Owned only"; `/api/modules?installed=1`
    backs it. Covered by tests using this machine's own files.
- **A bug hunt across the backend, the Lua scripts and the UI.** The most
  consequential findings, all verified against DCS's own installed API
  documentation (`API/Sim_ControlAPI.md`) or the real files on disk:
  - **Every in-game chat message was dropped.** DCS passes `onChatMessage` a
    *numeric* player id as `from`, but the backend decodes `from` as a string:
    `json.Unmarshal` failed on the whole line and the message was discarded in
    silence (live view and database). The hook now resolves the player's name
    (falling back to the id as text), so chat is never lost.
  - **The config file was never applied.** `dcsmanager.cfg` began with `#`
    comments, but it is loaded with `loadfile()` and must be valid Lua; `#` is a
    syntax error, so the whole chunk was rejected and *every* setting —
    including `dcsmanager_host` and the ports — was silently ignored. Comments
    are now `--`, `tools/check-lua.mjs` parses the config (a test locks it in),
    and the embedded copy was regenerated.
  - **Every periodic timer in the hook was stuck.** `LoGetModelTime` is not a
    global in the Hooks Lua state (the export API lives in the `Export.`
    namespace), so `t` was always 0 and the "players", "slot types" and
    "read backend commands" timers never fired: `POST /api/chat` could not reach
    DCS. The hook now calls `Export.LoGetModelTime`.
  - **Weapon, victim and killer-type statistics were empty.** `onGameEvent` was
    declared with four parameters, but DCS passes up to seven
    (`kill` = killer ID/type/side, victim ID/type/side, weapon). The tail —
    including the weapon name — was dropped. All arguments are now forwarded.
  - **The mission's theatre was never sent,** so every mission was recorded as
    "Caucasus" regardless of the map. The hook now reads it from
    `Sim.getCurrentMission()`.
  - **The command-channel chat only reached the server's own coalition.**
    `net.send_chat` needs `(message, true)` to broadcast; without the second
    argument it is side-limited.
  - **The airframe charts were unreachable.** Nothing selected an airfield's
    charts or opened the viewer, so the whole chart feature was dead UI. The
    detail pane now loads the charts on selection and opens them in the viewer.
  - **The bundled TACAN/VOR extraction could keep the wrong aid.** A field's VOR
    was overwritten by a `world_*` one attached by name; it now keeps the first
    (the field's own).
  - **UI races and stale data:** selecting a debrief, switching the stats scope
    or the heatmap source could land an older response last and show the wrong
    data; these now drop stale responses. Changing theatre no longer keeps the
    previous map's airfield selected, "Near me" refreshes the data-source badge,
    missing HTTP statuses surface as errors instead of empty results, and the
    i18n substitution no longer mangles values containing `$&`.
  - **The test tools crashed on a bad argument.** `node send-telemetry.mjs host
    abc` threw an uncaught `ERR_SOCKET_BAD_PORT`; an invalid port or duration is
    now rejected with a clear message, and UDP errors are handled.
  - Removed two dead declarations flagged by staticcheck; a test no longer
    contains a "this value is never used" assignment. `staticcheck ./...` and
    `go vet ./...` are now clean.
- **A non-Windows build spammed panel errors forever.** `hid.Enumerate` returns
  `ErrUnsupported` on a platform without Windows HID support, and the panel
  service published it as an error event on every poll tick — the UI would fill
  with "hid: only supported on Windows". The scanner now treats that signal as
  "no panels here", stops scanning, and never raises it as an error. This is what
  made the Linux `go test -race ./...` fail: `TestServiceStartsAndStops` asserted
  no error events. Covered by a platform-independent test that injects the
  unsupported error.
- **Two kinds of TACAN were missing from the airfield data.** Both were found on
  Cold War Germany, and both dropped the field's TACAN silently:
  - DCS ships two TACAN flavours — `BEACON_TYPE_TACAN` (often paired with a VOR)
    and `BEACON_TYPE_AIRPORT_TACAN`, the field's own facility. Only the first
    was handled, so **Nordholz (118X NDO)** lost its TACAN, and was even shown as
    an NDB because the unrecognised entry fell into a fallback.
  - Some aids are modelled as a `world_*` beacon with **no airfield id**, naming
    the field in `display_name` instead. The beacon reader discarded every
    `world_*` entry, so the VORTACs of **Hamburg (78X HAM)** and **Fulda
    (58X FUL)** never reached their airfield. A *named* world beacon is now kept
    and attached to the airfield of the same name; a nameless one is still
    ignored.
  - Cold War Germany now reports **22 TACAN** airfields instead of 19, and the
    bundled dataset was regenerated. The Caucasus is unchanged (6 TACAN, 5 VOR,
    4 RSBN); covered by a test that feeds both quirks to the extractor.
- **The native window could crash the whole manager on startup.** The WebView2
  control, its COM objects and its message loop all have to live on one OS
  thread, but the window was created from a goroutine Go is free to migrate
  between threads: the completion callback then ran on a different thread and
  dereferenced a half-initialised object, crashing the process with an access
  violation. The window goroutine is now locked to its thread
  (`runtime.LockOSThread`). Verified over five consecutive launches.
- **The statistics endpoints panicked when there was no database.** With
  persistence off (`DCSMANAGER_DB_ENABLED=false`, the test-tool mode),
  `stats.New` still returns a service but with a nil database, and the guard
  only checked `stats == nil` — never the database — so every stats route
  dereferenced a nil `*db.DB`. They now answer `{"enabled": false}` with a 200,
  and the tab shows a clear message instead of empty tables or an error. Covered
  by a test over all five routes.
- **`SouthEastAsia` was offered as a theatre, but DCS has no such terrain.** The
  theatre list carried an entry that is not one of the 14 terrains DCS sells, so
  the UI advertised a map that cannot be flown. The list is now exactly DCS's own
  set (Caucasus, Syria, Nevada, Persian Gulf, Marianas, Marianas WWII, Sinai,
  Kola, Afghanistan, Iraq, South Atlantic → `Falklands`, Normandy, The Channel,
  Cold War Germany), and a test rejects any id DCS does not have.
- **Most of DCS's navigation aids were missing from the airfield data.** The
  beacon reader only handled `BEACON_TYPE_AIRPORT_HOMER` and a few VOR types, so
  every `BEACON_TYPE_HOMER`, `BEACON_TYPE_ILS_FAR_HOMER`,
  `BEACON_TYPE_ILS_NEAR_HOMER`, `BEACON_TYPE_VORTAC` and `BEACON_TYPE_DME` in
  DCS's own `beacons.lua` was silently ignored. Plain homers and the ILS outer /
  inner markers are non-directional beacons (an ADF or the markers of an ILS),
  and a VORTAC is a VOR that also carries a TACAN: all of them belong in the data
  card. Measured on the five installed maps:
  - NDBs: Caucasus 0 → **47** (on 18 airfields), Persian Gulf 0 → **10**,
    Kola 0 → **1**, Marianas 0 → **1**.
  - VORs: Persian Gulf 1 → **16**, Kola 4 → **9**.
  - TACANs: Persian Gulf 8 → **10** (the VORTACs).
- **Starting the manager twice died silently.** A second `dcsmanager.exe` failed on
  the UDP bind and exited, printing to a console the user never sees when
  double-clicking — it looked like nothing happened at all. The manager now asks
  `GET /api/health` for its own service name before starting: in window mode it
  shows a message box and stops, and `dcsmanager serve` exits with code 3 and a
  one-line reason. A duplicate launch now says so instead of vanishing.
- **Window mode wrote no log.** A double-clicked executable has no console, so a
  startup failure left no trace to investigate. Window mode now appends to
  `data/dcsmanager.log` (next to `DCSMANAGER_DB_PATH`), which is where "it does not start"
  finally gets an answer.

### Fixed

- **The debrief and the mission-end message were lost: `conn:send` only wrote part
  of the line, and the partial write looked like success.** LuaSocket's `send`
  may write a fraction of the buffer and still return a value, so the code's
  `if not ok` check passed while the rest of the line — including the closing
  newline — was dropped. The backend, reading newline-delimited JSON, discarded
  the incomplete line as malformed. The log said "debrief sent" because the
  message had merely been *attempted*.
  - Sends now loop until every byte is written, and a real failure is reported in
    `dcs.log` instead of being announced as sent. Reproduced end to end: the same
    debrief sent by hand over the same socket is stored correctly, which is how
    the transport, not the parser, was identified as the culprit.
- **Installing on top of an older marker appended a second block instead of
  updating the first.** A file written by an earlier installer carried a marker
  whose punctuation had been damaged by an encoding round-trip (the em dash had
  become `â€"`), so the byte-for-byte search no longer matched it and the block
  was appended again. Both blocks defined the same Lua globals, and the older one
  kept the `setpayloadsize` bug. The installer now recognises a block by its
  keyword on a comment line, collapses duplicates into a single block, keeps the
  content between them, and removes every block on uninstall. Covered by tests,
  including one that a keyword merely mentioned in user code is never taken for a
  block boundary — a mistake there would delete real content.
- **The live map received nothing: `setpayloadsize` does not exist in DCS's
  LuaSocket.** The connect sequence was `socket.udp()` → `setpayloadsize(65507)` →
  `setpeername(host, port)`. The middle call raises an error, and because connect
  ran inside a `pcall`, the error was swallowed: **`setpeername` was never
  reached**, so the socket had no destination and every send failed silently.
  Verified against `bin/lua-socket.dll`, where the symbol is absent.
  - The socket is now configured defensively, a failed connect is reported in
    `dcs.log` instead of hidden, and a send failure is logged rather than
    swallowed. That silence is what made this take a full session to find.
  - World messages are split into batches under the datagram limit, since
    LuaSocket's default payload size applies and a large message fails outright.
  - `tools/check-lua.mjs` parses the Lua scripts before a session, so a syntax
    error is caught here rather than costing a game restart.
- **The pause banner broke the whole layout.** Adding it as a fourth child of a
  grid that declared three rows put it on an implicit row, which sized the main
  area to its content and stretched the banner across the page. The layout is a
  flex column now, so any number of optional banners is handled — a grid with a
  fixed row count cannot be. Verified with both banners visible: header 48 px,
  each banner 32 px, map 1133 px, total exactly the viewport height.
- **The mission's view options never reached the backend, so only your own
  coalition was ever shown.** Two independent bugs in the same chain, both found
  by running the game for real:
  - the hook called `Sim.getMissionOptions`, which **does not exist**. The correct
    API is `DCS.getMissionOptions` (`MissionEditor/GameGUI.lua`).
  - the backend then looked for `optionsView` at the top level of the table, but
    DCS nests it under `difficulty`. It stayed on its restrictive default, which
    is exactly why only allies — and in fact only your own aircraft — appeared.
  The hook now calls the right API, logs which source it used, and the backend
  accepts both nested and flat shapes. Covered by `TestOptionStringNested` and
  `TestApplyMissionOptionsSetsMode`.
- **The debrief was never sent.** The hook looked for base64 in `socket.base64`,
  which is not part of LuaSocket: it lives in the `mime` module. The log said so
  plainly ("base64 encoding unavailable, debrief not sent") and the code fell
  back to nothing. `mime` is now required and used first.
- **Pausing DCS recorded every unit as destroyed.** `LuaExportActivityNextEvent`
  is only called while simulation time advances, so pausing the game stops the
  telemetry entirely. The backend read that silence as "all units vanished" and
  wrote a batch of losses at every pause — polluting the analytics and the loss
  map. The tracker now samples nothing and declares nothing lost while the feed
  is stopped, and keeps its tracked state so the map resumes where it left off.
  Covered by `TestPausedFeedDoesNotReportLosses`.
  - The state is now surfaced instead of being silent: the session frame carries
    `paused` and `feedAgeMs`, and the map shows a banner explaining that the
    simulation is not advancing. Without it, a paused game looked like a broken
    app, which is exactly how it was first reported.
- **The map extents were wrong for every theatre, and are now measured instead of
  guessed.** The bounds were literals typed by hand, and comparing them with
  DCS's own data showed they were wrong everywhere: Kola really spans 11.7 to
  40.1 degrees of longitude where the literal said 19 to 34, the Marianas reach
  latitude 20.7 where the literal stopped at 15.6, and the Persian Gulf starts at
  51.0 where the literal said 47. The extent is now computed from the theatre's
  own airfields and settlements, so it is exact and follows DCS when a map is
  patched. Verified: no airfield falls outside its theatre's box.
  - The hardcoded bounds remain only as a fallback for a map that is not
    installed, where there is nothing to measure.
- **The Marianas WWII terrain was indexed under the wrong id.** DCS ships the
  folder as `MarianasWWII` but declares the theatre as `MarianaIslandsWWII`, and
  the folder name was used as the id, so the theatre keyed its airfields under a
  name the UI never asks for. The id declared in the terrain's `entry.lua` is now
  authoritative, which also gives that map a measured extent and makes its 11
  airfields reachable.
- **Four defects found by a systematic review**, none of them visible in normal
  use:
  - **A truncated data file could crash the backend.** The gettext wrapper `_(`
    is the only value form that reached the string parser without a guaranteed
    quote, so a file ending right there indexed past the end of the input. That
    is a panic on a malformed DCS file, not an error.
  - **A malformed table could silently corrupt values.** A table mixing
    positional entries and explicit `[n]` keys had its positional values
    renumbered from 1 on top of the explicit ones. The shape contract is now
    documented by `TestTableShapes`, because it is load-bearing: DCS's
    `radio.lua`/`beacons.lua` are positional and must stay a map.
  - **The chosen airfield position and its navigation aids varied between runs.**
    Beacons were read by ranging over a Go map, so which of two ILS beacons won,
    and in which order the aids were listed, depended on map iteration order.
    Entries are now ordered. Verified over three launches: identical results.
  - **Two airfields standing close together could adopt the same embedded id**,
    after which the id index kept only one of them, with the other's name and
    runway attached to it. An embedded entry can now be claimed once.
- **`internal/charts` disagreed with `internal/theatre` on two theatre ids.**
  Charts under a Sinai folder were tagged `Sinai` (DCS says `SinaiMap`) and
  Marianas WWII `MarianasWWII` (DCS says `MarianaIslandsWWII`), so filtering by
  theatre returned nothing and an id the selector does not know was advertised. A
  test now ties the two lists together.
- **A chart whose folder matched no theatre was invisible.** It was indexed and
  servable, but a listing without a filter walked theatres only, so it never
  appeared. Such charts are now returned too.
- **A two-character airfield code could never match a chart.** The search index
  required three characters, so `H4_VAD.png` was unreachable while the code
  comment claimed the opposite.
- **`install-lua` could silently install an outdated script.** A file that
  existed but could not be read (permissions, a lock) fell back to the embedded
  copy without a word. Only a genuinely missing file does now.
- **The backup-name search was an unbounded loop.** A `stat` error other than
  "does not exist" never broke it, so it spun and grew the path string forever
  instead of reporting the problem.
- **The tracker could open two missions for the same session.** The get-or-create
  ran outside the tracker's mutex, so two goroutines could both find no open
  mission and each create one. It now runs under the lock, and the created id is
  remembered. A unit whose loss has been recorded is forgotten, so the tracking
  maps no longer grow with every unit ever seen.
- **A TCP connection handled before its callbacks were wired lost its first
  message.** The listener is now started only once every callback is in place,
  removing the window in which a handler read a nil callback.

### Security

- **The API is now protected against being driven by a web page.** It has a
  destructive endpoint (`/api/maintenance/purge`) and no authentication, and the
  server was bound to `0.0.0.0`, so:
  - any site you visited while the manager ran could `POST` to it — a plain POST
    is a "simple request" that reaches the server without a preflight — and
    delete your database. This was reproduced before fixing it.
  - a DNS-rebinding page could reach it with a matching `Origin`.
  The origin is now checked against the request's Host, the Host must name the
  local machine while the server is bound to loopback, and **the default listen
  address is `127.0.0.1:8080`** instead of `0.0.0.0:8080`. Reaching the UI from
  another device is still possible, deliberately, with
  `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080` — in which case the README says the API is
  unauthenticated. Covered by `TestOriginGuard` and `TestIsLoopbackAddr`.
- **Player names were injected into Leaflet tooltips unescaped.** Leaflet renders
  tooltip content as HTML, so a player called `<img src=x onerror=...>` in a
  multiplayer mission would have run script in every viewer's browser. Airfield
  names, town names and unit ids are escaped too, for the same reason.

### Fixed

- **Wrong theatre identifiers hid airfields that had been read correctly.** DCS
  declares `MarianaIslands` and `SinaiMap`; the code used `Marianas` and `Sinai`,
  so the Marianas' 5 airfields were unreachable while `MarianaIslandsWWII`,
  `GermanyCW` and `SouthEastAsia` were missing from the list entirely. The ids
  are now DCS's own (15 theatres), the old spellings still resolve as aliases so
  a saved preference does not strand anyone on an empty map, and a stored theatre
  that no longer exists falls back to Caucasus. Covered by `TestDCSIdentifiers`.
- **An airfield DCS gives no position for is now reported.** 32 of the 101
  airfields come from a radio entry with no matching beacon, so they have no
  coordinates and cannot be placed (all 11 of Marianas WWII, 17 of Kola,
  Novorossiysk and Soganlug in the Caucasus). They were silently unmappable; the
  startup log now states how many per theatre, so the gap is visible rather than
  mysterious. **69 airfields are mappable, 101 are listed.**
- **Authentic DCS tiles could never load.** The tile handler appended ".png" to
  the `y` segment while Leaflet already sends it (the template is
  `/{z}/{x}/{y}.png`), so every request resolved to `11.png.png` and returned
  404. The bug was invisible until a tile actually existed on disk; it is now
  covered by `TestHandleTilesExtension`, which also checks that traversal stays
  refused.
- **A navigation aid could be shown with an impossible frequency.** DCS declares
  Ivalo's ILS at 212 MHz and Sas Al Nakheel's VOR at 128.925 MHz — outside the
  band those aids use, so no pilot could tune them. Frequencies are now validated
  against their band (ILS 108.10–111.95 MHz, VOR 108–117.95 MHz, NDB 190–1750
  kHz), and an out-of-band aid is dropped rather than displayed. 1 of 122 ILS
  entries was affected. A rejection is logged at startup so a data problem
  upstream stays visible instead of being swallowed.
- **Long chart file names overflowed the airfield card**, whose width could also
  exceed the map on a narrow window. Both are clamped now, with the name
  truncated by ellipsis.

### Upcoming

- Other features inspired by MizMap / MovingMap: BRA measurement, SAM circles,
  MIL-STD-2525C symbols

## [1.0.0-beta.3] — 2026-09-26

The manager becomes a **local companion**: it now reads DCS's own files. That
single change is what turned the airfields from a hand-transcribed dataset for one
map into the simulator's own truth for every installed map.

It is also the first release that **exercises `dcsmanager.exe` on Windows in CI** and
the first in which the aeronautical charts you already have on disk are readable
from the app.

Still a pre-release: the Lua scripts have not yet been run against a live DCS
session. See "Beta scope" in the previous entry for the standing caveat.

### Changed

- **Airfield data is no longer transcribed by hand for the Caucasus.** DCS is
  read first; the curated entry is only used to enrich it.
- **The manager is now local-only.** It runs on the same Windows machine as DCS.
  The Docker/Linux deployment is removed: `deploy/`, `.dockerignore`,
  `docs/deployment.md`, the `make docker` / `make docker-multiarch` targets and
  the `build.ps1 -Target docker` option are gone, along with every LAN-address and
  container-port caveat in the documentation. This is what makes the airfield
  reading possible: a local backend can read DCS's own files, and it removes the
  whole class of "wrong IP / firewall / port publishing" support questions.
- CI and the release workflow now build and smoke-test **`dcsmanager.exe` on Windows**
  rather than a Linux stand-in, so the artifact that is verified is the artifact
  users download.

### Added

- **Aeronautical charts viewer.** The approach plates, ground plans and
  procedure charts kept in `maps_dcs/` are indexed by name (file name, kind,
  runway) and listed on each airfield's data card: choose an airfield, see its
  charts, click one to read it full screen with zoom, and open it in a tab if
  wanted. 114 charts across 8 theatres are found in the current folder.
  - They are matched to an airfield by name, which is how the scans are named
    ("01_VAD_UG5X_Kobuleti.png", "NORWAY_LAKSELV-ILS-RWY34.jpg"), so the Kola
    instrument charts work without any extra dataset.
  - They are shown **as documents**, never overlaid: the scans are not
    georeferenced and are in a conic projection, so warping them onto the map
    would be wrong. A note in the viewer says so.
  - They are never shipped and never redistributed; the folder is local and each
    scan keeps its own licence. `DCSMANAGER_CHARTS_DIR` changes the folder.
  - The file endpoint serves only files present in the index, so a crafted URL
    cannot read anything else.
- **Aeronautical basemap style.** A fifth basemap, "Aeronautical", draws on the
  relief base — contour lines, shaded relief and land use, which is already
  close to a chart — and shows the airfields as permanent callouts (name, ICAO,
  Tower, TACAN, ILS) that appear as the map is zoomed in, so the view never turns
  into a pile of overlapping boxes. Selecting the style reveals the airfields,
  since that is what the style exists for.
  - It is a *chart-like base plus real aeronautical content*, not a scanned
    chart. No free, worldwide, key-less source of aeronautical tiles exists:
    OpenAIP requires an API key, open flightmaps has no public endpoint, VFRMAP
    covers the United States only, and DCS's own F10 imagery is Eagle Dynamics'
    copyrighted work — reading factual data from the installation is one thing,
    re-serving their map imagery is another. The reason is recorded in the
    `basemap` package comment so it does not have to be rediscovered.
  - A pale, washed-out rendering was tried first and had to be replaced: it made
    the map unreadable. The relief base needs only a slight calming, and the
    result is far more legible.
- **Airfields read from DCS itself.** Because the manager is local, it reads the
  simulator's own terrain files (`Mods/terrains/<map>/radio.lua` and
  `beacons.lua`) instead of relying on a hand-transcribed dataset. The two files
  share an identifier (`airfield22_0` = Batumi in both): the first gives the name
  and the ATC frequency, the second every navigation aid (TACAN, ILS, VOR, RSBN,
  NDB, PRMG) with real coordinates.
  - **101 airfields across 5 maps** instead of 21 on one, and *more complete*:
    DCS declares 6 TACAN and 13 ILS for the Caucasus where the curated dataset
    had 5 and 10.
  - The simulator's data is authoritative and updates with each patch; the
    bundled dataset remains as a fallback (and supplies the ICAO codes, runways,
    coalitions and charts that DCS does not expose, matched by proximity).
  - The Airfields tab shows where the data came from ("Read from DCS" vs
    "Bundled data").
- **Towns layer.** `Mods/terrains/<map>/map/towns.lua` holds thousands of
  geolocated settlements (1691 for the Caucasus, 385 for the Persian Gulf). They
  are exposed as `/api/towns` and can be drawn on the map, giving it context
  with no mission data at all.
- **`DCSMANAGER_SAVED_GAMES`** overrides the Saved Games folder. The DCS installation
  is located through the registry, with a fallback to the `Command line:` line of
  `Logs/dcs.log`.
- The Lua parser now understands the constructs DCS data files use: the gettext
  wrapper `_("…")` and bare enum constants (`BEACON_TYPE_TACAN`,
  `MODULATIONTYPE_AM`, `VHF_HI`). Without them, reading terrain data is
  impossible.
- **Theatre selector and map extent.** The live map now has a theatre selector
  (Caucasus, Syria, Nevada, Persian Gulf, Marianas, Sinai, Kola, Afghanistan,
  Iraq, Falklands, Normandy, The Channel). Choosing one frames the map on that
  map's bounding box, switches the airfield list to it, and offers a "Bounds"
  button that outlines the DCS map extent on the map. The choice is persisted.
- **Airfields on the map with their data.** An "Airfields" button reveals the
  theatre's airfields as markers; clicking one opens a data card (ICAO,
  coalition, coordinates, elevation, runway, Tower, TACAN, ILS). The card also
  has a "Show on the map" button, and clicking an airfield in the Airfields tab
  now switches to the map and focuses it.
- **Session source tracking (`live` / `test`) and `dcsmanager purge`.** A session
  recorded while the test tools are running is indistinguishable from a real
  flight, because those tools speak exactly the same protocol as DCS. Every
  mission now carries a `source`, detected automatically from the fixture
  callsigns (`DCSMANAGER_SOURCE` forces the verdict), and statistics exclude `test`
  sessions unless `?includeTest=1` is passed. New CLI and HTTP endpoints
  (`dcsmanager purge`, `DELETE /api/maintenance/purge`) remove sessions, which
  previously had no option short of deleting the database file.

### Fixed

- **The map (and its overlay cards) could be pushed off screen.** A wide header
  expanded the layout's grid column, so on narrow windows part of the map and the
  unit/airfield cards ended up outside the viewport. The header now wraps and the
  grid column is clamped.
- **Phantom empty missions.** The position tracker opened a "Session without
  mission" at its first tick, even when no unit had ever been reported, so an
  idle backend accumulated empty sessions in the UI. A mission is now created
  only once a real position has been sampled.
- **A session detected as simulated mid-flight stayed counted as real.** The
  tracker can open a mission before the first test packet arrives; the mission is
  now promoted to `test` on later ticks, and can never fall back to `live`.
- **Analytics could open on a simulated session.** The heatmap and trails default
  to the newest live mission instead of the newest mission overall.

## [1.0.0-beta.2] — 2026-09-26

Fixes a broken first run in beta.1: the downloaded `dcsmanager.exe` could not run
`install-lua`.

### Fixed

- **`install-lua` did not work from a downloaded binary.** The scripts had to sit
  in a `dcs-lua/` folder next to the executable, which a release archive never
  contains, so the documented `.\dcsmanager.exe install-lua` failed with
  "dcs-lua directory not found". The scripts are now **embedded in the binary**
  (generated by `tools/gen-lua-embed.mjs`), with the on-disk folder still taking
  precedence for development. Covered by `TestEmbeddedFallback`, and CI fails if
  `dcs-lua/` changes without regenerating the embedded copy.
- Path separator bug: `Hooks/dcsmanager.lua` resolved on Linux but `Hooks\dcsmanager.lua`
  failed on Windows. Relative paths are now normalised before lookup.

### Notes

- `v1.0.0-beta.1` is superseded. Its binary could not install the Lua scripts, so
  prefer `beta.2`. The tag is left in place rather than moved, since a published
  tag should stay immutable.

## [1.0.0-beta.1] — 2026-09-26

First public **pre-release**. The whole feature set is implemented and the whole
chain is tested, but the DCS-side scripts have not yet been exercised against a
live DCS installation (see "Beta scope" below).

**Bilingual release.** The whole project is in English, with French kept as a
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

- **License**: MIT (`LICENSE`), plus a non-affiliation notice for Eagle Dynamics
  in both READMEs.

- **Provenance note** for the aerodrome dataset
  (`backend/internal/aerodrome/data/README.md`), covering the source charts and
  the factual/database-right nature of the data.

### Changed

- Every French string, comment, log line, CLI message, docstring and documentation
  page has been translated to English: backend (all packages), Lua scripts, tools,
  CI workflow, Docker files, PowerShell scripts, examples.
- **Protocol values were deliberately left untouched** (`dcsmanager_*` config keys,
  JSON keys, DCS event names, coalition values, `optview_*`, theatre ids, basemap
  ids), so existing installations keep working.
- The `DCSMANAGER-BEGIN`/`DCSMANAGER-END` markers were translated **on both sides**
  (`install.go` and the Lua scripts) so they still match byte-for-byte.
- The fog-of-war banner is now translated client-side from the machine-readable
  `mode`, instead of displaying the backend's label.

### Fixed

- **Encoding corruption** in `docs/phase2-events.md`: the translation had turned
  every letter `i` into `n` (`internal` -> `nnternal`, `with` -> `wnth`). The page
  was rewritten and a scan confirmed no other file was affected.
- `frontend/index.html` declared `lang="fr"` while the app now defaults to English.
- CI and Docker comments were still French.

### Beta scope

Verified:

- Live map from UDP telemetry through the Go backend to the browser (SSE).
- Events, players and chat over TCP, persisted to SQLite and replayed on restart.
- Debrief parsing against a **real** `debrief.log`, including chunked transfer.
- Advanced statistics, analytics (heatmap, trails, sorties) and the Caucasus
  airfield dataset.
- Fog-of-war filtering driven by DCS's official `optionsView` values.
- Single self-contained binary and the Docker image; CI green.

Not yet verified against a live DCS session:

- `Scripts/Export.lua` and `Scripts/Hooks/dcsmanager.lua` running inside DCS.
- `Sim.getMissionOptions()` on a real mission (values taken from DCS's own
  `optionsDb.lua`).
- Fog-of-war behaviour with real units and coalitions.
- Debrief transfer from an actual end-of-mission.
- LuaSocket availability in the GUI/hooks Lua state.

Feedback and bug reports are welcome via GitHub Issues.

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
  - `internal/config`: `DCSMANAGER_REVEAL_ALL_UNITS` (default `false`).

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

- **CLI (`dcsmanager`)**
  - `install-lua`: installs/merges the scripts into Saved Games;
  - `uninstall-lua`: removes the block and `Hooks/dcsmanager.lua`;
  - `status`: `installed` / `outdated` / `missing` per file;
  - `version` / `help`.
  - Automatic detection of `Saved Games` (`DCS.openbeta` takes priority) and of the
    `dcs-lua` folder; `--saved-games`, `--lua-dir`, `--dry-run`.

- **Lua injector (`internal/install`)**
  - **Never replaces** an existing `Export.lua` (Tacview, SRS, DCS-BIOS…):
    merges a block delimited by `>>> DCSMANAGER-BEGIN >>>` / `<<< DCSMANAGER-END <<<`.
  - **Timestamped backup** before any modification.
  - **Idempotent**: a second run updates the block in place.
  - Markers added in `Export.lua` and `Hooks/dcsmanager.lua`.

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
  - Configuration: `DCSMANAGER_TRACK_INTERVAL`, `DCSMANAGER_TRACK_GRACE`,
    `DCSMANAGER_TRACK_RETENTION`.
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
  - `Hooks/dcsmanager.lua`: resolution of the **aircraft type** per player via
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
  - `Hooks/dcsmanager.lua`: reads `debrief.log` at the end of the mission and sends it in 32 KB
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
  - `internal/config`: `DCSMANAGER_DB_ENABLED`.

- **DCS scripts**
  - `Hooks/dcsmanager.lua`: TCP sending of events (`onGameEvent`), chat
    (`onChatMessage`), players and their statistics (`net.get_player_list`,
    `net.get_player_info`, `net.get_stat`), and mission transitions. Persistent
    connection, automatic reconnection, calls protected by `pcall`, never blocking.
  - `Config/dcsmanager.cfg`: `dcsmanager_players_interval`.

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
  - `internal/config`: new options (`DCSMANAGER_UNIT_TTL`, `DCSMANAGER_TILES_DIR`,
    `DCSMANAGER_BASEMAP_URL`, `DCSMANAGER_CATEGORIES`, `DCSMANAGER_MAX_UNITS`).

- **DCS scripts**
  - `Export.lua`: export of **all world objects** in addition to the player, filterable
    by radius (`dcsmanager_world_radius`), capped (`dcsmanager_max_objects`), selectable
    coalitions; defensive access to the `LoGet*`/`Export.*` functions.
  - `Config/dcsmanager.cfg`: new documented options.

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
  - `Config/dcsmanager.cfg`: configuration template (backend IP, ports, send interval).
  - `Export.lua`: export of the player's position to the backend over UDP/JSON,
    sampled once per second via `LuaExportActivityNextEvent` (no impact
    on simulator performance).

- **Go backend (`backend/`)**
  - `internal/config`: configuration via environment variables (`DCSMANAGER_*`) with defaults.
  - `internal/udp`: UDP receiver decoding the JSON positions.
  - `internal/state`: in-memory store of units (with expiry).
  - `internal/api`: HTTP server (REST `GET /api/state`, **Server-Sent Events**
    stream `/api/events`) + serving of the embedded web interface.
  - `cmd/dcsmanager/main.go`: entry point assembling the building blocks.

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
