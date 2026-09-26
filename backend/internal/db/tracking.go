package db

import (
	"time"

	"dcsmm/internal/model"
)

// SaveSamples stores a batch of position/telemetry samples in one transaction.
func (d *DB) SaveSamples(missionID int64, samples []model.Sample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO track_positions(mission_id, unit_id, name, type, category, coalition,
			lat, lng, alt, heading, speed, g, ownship, real_ts)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UnixMilli()
	for _, s := range samples {
		ts := s.RealTS
		if ts == 0 {
			ts = now
		}
		if _, err := stmt.Exec(
			nullInt(missionID), s.UnitID, nullStr(s.Name), nullStr(s.Type),
			nullStr(s.Category), nullStr(s.Coalition),
			s.Lat, s.Lng, s.Alt, s.Heading, s.Speed, s.G, boolInt(s.Ownship), ts,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SaveLoss records a unit that vanished, with its last known position.
func (d *DB) SaveLoss(missionID int64, s model.Sample) error {
	_, err := d.sql.Exec(`
		INSERT INTO losses(mission_id, unit_id, type, category, coalition, lat, lng, alt, ownship, real_ts)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(missionID), s.UnitID, nullStr(s.Type), nullStr(s.Category),
		nullStr(s.Coalition), s.Lat, s.Lng, s.Alt, boolInt(s.Ownship),
		time.Now().UnixMilli(),
	)
	return err
}

// HeatPoint is an aggregated position/loss cluster for the heatmap.
type HeatPoint struct {
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Weight int     `json:"weight"`
}

// Heatmap aggregates positions (source "positions") or losses (source "losses")
// into a grid of the given size in degrees, for the scope.
//
// Grid size is expressed in degrees so the aggregation is projection-free.
func (d *DB) Heatmap(missionID int64, source string, grid float64, limit int) ([]HeatPoint, error) {
	if grid <= 0 {
		grid = 0.05 // ~5 km at mid latitudes
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}

	table := "track_positions"
	if source == "losses" {
		table = "losses"
	}

	filter := ""
	args := []any{grid, grid, grid, grid}
	if missionID > 0 {
		filter = " WHERE mission_id = ?"
		args = append(args, missionID)
	}
	args = append(args, limit)

	rows, err := d.sql.Query(`
		SELECT ROUND(lat / ?, 0) * ? AS glat,
		       ROUND(lng / ?, 0) * ? AS glng,
		       COUNT(*) AS weight
		FROM `+table+filter+`
		GROUP BY glat, glng
		ORDER BY weight DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HeatPoint
	for rows.Next() {
		var p HeatPoint
		if err := rows.Scan(&p.Lat, &p.Lng, &p.Weight); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TrailPoint is one point of a unit's track.
type TrailPoint struct {
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Alt    float64 `json:"alt"`
	Speed  float64 `json:"speed,omitempty"`
	G      float64 `json:"g,omitempty"`
	RealTS int64   `json:"realTs"`
}

// Trails returns the tracks of the most active units in the scope, keyed by
// unit id. Only aircraft (plane/heli) are included by default because ground
// units barely move.
func (d *DB) Trails(missionID int64, limitUnits, maxPointsPerUnit int) (map[string][]TrailPoint, error) {
	if limitUnits <= 0 || limitUnits > 50 {
		limitUnits = 10
	}
	if maxPointsPerUnit <= 0 || maxPointsPerUnit > 2000 {
		maxPointsPerUnit = 400
	}

	where := " WHERE category IN ('plane','heli')"
	var args []any
	if missionID > 0 {
		where += " AND mission_id = ?"
		args = append(args, missionID)
	}

	// Pick the units with the most samples (longest tracks).
	args = append(args, limitUnits)
	rows, err := d.sql.Query(`
		SELECT unit_id FROM track_positions`+where+`
		GROUP BY unit_id ORDER BY COUNT(*) DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	var unitIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		unitIDs = append(unitIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[string][]TrailPoint, len(unitIDs))
	for _, id := range unitIDs {
		pts, err := d.trailFor(missionID, id, maxPointsPerUnit)
		if err != nil {
			return nil, err
		}
		out[id] = pts
	}
	return out, nil
}

func (d *DB) trailFor(missionID int64, unitID string, maxPoints int) ([]TrailPoint, error) {
	where := " WHERE unit_id = ?"
	args := []any{unitID}
	if missionID > 0 {
		where += " AND mission_id = ?"
		args = append(args, missionID)
	}
	args = append(args, maxPoints)

	rows, err := d.sql.Query(`
		SELECT lat, lng, COALESCE(alt,0), COALESCE(speed,0), COALESCE(g,0), real_ts
		FROM track_positions`+where+`
		ORDER BY real_ts ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pts []TrailPoint
	for rows.Next() {
		var p TrailPoint
		if err := rows.Scan(&p.Lat, &p.Lng, &p.Alt, &p.Speed, &p.G, &p.RealTS); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// PruneTracking deletes tracking data older than the given age, to keep the
// database bounded on long-running servers.
func (d *DB) PruneTracking(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()
	var total int64
	for _, table := range []string{"track_positions", "losses"} {
		res, err := d.sql.Exec(`DELETE FROM `+table+` WHERE real_ts < ?`, cutoff)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
