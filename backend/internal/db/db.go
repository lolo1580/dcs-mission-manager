// Package db is the durable store of the manager: mission history, game events,
// chat and per-player statistics. It uses pure-Go SQLite, so the binary stays
// CGO-free and builds without a C toolchain.
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"dcsmanager/internal/idgen"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite handle.
type DB struct {
	sql *sql.DB
	// openMissionMu serialises the get-or-create of the open mission. Several
	// components (the ingest writer, the tracker, the API) may ask for the open
	// mission at once; without this, they could all see "no open mission" and
	// each insert one, leaving several open missions at a time. It is held only
	// around the short get-or-create, never around I/O of other kinds.
	openMissionMu sync.Mutex
	// syncRevisionMu serialises the server-wide document revision counter.
	syncRevisionMu sync.Mutex
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
	if err := d.BackfillDebriefStats(); err != nil {
		log.Printf("db: debrief statistics backfill: %v", err)
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
	winner     TEXT,
	-- Import metadata: a mission uploaded by a desktop is identified by its
	-- owner, the desktop that produced it and the desktop-side mission id.
	owner_id    INTEGER,
	client_id   TEXT,
	external_id INTEGER
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
	country    TEXT,
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

-- Saved flight plans. The route is a JSON array of waypoints: a plan is edited
-- and saved as one document, so there is no benefit in a separate table.
CREATE TABLE IF NOT EXISTS flight_plans (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	uid            TEXT NOT NULL,
	revision       INTEGER NOT NULL DEFAULT 1,
	owner_id       INTEGER,
	visibility     TEXT NOT NULL DEFAULT 'private',
	deleted_at     INTEGER,
	sync_revision  INTEGER NOT NULL DEFAULT 0,
	dirty          INTEGER NOT NULL DEFAULT 1,
	name           TEXT NOT NULL,
	aircraft       TEXT,
	departure      TEXT,
	arrival        TEXT,
	cruise_alt_ft  INTEGER NOT NULL DEFAULT 0,
	cruise_tas_kt  INTEGER NOT NULL DEFAULT 0,
	fuel_burn_pph  REAL NOT NULL DEFAULT 0,
	reserve_min    INTEGER NOT NULL DEFAULT 0,
	wind_from_deg  INTEGER NOT NULL DEFAULT 0,
	wind_kt        INTEGER NOT NULL DEFAULT 0,
	flight_rules   TEXT NOT NULL DEFAULT 'VFR',
	notes          TEXT,
	route          TEXT NOT NULL DEFAULT '[]',
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS debriefs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	mission_id INTEGER REFERENCES missions(id),
	mission    TEXT,
	theatre    TEXT,
	raw        TEXT,
	parsed     TEXT NOT NULL,
	size       INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	external_id INTEGER
);
CREATE INDEX IF NOT EXISTS idx_debriefs_mission ON debriefs(mission_id);

-- Checklists are edited as one document, like flight plans. Progress is kept by
-- each device, so it is deliberately not stored here.
CREATE TABLE IF NOT EXISTS checklists (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	uid         TEXT NOT NULL UNIQUE,
	name        TEXT NOT NULL,
	aircraft    TEXT,
	phase       TEXT NOT NULL,
	description TEXT,
	source      TEXT NOT NULL DEFAULT 'user',
	visibility  TEXT NOT NULL DEFAULT 'private',
	revision    INTEGER NOT NULL DEFAULT 1,
	owner_id    INTEGER,
	deleted_at  INTEGER,
	sync_revision INTEGER NOT NULL DEFAULT 0,
	dirty       INTEGER NOT NULL DEFAULT 1,
	items       TEXT NOT NULL DEFAULT '[]',
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_checklists_updated ON checklists(updated_at);

-- Server-wide revision counter for document synchronisation.
CREATE TABLE IF NOT EXISTS server_meta (
	key   TEXT PRIMARY KEY,
	value INTEGER NOT NULL DEFAULT 0
);

-- Versions rejected by last-write-wins, kept for inspection.
CREATE TABLE IF NOT EXISTS conflict_copies (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	kind       TEXT NOT NULL,
	uid        TEXT NOT NULL,
	owner_id   INTEGER,
	revision   INTEGER,
	updated_at INTEGER,
	payload    TEXT NOT NULL,
	reason     TEXT,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_conflict_owner ON conflict_copies(owner_id);

-- Accounts, sessions and personal tokens. They are unused by the desktop
-- binary, but keeping one schema means a desktop database can become a server
-- database without a conversion step.
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	display_name  TEXT,
	email         TEXT,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL DEFAULT 'member',
	active        INTEGER NOT NULL DEFAULT 1,
	dcs_names     TEXT NOT NULL DEFAULT '[]',
	created_at    INTEGER NOT NULL,
	last_login_at INTEGER
);

CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	user_agent TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS password_resets (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	used_at    INTEGER
);
CREATE INDEX IF NOT EXISTS idx_password_resets_user ON password_resets(user_id);
CREATE INDEX IF NOT EXISTS idx_password_resets_expiry ON password_resets(expires_at);

CREATE TABLE IF NOT EXISTS api_tokens (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name         TEXT NOT NULL,
	token_hash   TEXT NOT NULL UNIQUE,
	created_at   INTEGER NOT NULL,
	last_used_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens(user_id);

CREATE TABLE IF NOT EXISTS audit_log (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
	action  TEXT NOT NULL,
	detail  TEXT,
	real_ts INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(real_ts);

-- Position and telemetry samples over time.
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
CREATE INDEX IF NOT EXISTS idx_stats_player_ts ON player_stats(player_id, real_ts);
CREATE UNIQUE INDEX IF NOT EXISTS idx_flight_plans_uid ON flight_plans(uid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_missions_import
	ON missions(owner_id, client_id, external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_debriefs_import
	ON debriefs(mission_id, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_flight_plans_sync ON flight_plans(sync_revision);
CREATE INDEX IF NOT EXISTS idx_checklists_sync ON checklists(sync_revision);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)
	WHERE email IS NOT NULL AND email <> '';
`

	if _, err := d.sql.Exec(schema); err != nil {
		return err
	}
	if err := d.migrations(); err != nil {
		return err
	}
	if _, err := d.sql.Exec(indexes); err != nil {
		return err
	}
	return d.ensureSingleOpenMissionIndex()
}

// ensureSingleOpenMissionIndex enforces "at most one open mission" at the database
// level, so a race between components cannot leave several missions open. A
// previously written database may already violate it, so older duplicates are
// reconciled (all but the newest are closed) before the index is created.
func (d *DB) ensureSingleOpenMissionIndex() error {
	const create = `CREATE UNIQUE INDEX IF NOT EXISTS idx_missions_one_open
		ON missions((1)) WHERE ended_at IS NULL`
	if _, err := d.sql.Exec(create); err == nil {
		return nil
	}

	// Reconcile a database that predates the constraint: close every open mission
	// but the most recent one.
	if _, err := d.sql.Exec(`
		UPDATE missions SET ended_at = started_at
		WHERE ended_at IS NULL
		  AND id <> (SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1)`,
	); err != nil {
		return err
	}
	_, err := d.sql.Exec(create)
	return err
}

// migrations applies additive schema changes to databases created by an older
// version. `CREATE TABLE IF NOT EXISTS` never alters an existing table, so a
// database written before a column existed must be upgraded here.
func (d *DB) migrations() error {
	if err := d.ensureColumn("missions", "source", "TEXT NOT NULL DEFAULT 'live'"); err != nil {
		return err
	}
	if err := d.ensureColumn("player_stats", "country", "TEXT"); err != nil {
		return err
	}
	// Flight plans gained a stable identifier and a revision counter for future
	// synchronisation. Existing plans get an identifier before the unique index
	// is created.
	if err := d.ensureColumn("flight_plans", "uid", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "revision", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	// Server imports: a mission uploaded by a desktop is identified by its
	// owner, client and desktop-side id. The columns are empty on the desktop.
	if err := d.ensureColumn("missions", "owner_id", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("missions", "client_id", "TEXT"); err != nil {
		return err
	}
	if err := d.ensureColumn("missions", "external_id", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("debriefs", "external_id", "INTEGER"); err != nil {
		return err
	}
	// Document ownership and synchronisation.
	if err := d.ensureColumn("users", "email", "TEXT"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "owner_id", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "visibility", "TEXT NOT NULL DEFAULT 'private'"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "deleted_at", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "sync_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("flight_plans", "dirty", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := d.ensureColumn("checklists", "owner_id", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("checklists", "deleted_at", "INTEGER"); err != nil {
		return err
	}
	if err := d.ensureColumn("checklists", "sync_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("checklists", "dirty", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	return d.backfillFlightPlanUIDs()
}

// backfillFlightPlanUIDs gives an identifier to plans written by a version that
// predates the uid column.
func (d *DB) backfillFlightPlanUIDs() error {
	rows, err := d.sql.Query(`SELECT id FROM flight_plans WHERE uid IS NULL OR uid = ''`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		if _, err := d.sql.Exec(`UPDATE flight_plans SET uid = ? WHERE id = ?`, idgen.New("fp_"), id); err != nil {
			return err
		}
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
