// Package migrate copies an existing SQLite manager database into a PostgreSQL
// one, preserving primary keys so foreign keys keep pointing at the same rows.
//
// It exists because the PostgreSQL backend is a new integration with no in-place
// upgrade path: a user who has been running on SQLite and wants to switch to
// PostgreSQL (or to feed the statistics plugin, which reads PostgreSQL) needs
// their history carried over. It is a one-shot maintenance tool, run from the
// CLI as `dcsmanager migrate-db`.
//
// It works at the SQL level rather than through db.Store: the Store interface
// deliberately hides raw table access, but a faithful copy needs every column,
// including the ones no query exposes (player_stats, players, tracking).
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	_ "modernc.org/sqlite"             // database/sql driver "sqlite"
)

// table describes one table to copy. identity is true for tables whose primary
// key is a PostgreSQL IDENTITY column: inserting explicit ids then requires
// OVERRIDING SYSTEM VALUE, and the sequence must be advanced afterwards.
type table struct {
	name     string
	identity bool
	cols     []string
}

// tables are copied in dependency order (referenced tables first), so foreign
// keys resolve as rows land.
var tables = []table{
	{"missions", true, []string{"id", "name", "theatre", "source", "started_at", "ended_at", "winner"}},
	{"players", true, []string{"id", "ucid", "name", "first_seen", "last_seen"}},
	{"events", true, []string{"id", "mission_id", "event", "args", "detail", "t", "real_ts"}},
	{"chat", true, []string{"id", "mission_id", "from", "message", "real_ts"}},
	{"player_stats", true, []string{
		"id", "mission_id", "player_id", "dcs_player_id", "side", "slot", "unit_type",
		"ping", "crashes", "kills_car", "kills_air", "kills_ship", "score",
		"landings", "ejects", "real_ts",
	}},
	{"debriefs", true, []string{"id", "mission_id", "mission", "theatre", "raw", "parsed", "size", "created_at"}},
	{"track_positions", true, []string{
		"id", "mission_id", "unit_id", "name", "type", "category", "coalition",
		"lat", "lng", "alt", "heading", "speed", "g", "ownship", "real_ts",
	}},
	{"losses", true, []string{
		"id", "mission_id", "unit_id", "type", "category", "coalition",
		"lat", "lng", "alt", "ownship", "real_ts",
	}},
	{"meta", false, []string{"key", "value"}},
}

// TableResult reports what one table contributed.
type TableResult struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

// Result is the whole migration outcome.
type Result struct {
	Tables []TableResult `json:"tables"`
	Total  int64         `json:"total"`
}

// Run copies sqlitePath into postgresDSN. The destination schema must already
// exist (the PostgreSQL store creates it on Open). Existing rows are left
// untouched: every insert is `ON CONFLICT DO NOTHING`, so a re-run only adds
// what is missing. Tables are copied in dependency order; each in its own
// transaction, so a failure names the table it stopped on.
func Run(ctx context.Context, sqlitePath, postgresDSN string) (Result, error) {
	var res Result

	src, err := sql.Open("sqlite", sqlitePath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return res, fmt.Errorf("open sqlite: %w", err)
	}
	defer src.Close()
	if err := src.PingContext(ctx); err != nil {
		return res, fmt.Errorf("sqlite unreachable: %w", err)
	}

	dst, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		return res, fmt.Errorf("open postgres: %w", err)
	}
	defer dst.Close()
	if err := dst.PingContext(ctx); err != nil {
		return res, fmt.Errorf("postgres unreachable: %w", err)
	}

	for _, t := range tables {
		n, err := copyTable(ctx, src, dst, t)
		if err != nil {
			return res, fmt.Errorf("table %s: %w", t.name, err)
		}
		res.Tables = append(res.Tables, TableResult{Table: t.name, Rows: n})
		res.Total += n
	}
	return res, nil
}

// copyTable streams one table from source to destination inside a transaction.
func copyTable(ctx context.Context, src, dst *sql.DB, t table) (int64, error) {
	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	quoted := make([]string, len(t.cols))
	placeholders := make([]string, len(t.cols))
	for i, c := range t.cols {
		quoted[i] = `"` + c + `"`
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	insert := "INSERT INTO " + t.name + " (" + strings.Join(quoted, ",") + ") "
	if t.identity {
		// Explicit ids into a GENERATED ALWAYS AS IDENTITY column require this.
		insert += "OVERRIDING SYSTEM VALUE "
	}
	insert += "VALUES (" + strings.Join(placeholders, ",") + ") ON CONFLICT DO NOTHING"

	stmt, err := tx.PrepareContext(ctx, insert)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	rows, err := src.QueryContext(ctx, "SELECT "+strings.Join(quoted, ",")+" FROM "+t.name)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	defer rows.Close()

	var copied int64
	vals := make([]any, len(t.cols))
	ptrs := make([]any, len(t.cols))
	for i := range ptrs {
		ptrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return copied, fmt.Errorf("scan: %w", err)
		}
		res, err := stmt.ExecContext(ctx, vals...)
		if err != nil {
			return copied, fmt.Errorf("insert: %w", err)
		}
		if n, err := res.RowsAffected(); err == nil {
			copied += n
		}
	}
	if err := rows.Err(); err != nil {
		return copied, err
	}

	if t.identity && copied > 0 {
		// Advance the identity sequence past the highest copied id, so the
		// manager's own inserts do not collide.
		seq := fmt.Sprintf(
			"SELECT setval(pg_get_serial_sequence('%s','id'), (SELECT MAX(id) FROM %s))",
			t.name, t.name)
		if _, err := tx.ExecContext(ctx, seq); err != nil {
			return copied, fmt.Errorf("reset sequence: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return copied, err
	}
	return copied, nil
}
