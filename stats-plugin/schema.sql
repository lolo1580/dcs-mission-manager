-- Schema of the DCS Manager statistics plugin.
--
-- This database is INDEPENDENT from the manager's SQLite file: the plugin only
-- consumes the manager's REST API and stores what it reads here.
--
-- MULTI-MANAGER: several plugin instances may share this database, one per
-- manager. The `instance` column identifies the manager (its MANAGER_NAME), so
-- every table is scoped by it and instances never mix. A single plugin process
-- only ever writes its own instance's rows.
--
-- NOTE: this schema changed in phase P4 (added `instance`, added `missions`).
-- The plugin's data is entirely derived from the manager, so the simplest
-- upgrade is to drop and recreate the tables (see stats-plugin/README.md).

CREATE TABLE IF NOT EXISTS stat_snapshots (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instance     text NOT NULL DEFAULT 'local',
    kind         text NOT NULL,              -- overview | pilots | weapons | engines | network
    scope        text NOT NULL,              -- career | mission
    include_test boolean NOT NULL DEFAULT false,
    payload      jsonb NOT NULL,
    payload_hash text NOT NULL,
    captured_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_snap_lookup
    ON stat_snapshots(instance, kind, scope, captured_at DESC);

-- One row per sync pass, per instance, so health can report whether polling is
-- healthy and when it last ran.
CREATE TABLE IF NOT EXISTS sync_runs (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instance text NOT NULL DEFAULT 'local',
    at       timestamptz NOT NULL DEFAULT now(),
    ok       boolean NOT NULL,
    error    text
);
CREATE INDEX IF NOT EXISTS idx_runs_instance ON sync_runs(instance, at DESC);

-- Incremental cursors: the largest event/chat/mission id already mirrored, per
-- instance. A monotonic GREATEST on update means a late, smaller batch can never
-- rewind a cursor.
CREATE TABLE IF NOT EXISTS sync_cursor (
    instance   text NOT NULL DEFAULT 'local',
    name       text NOT NULL,                -- events | chat | missions
    last_id    bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (instance, name)
);

-- Mirror of the manager's events. ids are unique per instance, hence the
-- composite unique index rather than a single-column primary key.
CREATE TABLE IF NOT EXISTS events (
    instance  text NOT NULL DEFAULT 'local',
    id        bigint NOT NULL,
    event     text NOT NULL,
    args      jsonb,
    detail    jsonb,
    t         double precision,
    real_ts   bigint NOT NULL,
    synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS events_instance_id  ON events(instance, id);
CREATE INDEX IF NOT EXISTS idx_events_kind            ON events(instance, event);
CREATE INDEX IF NOT EXISTS idx_events_ts              ON events(instance, real_ts);

CREATE TABLE IF NOT EXISTS chat (
    instance  text NOT NULL DEFAULT 'local',
    id        bigint NOT NULL,
    "from"    text NOT NULL,
    message   text NOT NULL,
    real_ts   bigint NOT NULL,
    synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS chat_instance_id ON chat(instance, id);

-- Mirror of the manager's missions. `source` is the mission's own tag
-- (live/test); `instance` identifies which manager it came from.
CREATE TABLE IF NOT EXISTS missions (
    instance  text NOT NULL DEFAULT 'local',
    id        bigint NOT NULL,
    name      text NOT NULL,
    theatre   text,
    source    text NOT NULL DEFAULT 'live',
    started_at bigint NOT NULL,
    ended_at  bigint,
    winner    text,
    synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS missions_instance_id ON missions(instance, id);
CREATE INDEX IF NOT EXISTS idx_missions_instance_started ON missions(instance, started_at DESC);
