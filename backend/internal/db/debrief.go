package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"dcsmm/internal/model"
)

// SaveDebrief stores a parsed debrief and returns it with its assigned id.
func (d *DB) SaveDebrief(rec model.Debrief) (model.Debrief, error) {
	parsed, err := json.Marshal(rec.Parsed)
	if err != nil {
		return rec, err
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UnixMilli()
	}

	res, err := d.sql.Exec(
		`INSERT INTO debriefs(mission_id, mission, theatre, raw, parsed, size, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		nullInt(rec.MissionID), nullStr(rec.Mission), nullStr(rec.Theatre),
		nullStr(rec.Raw), string(parsed), rec.Size, rec.CreatedAt,
	)
	if err != nil {
		return rec, err
	}
	rec.ID, err = res.LastInsertId()
	return rec, err
}

// Debriefs returns recent debriefs (metadata only, newest first).
func (d *DB) Debriefs(limit int) ([]model.Debrief, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.sql.Query(
		`SELECT id, COALESCE(mission_id,0), COALESCE(mission,''), COALESCE(theatre,''),
		        parsed, size, created_at
		 FROM debriefs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Debrief
	for rows.Next() {
		rec, err := scanDebrief(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Debrief returns one debrief by id, including its raw content.
func (d *DB) Debrief(id int64) (model.Debrief, error) {
	row := d.sql.QueryRow(
		`SELECT id, COALESCE(mission_id,0), COALESCE(mission,''), COALESCE(theatre,''),
		        parsed, size, created_at, COALESCE(raw,'')
		 FROM debriefs WHERE id = ?`, id)

	var rec model.Debrief
	var parsed string
	if err := row.Scan(&rec.ID, &rec.MissionID, &rec.Mission, &rec.Theatre,
		&parsed, &rec.Size, &rec.CreatedAt, &rec.Raw); err != nil {
		return rec, err
	}
	if err := json.Unmarshal([]byte(parsed), &rec.Parsed); err != nil {
		return rec, err
	}
	return rec, nil
}

func scanDebrief(rows *sql.Rows) (model.Debrief, error) {
	var rec model.Debrief
	var parsed string
	if err := rows.Scan(&rec.ID, &rec.MissionID, &rec.Mission, &rec.Theatre,
		&parsed, &rec.Size, &rec.CreatedAt); err != nil {
		return rec, err
	}
	if err := json.Unmarshal([]byte(parsed), &rec.Parsed); err != nil {
		return rec, err
	}
	return rec, nil
}
