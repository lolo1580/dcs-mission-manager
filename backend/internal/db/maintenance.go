package db

import (
	"database/sql"
	"time"

	"dcsmm/internal/model"
)

// Mission sources. Every mission row is tagged with one of these so recorded
// sessions can be told apart long after the fact.
//
// The distinction matters because the backend cannot reliably detect simulated
// telemetry: the test tools speak exactly the same UDP/TCP protocol as DCS. A
// session started while the tools are running therefore looks real in every
// respect. Tagging at the source is the only dependable way to keep test data
// out of statistics, and it also protects future features (such as replay) from
// presenting a fabricated flight as a real one.
const (
	// SourceLive is a session recorded from DCS (the default, and the only
	// source counted in statistics unless explicitly requested).
	SourceLive = "live"
	// SourceTest is a session produced by the test tools (tools/send-*.mjs).
	SourceTest = "test"
)

// ValidSource reports whether s is a known mission source.
func ValidSource(s string) bool {
	return s == SourceLive || s == SourceTest
}

// EnsureMissionTagged returns the id of the open mission, creating one with the
// given source if none exists. The default is SourceLive, so an ordinary
// incoming mission is never tagged by accident.
func (d *DB) EnsureMissionTagged(name, theatre, source string) (int64, error) {
	if !ValidSource(source) {
		source = SourceLive
	}

	var id int64
	err := d.sql.QueryRow(`SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id)
	if err == nil {
		// If the open mission was created as live but the current session turns
		// out to be simulated, upgrade the tag: a run must never be counted as
		// real just because it was announced before the first test packet.
		if source == SourceTest {
			if _, upErr := d.sql.Exec(
				`UPDATE missions SET source = ? WHERE id = ? AND source = ?`,
				SourceTest, id, SourceLive,
			); upErr != nil {
				return id, upErr
			}
		}
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := d.sql.Exec(
		`INSERT INTO missions(name, theatre, source, started_at) VALUES(?, ?, ?, ?)`,
		name, theatre, source, time.Now().UnixMilli(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CountMissions returns the number of missions for the given source.
func (d *DB) CountMissions(source string) (int, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM missions WHERE source = ?`, source).Scan(&n)
	return n, err
}

// UpgradeMissionSource marks an existing mission as simulated when it is still
// tagged live. It never downgrades, so a mission cannot lose its "test" tag.
//
// This is needed because a session can be recorded before it is recognised as
// simulated: the tracker may open a mission at its first tick, and the test
// packet that proves the session is fake can arrive just afterwards. Without
// this upgrade the session would stay counted as real for its whole duration.
func (d *DB) UpgradeMissionSource(id int64, source string) error {
	if source != SourceTest || id == 0 {
		return nil
	}
	_, err := d.sql.Exec(
		`UPDATE missions SET source = ? WHERE id = ? AND source = ?`,
		SourceTest, id, SourceLive,
	)
	return err
}

// PurgeResult reports what a purge removed, per table.
type PurgeResult struct {
	Missions int64            `json:"missions"`
	Deleted  map[string]int64 `json:"deleted"`
}

// Total returns the total number of rows deleted.
func (r PurgeResult) Total() int64 {
	total := r.Missions
	for _, n := range r.Deleted {
		total += n
	}
	return total
}

// tableLinks maps each table to the column that links it to a mission.
var tableLinks = map[string]string{
	"track_positions": "mission_id",
	"losses":          "mission_id",
	"events":          "mission_id",
	"chat":            "mission_id",
	"player_stats":    "mission_id",
	"debriefs":        "mission_id",
}

// PurgeMission deletes one mission and every row that references it, in a single
// transaction. Rows whose mission_id is NULL are removed as well, because they
// belong to no surviving mission.
//
// Player identities are deliberately kept: a player row is an identity, not a
// session, and may also be referenced by missions that are not being purged.
func (d *DB) PurgeMission(id int64) (PurgeResult, error) {
	res := PurgeResult{Deleted: map[string]int64{}}

	tx, err := d.sql.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for table, column := range tableLinks {
		// Deleting first by mission id, then the orphaned rows.
		out, err := tx.Exec(
			`DELETE FROM `+table+` WHERE `+column+` = ? OR `+column+` IS NULL`, id)
		if err != nil {
			return res, err
		}
		n, _ := out.RowsAffected()
		res.Deleted[table] = n
	}

	out, err := tx.Exec(`DELETE FROM missions WHERE id = ?`, id)
	if err != nil {
		return res, err
	}
	res.Missions, _ = out.RowsAffected()

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// PurgeSource deletes every mission (and its rows) having the given source.
// Deleting a source also removes the rows that belong to no mission, since they
// cannot be attributed to anything else.
func (d *DB) PurgeSource(source string) (PurgeResult, error) {
	res := PurgeResult{Deleted: map[string]int64{}}

	tx, err := d.sql.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for table, column := range tableLinks {
		out, err := tx.Exec(`
			DELETE FROM `+table+`
			WHERE `+column+` IN (SELECT id FROM missions WHERE source = ?)
			   OR `+column+` IS NULL`, source)
		if err != nil {
			return res, err
		}
		n, _ := out.RowsAffected()
		res.Deleted[table] = n
	}

	out, err := tx.Exec(`DELETE FROM missions WHERE source = ?`, source)
	if err != nil {
		return res, err
	}
	res.Missions, _ = out.RowsAffected()

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// PurgeAll deletes every recorded session and all tracking data, leaving the
// schema and the player identities in place.
func (d *DB) PurgeAll() (PurgeResult, error) {
	res := PurgeResult{Deleted: map[string]int64{}}

	tx, err := d.sql.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for table := range tableLinks {
		out, err := tx.Exec(`DELETE FROM ` + table)
		if err != nil {
			return res, err
		}
		n, _ := out.RowsAffected()
		res.Deleted[table] = n
	}

	out, err := tx.Exec(`DELETE FROM missions`)
	if err != nil {
		return res, err
	}
	res.Missions, _ = out.RowsAffected()

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// MissionsWithSource returns recent missions filtered by source, newest first.
// An empty source returns all of them.
func (d *DB) MissionsWithSource(source string, limit int) ([]model.Mission, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
	             COALESCE(ended_at,0), COALESCE(winner,'')
	      FROM missions`
	var args []any
	if source != "" {
		q += ` WHERE source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Mission
	for rows.Next() {
		var m model.Mission
		if err := rows.Scan(&m.ID, &m.Name, &m.Theatre, &m.Source,
			&m.StartedAt, &m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
