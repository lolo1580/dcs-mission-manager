package db

import (
	"time"

	"dcsmanager/internal/model"
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
