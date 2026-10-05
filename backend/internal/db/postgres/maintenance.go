package postgres

import (
	"dcsmanager/internal/db"
)

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
// transaction. Rows whose mission_id is NULL are removed too.
func (d *DB) PurgeMission(id int64) (db.PurgeResult, error) {
	res := db.PurgeResult{Deleted: map[string]int64{}}

	tx, err := d.sql.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for table, column := range tableLinks {
		out, err := tx.Exec(
			`DELETE FROM `+table+` WHERE `+column+` = $1 OR `+column+` IS NULL`, id)
		if err != nil {
			return res, err
		}
		n, _ := out.RowsAffected()
		res.Deleted[table] = n
	}

	out, err := tx.Exec(`DELETE FROM missions WHERE id = $1`, id)
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
func (d *DB) PurgeSource(source string) (db.PurgeResult, error) {
	res := db.PurgeResult{Deleted: map[string]int64{}}

	tx, err := d.sql.Begin()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for table, column := range tableLinks {
		out, err := tx.Exec(`
			DELETE FROM `+table+`
			WHERE `+column+` IN (SELECT id FROM missions WHERE source = $1)
			   OR `+column+` IS NULL`, source)
		if err != nil {
			return res, err
		}
		n, _ := out.RowsAffected()
		res.Deleted[table] = n
	}

	out, err := tx.Exec(`DELETE FROM missions WHERE source = $1`, source)
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
func (d *DB) PurgeAll() (db.PurgeResult, error) {
	res := db.PurgeResult{Deleted: map[string]int64{}}

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
