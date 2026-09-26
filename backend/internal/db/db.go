// Package db is the durable store of the manager: mission history, game events,
// chat and per-player statistics. It uses pure-Go SQLite so the binary stays
// CGO-free and cross-compiles cleanly (Windows .exe and Docker Linux).
package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite handle.
type DB struct {
	sql *sql.DB
}

// Open opens (and migrates) the database at path. Parent directories are
// created as needed.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	// WAL improves concurrent read/write behaviour; busy_timeout avoids
	// spurious "database is locked" errors.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
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

func (d *DB) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS missions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	theatre    TEXT,
	started_at INTEGER NOT NULL,
	ended_at   INTEGER,
	winner     TEXT
);

CREATE TABLE IF NOT EXISTS events (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	event   TEXT NOT NULL,
	args    TEXT,
	detail  TEXT,
	t       REAL,
	real_ts INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_mission ON events(mission_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);

CREATE TABLE IF NOT EXISTS chat (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	"from"  TEXT NOT NULL,
	message TEXT NOT NULL,
	real_ts INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_mission ON chat(mission_id);

CREATE TABLE IF NOT EXISTS players (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	ucid      TEXT,
	name      TEXT NOT NULL,
	first_seen INTEGER NOT NULL,
	last_seen  INTEGER NOT NULL,
	UNIQUE(ucid, name)
);
CREATE INDEX IF NOT EXISTS idx_players_ucid ON players(ucid);

CREATE TABLE IF NOT EXISTS player_stats (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	player_id  INTEGER REFERENCES players(id),
	side       INTEGER,
	slot       TEXT,
	ping       INTEGER,
	crashes    INTEGER,
	kills_car  INTEGER,
	kills_air  INTEGER,
	kills_ship INTEGER,
	score      INTEGER,
	landings   INTEGER,
	ejects     INTEGER,
	real_ts    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_stats_mission ON player_stats(mission_id);
CREATE INDEX IF NOT EXISTS idx_stats_player ON player_stats(player_id);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT
);

CREATE TABLE IF NOT EXISTS debriefs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	mission    TEXT,
	theatre    TEXT,
	raw        TEXT,
	parsed     TEXT NOT NULL,
	size       INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_debriefs_mission ON debriefs(mission_id);
`
	_, err := d.sql.Exec(schema)
	return err
}

// SQL exposes the raw handle for packages that need it (stats queries).
func (d *DB) SQL() *sql.DB {
	return d.sql
}
