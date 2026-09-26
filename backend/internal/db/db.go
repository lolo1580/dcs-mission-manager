// Package db is the durable store of the manager: mission history, game events,
// chat and per-player statistics. It uses pure-Go SQLite so the binary stays
// CGO-free and cross-compiles cleanly (Windows .exe and Docker Linux).
package db

import (
	"database/sql"
	"fmt"
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
	-- source separates real DCS sessions ("live") from simulated ones
	-- ("test"). Test data is kept but excluded from statistics by default, so a
	-- development session can never silently flatter a real career.
	source     TEXT NOT NULL DEFAULT 'live',
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
	dcs_player_id INTEGER,
	side       INTEGER,
	slot       TEXT,
	unit_type  TEXT,
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
CREATE INDEX IF NOT EXISTS idx_stats_dcs_player ON player_stats(mission_id, dcs_player_id);

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

-- Position and telemetry samples over time, for trails and heatmaps.
CREATE TABLE IF NOT EXISTS track_positions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	unit_id    TEXT NOT NULL,
	name       TEXT,
	type       TEXT,
	category   TEXT,
	coalition  TEXT,
	lat        REAL NOT NULL,
	lng        REAL NOT NULL,
	alt        REAL,
	heading    REAL,
	speed      REAL,
	g          REAL,
	ownship    INTEGER NOT NULL DEFAULT 0,
	real_ts    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_track_mission ON track_positions(mission_id);
CREATE INDEX IF NOT EXISTS idx_track_unit ON track_positions(mission_id, unit_id);
CREATE INDEX IF NOT EXISTS idx_track_ts ON track_positions(real_ts);

-- Units that vanished from the world (presumed destroyed/despawned).
CREATE TABLE IF NOT EXISTS losses (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	unit_id    TEXT NOT NULL,
	type       TEXT,
	category   TEXT,
	coalition  TEXT,
	lat        REAL NOT NULL,
	lng        REAL NOT NULL,
	alt        REAL,
	ownship    INTEGER NOT NULL DEFAULT 0,
	real_ts    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_losses_mission ON losses(mission_id);
CREATE INDEX IF NOT EXISTS idx_losses_ts ON losses(real_ts);
`

	// Indexes are created after the additive migrations, because an index may
	// reference a column that only exists once a migration has added it (which
	// is the case for idx_missions_source on databases predating the column).
	const indexes = `
CREATE INDEX IF NOT EXISTS idx_missions_source ON missions(source);
`

	if _, err := d.sql.Exec(schema); err != nil {
		return err
	}
	if err := d.migrations(); err != nil {
		return err
	}
	_, err := d.sql.Exec(indexes)
	return err
}

// migrations applies additive schema changes to databases created by an older
// version. `CREATE TABLE IF NOT EXISTS` never alters an existing table, so a
// database written before a column existed must be upgraded here.
func (d *DB) migrations() error {
	if err := d.ensureColumn("missions", "source", "TEXT NOT NULL DEFAULT 'live'"); err != nil {
		return err
	}
	return nil
}

// ensureColumn adds a column to a table when it is missing. It is a no-op on
// databases that already have it, which makes it safe to run on every start.
func (d *DB) ensureColumn(table, column, definition string) error {
	rows, err := d.sql.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if found {
		return nil
	}
	_, err = d.sql.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

// SQL exposes the raw handle for packages that need it (stats queries).
func (d *DB) SQL() *sql.DB {
	return d.sql
}
