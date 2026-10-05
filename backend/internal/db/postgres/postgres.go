// Package postgres is the PostgreSQL implementation of the db.Store contract.
//
// It mirrors the pure-Go SQLite store (package db) method for method, so the
// rest of the manager can switch engines without knowing: the consumers depend
// on db.Store, not on a concrete type. Unlike the SQLite path, the driver is not
// pure Go, but pgx is used through database/sql (pgx/stdlib) because the
// analytics package needs *sql.Rows/*sql.Row.
//
// It is a NEW integration (no legacy database to migrate): the schema is created
// from scratch, and the only "migration" kept is the additive source column, for
// symmetry with SQLite.
package postgres

import (
	"database/sql"
	"strconv"
	"strings"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"

	"dcsmanager/internal/db"
)

// DB wraps the PostgreSQL handle.
type DB struct {
	sql *sql.DB
	// openMissionMu serialises the get-or-create of the open mission, mirroring
	// the SQLite store (see db.go for why: concurrent callers must not each open
	// a mission).
	openMissionMu sync.Mutex
}

// Open opens (and migrates) the database at the given DSN, e.g.
// "postgres://user:pass@host:5432/dcsmanager?sslmode=disable".
func Open(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	d := &DB{sql: sqlDB}
	if err := d.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the underlying handle.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// Query runs a read query, rebinding `?` placeholders to PostgreSQL's `$n`.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.sql.Query(rebind(query), args...)
}

// QueryRow runs a single-row read query, rebinding `?` placeholders.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.sql.QueryRow(rebind(query), args...)
}

// Compile-time proof that the PostgreSQL store satisfies the db.Store contract,
// so it can be dropped in wherever the SQLite store is used.
var _ db.Store = (*DB)(nil)

// rebind rewrites the `?` placeholders used throughout the codebase into the
// numbered form PostgreSQL expects ($1, $2, …). No query in the project contains
// a literal `?` outside a placeholder, so a straight scan is safe.
func rebind(query string) string {
	if !strings.ContainsRune(query, '?') {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

// lastInsertID reads the id returned by an `INSERT … RETURNING id`, replacing
// SQLite's LastInsertId (which the PostgreSQL driver does not implement).
func lastInsertID(row *sql.Row) (int64, error) {
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// nullInt maps 0 to NULL, matching the SQLite store's convention for optional
// foreign keys (a mission id of 0 means "no mission").
func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// nullStr maps an empty string to NULL.
func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// boolInt stores a bool as the smallint (0/1) the schema uses.
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// schema is the PostgreSQL translation of the SQLite schema. Differences from
// db.go's version: IDENTITY instead of AUTOINCREMENT; bigint for millisecond
// timestamps and ids; double precision for coordinates (NOT `real`, which is
// only 32-bit in PostgreSQL); an expression+partial index for "one open
// mission", since PostgreSQL rejects a constant index expression.
const schema = `
CREATE TABLE IF NOT EXISTS missions (
	id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name       text NOT NULL,
	theatre    text,
	source     text NOT NULL DEFAULT 'live',
	started_at bigint NOT NULL,
	ended_at   bigint,
	winner     text
);
CREATE INDEX IF NOT EXISTS idx_missions_source ON missions(source);

CREATE TABLE IF NOT EXISTS events (
	id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	event   text NOT NULL,
	args    text,
	detail  text,
	t       double precision,
	real_ts bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_mission ON events(mission_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);

CREATE TABLE IF NOT EXISTS chat (
	id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	"from"  text NOT NULL,
	message text NOT NULL,
	real_ts bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_mission ON chat(mission_id);

CREATE TABLE IF NOT EXISTS players (
	id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	ucid      text,
	name      text NOT NULL,
	first_seen bigint NOT NULL,
	last_seen  bigint NOT NULL,
	UNIQUE(ucid, name)
);
CREATE INDEX IF NOT EXISTS idx_players_ucid ON players(ucid);

CREATE TABLE IF NOT EXISTS player_stats (
	id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	player_id  bigint REFERENCES players(id),
	dcs_player_id bigint,
	side       integer,
	slot       text,
	unit_type  text,
	ping       integer,
	crashes    integer,
	kills_car  integer,
	kills_air  integer,
	kills_ship integer,
	score      integer,
	landings   integer,
	ejects     integer,
	real_ts    bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_stats_mission ON player_stats(mission_id);
CREATE INDEX IF NOT EXISTS idx_stats_player ON player_stats(player_id);
CREATE INDEX IF NOT EXISTS idx_stats_dcs_player ON player_stats(mission_id, dcs_player_id);

CREATE TABLE IF NOT EXISTS meta (
	key   text PRIMARY KEY,
	value text
);

CREATE TABLE IF NOT EXISTS debriefs (
	id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	mission    text,
	theatre    text,
	raw        text,
	parsed     text NOT NULL,
	size       integer NOT NULL,
	created_at bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_debriefs_mission ON debriefs(mission_id);

CREATE TABLE IF NOT EXISTS track_positions (
	id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	unit_id    text NOT NULL,
	name       text,
	type       text,
	category   text,
	coalition  text,
	lat        double precision NOT NULL,
	lng        double precision NOT NULL,
	alt        double precision,
	heading    double precision,
	speed      double precision,
	g          double precision,
	ownship    smallint NOT NULL DEFAULT 0,
	real_ts    bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_track_mission ON track_positions(mission_id);
CREATE INDEX IF NOT EXISTS idx_track_unit ON track_positions(mission_id, unit_id);
CREATE INDEX IF NOT EXISTS idx_track_ts ON track_positions(real_ts);

CREATE TABLE IF NOT EXISTS losses (
	id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	mission_id bigint REFERENCES missions(id),
	unit_id    text NOT NULL,
	type       text,
	category   text,
	coalition  text,
	lat        double precision NOT NULL,
	lng        double precision NOT NULL,
	alt        double precision,
	ownship    smallint NOT NULL DEFAULT 0,
	real_ts    bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_losses_mission ON losses(mission_id);
CREATE INDEX IF NOT EXISTS idx_losses_ts ON losses(real_ts);
`

func (d *DB) migrate() error {
	if _, err := d.sql.Exec(schema); err != nil {
		return err
	}
	// Additive migration kept for symmetry with the SQLite store.
	if _, err := d.sql.Exec(`ALTER TABLE missions ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'live'`); err != nil {
		return err
	}
	return d.ensureSingleOpenMissionIndex()
}

// ensureSingleOpenMissionIndex enforces "at most one open mission". PostgreSQL
// cannot index a constant expression, so the index is on `(ended_at IS NULL)`,
// which is true for every row the partial WHERE keeps, giving the same effect.
// Older duplicates are reconciled before the index is created.
func (d *DB) ensureSingleOpenMissionIndex() error {
	const create = `CREATE UNIQUE INDEX IF NOT EXISTS idx_missions_one_open
		ON missions ((ended_at IS NULL)) WHERE ended_at IS NULL`
	if _, err := d.sql.Exec(create); err == nil {
		return nil
	}
	if _, err := d.sql.Exec(`
		UPDATE missions SET ended_at = started_at
		WHERE ended_at IS NULL
		  AND id <> (SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1)`); err != nil {
		return err
	}
	_, err := d.sql.Exec(create)
	return err
}
