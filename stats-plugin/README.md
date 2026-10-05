# DCS Manager — Statistics plugin (PoC)

An **optional**, separate statistics service for DCS Manager. It is an external
consumer of the manager's REST API: it polls `/api/stats/*`, stores timestamped
snapshots in **PostgreSQL**, and serves its own read-only dashboard with
**trends over time** — something the manager itself cannot show, because it keeps
no history.

The manager is not modified, not containerised, and is unaware the plugin exists.
Full design: [`../docs/stats-plugin.md`](../docs/stats-plugin.md).

```
dcsmanager.exe (SQLite, unchanged) ──REST──► stats-web (this plugin) ──► PostgreSQL
```

## What the PoC does (phases P0–P4)

- polls `overview`, `pilots`, `weapons`, `engines`, `network` for one or more
  scopes (`career`, optionally `mission`);
- stores each payload as a `jsonb` snapshot, skipping byte-identical repeats;
- **mirrors events, chat and missions incrementally** through the manager's
  `?sinceId=` endpoint, with monotonic cursors, so the full history is kept;
- **multi-manager**: every row carries an `instance` (from `MANAGER_NAME`), so
  several plugin instances can share one PostgreSQL database without mixing;
- **optional auth**: `PLUGIN_AUTH_TOKEN` protects the plugin's API and dashboard
  (Bearer header, or `?token=` which sets a cookie);
- **exports**: CSV/JSON via `GET /api/plugin/export?type=…&format=…`;
- records the outcome of every sync pass;
- serves:
  - `GET /api/plugin/health` — status, manager reachability, counts, instances
  - `GET /api/plugin/instances` — managers present in the database
  - `GET /api/plugin/summary` — latest snapshot of each kind
  - `GET /api/plugin/latest?kind=pilots&scope=career` — any latest snapshot
  - `GET /api/plugin/series?metric=kills&scope=career` — a time series
  - `GET /api/plugin/missions` — mirrored missions
  - `GET /api/plugin/export?type=events|missions|series&format=csv|json`
  - `/api/plugin/{pilots,weapons,engines,network}` — convenience routes
  - `/` — a dashboard with tabs (Overview, Pilots, Weapons, Aircraft & vehicles,
    Network, Missions, Trends, Export), sortable columns and text filters

The incremental mirror needs the manager's additive `?sinceId=` routes
(`GET /api/history/events|chat|missions`).

## Quick start — Postgres in Docker, plugin locally (recommended)

Keeps the manager's API on `127.0.0.1`, which is the safe option because the API
has no authentication.

```powershell
# 1. Start PostgreSQL
docker compose -f stats-plugin/docker-compose.yml up -d postgres

# 2. Run the plugin against the local manager
cd stats-plugin
$env:DATABASE_URL    = "postgres://dcs:dcs@localhost:5432/stats?sslmode=disable"
$env:DCSMANAGER_URL  = "http://127.0.0.1:8080"
go run .
```

Open <http://localhost:8090>.

## Quick start — everything in Docker

> A container cannot reach the Windows host's `127.0.0.1`. The manager must be
> started with `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080`, which exposes an
> **unauthenticated** API (including the destructive purge endpoint). Prefer the
> local-plugin topology above unless you understand the trade-off — see
> `docs/stats-plugin.md` §8.

```powershell
$env:DCSMANAGER_HTTP_ADDR = "0.0.0.0:8080"
.\dcsmanager.exe serve
docker compose -f stats-plugin/docker-compose.yml up --build
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://dcs:dcs@localhost:5432/stats?sslmode=disable` | PostgreSQL connection string |
| `DCSMANAGER_URL` | `http://127.0.0.1:8080` | Base URL of the manager's REST API |
| `PLUGIN_LISTEN_ADDR` | `:8090` | Where the plugin's dashboard is served |
| `SYNC_INTERVAL` | `5m` | Poll interval (Go duration, or seconds as a number) |
| `SYNC_TIMEOUT` | `10s` | Timeout for a single API call |
| `INCLUDE_TEST` | `false` | Include simulated (`test`) sessions |
| `SYNC_SCOPES` | `career` | `career`, `mission`, or `career,mission` |
| `MIRROR_EVENTS` | `true` | Mirror events, chat and missions incrementally via `?sinceId=` |
| `MIRROR_BATCH` | `1000` | Rows fetched per incremental request |
| `MANAGER_NAME` | `local` | Identifies this manager in a shared database |
| `PLUGIN_AUTH_TOKEN` | *(empty)* | When set, protects the plugin's API and dashboard |

## Tests

```powershell
go test ./...                       # unit tests (no database needed)
```

The PostgreSQL SQL (jsonb casts, `ON CONFLICT`, `DISTINCT ON`, batch upserts,
cursors) is covered by an **integration suite that only runs when a database is
provided**, so the default `go test` stays green without Postgres:

```powershell
$env:PLUGIN_TEST_DATABASE_URL = "postgres://dcs:dcs@localhost:5432/stats_test?sslmode=disable"
go test ./...
```

Point it at a **throwaway** database: the suite drops and recreates the plugin
tables.

## Notes on the schema (P4)

P4 added an `instance` column to every table and a `missions` table. Because the
plugin's data is entirely derived from the manager, the simplest upgrade from an
earlier PoC database is to start clean:

```sql
DROP TABLE IF EXISTS stat_snapshots, sync_runs, sync_cursor, events, chat, missions;
```

The embedded schema recreates and migrates at startup. A proper additive
migration would be worth it only once this leaves PoC status.

See [`.env.example`](.env.example).

## Build

```powershell
# locally
go build -o stats-web.exe .

# container image (from the repository root)
docker build -f stats-plugin/Dockerfile -t dcsmanager-stats stats-plugin
```

`CGO_ENABLED=0` still holds: the plugin uses `pgx/v5`, a pure-Go PostgreSQL driver.

## Notes

- The plugin is **read-only** on the manager and never writes to it.
- PostgreSQL is the only required infrastructure; the manager keeps SQLite. See
  [`database-backends.md`](../docs/database-backends.md) for the alternative
  (making the manager itself support PostgreSQL).
- The `stat_snapshots` table grows by one row per (kind, scope) when the payload
  changes; identical polls are skipped.
