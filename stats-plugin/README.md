# DCS Manager — Statistics plugin

An **optional**, separate statistics dashboard for DCS Manager. It reads the
manager's **PostgreSQL** database directly and serves its own read-only web app.

The manager is the single writer; the plugin is a reader. There is no HTTP call
between them, no mirror, no polling: **PostgreSQL is the shared library**. The
plugin reads the manager's read-only views (`v_stats_*`), not its tables, so the
internal schema can evolve without breaking the plugin.

```
dcsmanager.exe (DCSMANAGER_DB_DRIVER=postgres) ──► PostgreSQL ◄── stats-web (read-only)
```

The manager can therefore stay bound to `127.0.0.1` — nothing needs to be
exposed. Full design: [`../docs/stats-plugin.md`](../docs/stats-plugin.md).

## Requirement: the manager must use PostgreSQL

SQLite is a local file; another machine (or container) cannot read it. So the
shared-library model applies only when the manager writes to PostgreSQL:

```powershell
# On the DCS machine
$env:DCSMANAGER_DB_DRIVER = "postgres"
$env:DCSMANAGER_DB_DSN    = "postgres://dcs:dcs@localhost:5432/dcsmanager?sslmode=disable"
.\dcsmanager.exe
```

See [`../docs/database-backends.md`](../docs/database-backends.md).

## What it shows

- **Overview** — missions, players, events, kills, deaths, crashes, ejections,
  friendly fire, coalition balance.
- **Pilots** — score, kills (air/ground/ship), deaths, K/D, landings, crashes,
  ejections, average ping, career-wide by UCID.
- **Weapons** — kills and friendly fire per weapon, victim and platform breadth.
- **Aircraft & vehicles** — exact DCS type: kills, losses, sorties, K/D.
- **Network** — average/max ping per player.
- **Missions** — the manager's mission list, with CSV export.
- **Trends** — events per day, computed from the manager's own event timestamps
  (exact, unlike a sampled series).
- **Export** — missions, pilots and event series as CSV or JSON.

## Read-only, enforced by PostgreSQL

The pool opens with `default_transaction_read_only=on`, so no query can modify
the manager's data. For defence in depth, point the plugin at a **SELECT-only
role**:

```sql
CREATE ROLE stats_reader LOGIN PASSWORD 'change-me';
GRANT CONNECT ON DATABASE dcsmanager TO stats_reader;
GRANT USAGE   ON SCHEMA public      TO stats_reader;
GRANT SELECT  ON ALL TABLES IN SCHEMA public TO stats_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO stats_reader;
```

Then use `postgres://stats_reader:change-me@host:5432/dcsmanager?sslmode=disable`.
Grant SELECT only on the views, if you prefer the reader not to see the raw
tables at all:

```sql
GRANT SELECT ON ALL TABLES IN SCHEMA public TO stats_reader; -- includes views
```

## Quick start

```powershell
# PostgreSQL (Docker)
docker compose -f stats-plugin/docker-compose.yml up -d postgres

# The plugin
cd stats-plugin
$env:MANAGER_DATABASE_URL = "postgres://stats_reader:change-me@localhost:5432/dcsmanager?sslmode=disable"
go run .
# dashboard: http://localhost:8090
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `MANAGER_DATABASE_URL` | *(required)* | The manager's PostgreSQL DSN. Read-only. |
| `PLUGIN_LISTEN_ADDR` | `:8090` | Where the plugin's dashboard is served |
| `QUERY_TIMEOUT` | `15s` | Timeout for a single query |
| `INCLUDE_TEST` | `false` | Include simulated (`test`) missions |
| `PLUGIN_AUTH_TOKEN` | *(empty)* | When set, protects the plugin's API and dashboard |

See [`.env.example`](.env.example).

## API

- `GET /api/plugin/health` — row counts and mode
- `GET /api/plugin/summary` — every aggregate in one call
- `GET /api/plugin/overview`
- `GET /api/plugin/{pilots,weapons,engines,network}`
- `GET /api/plugin/series?event=kill&days=30`
- `GET /api/plugin/missions?limit=200`
- `GET /api/plugin/export?type=missions|pilots|series&format=csv|json`

## Build

```powershell
go build -o stats-web.exe .
docker build -f stats-plugin/Dockerfile -t dcsmanager-stats stats-plugin   # from the repo root
```

## Tests

```powershell
go test ./...   # unit tests; no database needed
```

The SQL aggregations (latest-snapshot per mission/player, DCS id resolution,
the test-exclusion policy, event series, read-only enforcement) are covered by
an **integration suite that only runs with a database**:

```powershell
$env:PLUGIN_TEST_DATABASE_URL = "postgres://dcs:dcs@localhost:5432/stats_test?sslmode=disable"
go test ./...
```

Point it at a **throwaway** database: the suite recreates the manager's tables.
This runs in CI (`.github/workflows/ci.yml`) against a PostgreSQL service.
