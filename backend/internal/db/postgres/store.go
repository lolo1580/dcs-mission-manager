package postgres

import (
	"database/sql"
	"time"

	"dcsmanager/internal/db"
	"dcsmanager/internal/model"
)

// Mission sources, re-exported so callers of this package use the same
// constants as the SQLite store.
const (
	SourceLive = db.SourceLive
	SourceTest = db.SourceTest
)

// ValidSource reports whether s is a known mission source.
func ValidSource(s string) bool { return db.ValidSource(s) }

// EnsureMission returns the id of the open mission, creating one from the given
// details if none exists.
func (d *DB) EnsureMission(name, theatre string) (int64, error) {
	return d.EnsureMissionTagged(name, theatre, SourceLive)
}

// OpenMissionID returns the current open mission id, or 0 if none.
func (d *DB) OpenMissionID() int64 {
	var id int64
	err := d.sql.QueryRow(`SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// EndOpenMission closes the open mission and records the winner.
func (d *DB) EndOpenMission(winner string) error {
	_, err := d.sql.Exec(
		`UPDATE missions SET ended_at = $1, winner = $2 WHERE ended_at IS NULL`,
		time.Now().UnixMilli(), winner,
	)
	return err
}

// SaveEvent persists a game event, linking it to the given mission (0 = none).
func (d *DB) SaveEvent(missionID int64, e model.Event) error {
	args := marshalJSON(e.Args)
	var detail any
	if len(e.Detail) > 0 {
		detail = string(e.Detail)
	}
	_, err := d.sql.Exec(
		`INSERT INTO events(mission_id, event, args, detail, t, real_ts) VALUES($1, $2, $3, $4, $5, $6)`,
		nullInt(missionID), e.Event, string(args), detail, e.T, e.RealTS,
	)
	return err
}

// SaveChat persists a chat message.
func (d *DB) SaveChat(missionID int64, c model.Chat) error {
	_, err := d.sql.Exec(
		`INSERT INTO chat(mission_id, "from", message, real_ts) VALUES($1, $2, $3, $4)`,
		nullInt(missionID), c.From, c.Message, c.RealTS,
	)
	return err
}

// UpsertPlayer records a player identity (by UCID when available) and returns
// its row id.
func (d *DB) UpsertPlayer(ucid, name string) (int64, error) {
	now := time.Now().UnixMilli()

	if ucid != "" {
		var id int64
		err := d.sql.QueryRow(`SELECT id FROM players WHERE ucid = $1`, ucid).Scan(&id)
		switch err {
		case nil:
			_, updErr := d.sql.Exec(`UPDATE players SET name = $1, last_seen = $2 WHERE id = $3`, name, now, id)
			return id, updErr
		case sql.ErrNoRows:
			// fall through to insert
		default:
			return 0, err
		}
	}

	var id int64
	err := d.sql.QueryRow(`SELECT id FROM players WHERE name = $1 AND (ucid IS NULL OR ucid = '')`, name).Scan(&id)
	if err == nil {
		if ucid != "" {
			_, updErr := d.sql.Exec(`UPDATE players SET ucid = $1, last_seen = $2 WHERE id = $3`, ucid, now, id)
			return id, updErr
		}
		_, updErr := d.sql.Exec(`UPDATE players SET last_seen = $1 WHERE id = $2`, now, id)
		return id, updErr
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	row := d.sql.QueryRow(
		`INSERT INTO players(ucid, name, first_seen, last_seen) VALUES($1, $2, $3, $4) RETURNING id`,
		nullStr(ucid), name, now, now,
	)
	return lastInsertID(row)
}

// SaveStats records a player's statistics snapshot for a mission.
func (d *DB) SaveStats(missionID, playerID int64, p model.Player) error {
	_, err := d.sql.Exec(
		`INSERT INTO player_stats(mission_id, player_id, dcs_player_id, side, slot, unit_type, ping, crashes,
			kills_car, kills_air, kills_ship, score, landings, ejects, real_ts)
		 VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		nullInt(missionID), nullInt(playerID), p.ID, p.Side, p.Slot, nullStr(p.UnitType), p.Ping, p.Crashes,
		p.KillsCar, p.KillsAir, p.KillsShip, p.Score, p.Landings, p.Ejects,
		time.Now().UnixMilli(),
	)
	return err
}

// RecentEvents returns the most recent events, newest first.
func (d *DB) RecentEvents(limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT id, event, args, COALESCE(detail, ''), COALESCE(t, 0), real_ts
		 FROM events ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// RecentChat returns the most recent chat messages, newest first.
func (d *DB) RecentChat(limit int) ([]model.Chat, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT id, "from", message, real_ts FROM chat ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChat(rows)
}

// Missions returns recent missions, newest first.
func (d *DB) Missions(limit int) ([]model.Mission, error) {
	return d.MissionsWithSource("", limit)
}

// MissionsWithSource returns recent missions filtered by source, newest first.
func (d *DB) MissionsWithSource(source string, limit int) ([]model.Mission, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
	             COALESCE(ended_at,0), COALESCE(winner,'')
	      FROM missions`
	var args []any
	if source != "" {
		q += ` WHERE source = $1`
		args = append(args, source)
	}
	q += ` ORDER BY id DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit)

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Mission
	for rows.Next() {
		var m model.Mission
		if err := rows.Scan(&m.ID, &m.Name, &m.Theatre, &m.Source, &m.StartedAt, &m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// EventsSince is the incremental counterpart of RecentEvents.
func (d *DB) EventsSince(sinceID int64, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, event, args, COALESCE(detail, ''), COALESCE(t, 0), real_ts
		 FROM events WHERE id > $1 ORDER BY id ASC LIMIT $2`, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// ChatSince is the incremental counterpart of RecentChat.
func (d *DB) ChatSince(sinceID int64, limit int) ([]model.Chat, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, "from", message, real_ts FROM chat WHERE id > $1 ORDER BY id ASC LIMIT $2`, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChat(rows)
}

// MissionsSince returns missions with an id strictly greater than sinceID.
func (d *DB) MissionsSince(sinceID int64, limit int) ([]model.Mission, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
		        COALESCE(ended_at,0), COALESCE(winner,'')
		 FROM missions WHERE id > $1 ORDER BY id ASC LIMIT $2`, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Mission
	for rows.Next() {
		var m model.Mission
		if err := rows.Scan(&m.ID, &m.Name, &m.Theatre, &m.Source, &m.StartedAt, &m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// EnsureMissionTagged returns the id of the open mission, creating one with the
// given source if none exists. The get-or-create is serialised by openMissionMu.
func (d *DB) EnsureMissionTagged(name, theatre, source string) (int64, error) {
	if !ValidSource(source) {
		source = SourceLive
	}

	d.openMissionMu.Lock()
	defer d.openMissionMu.Unlock()

	var id int64
	err := d.sql.QueryRow(`SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id)
	if err == nil {
		if source == SourceTest {
			if _, upErr := d.sql.Exec(
				`UPDATE missions SET source = $1 WHERE id = $2 AND source = $3`,
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

	row := d.sql.QueryRow(
		`INSERT INTO missions(name, theatre, source, started_at) VALUES($1, $2, $3, $4) RETURNING id`,
		name, theatre, source, time.Now().UnixMilli(),
	)
	newID, err := lastInsertID(row)
	if err != nil {
		if id := d.OpenMissionID(); id != 0 {
			return id, nil
		}
		return 0, err
	}
	return newID, nil
}

// StartMission records the start of a mission, opening a new one even if another
// is still open (see the SQLite implementation for the full rationale).
func (d *DB) StartMission(name, theatre, source string) (int64, error) {
	if !ValidSource(source) {
		source = SourceLive
	}

	d.openMissionMu.Lock()
	defer d.openMissionMu.Unlock()

	var (
		id      int64
		oldName string
		oldThe  string
	)
	err := d.sql.QueryRow(
		`SELECT id, name, COALESCE(theatre,'') FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`,
	).Scan(&id, &oldName, &oldThe)

	switch {
	case err == nil && oldName == name && oldThe == theatre:
		if source == SourceTest {
			if _, upErr := d.sql.Exec(`UPDATE missions SET source = $1 WHERE id = $2 AND source = $3`,
				SourceTest, id, SourceLive); upErr != nil {
				return id, upErr
			}
		}
		return id, nil
	case err == nil:
		if _, endErr := d.sql.Exec(
			`UPDATE missions SET ended_at = $1 WHERE id = $2`, time.Now().UnixMilli(), id); endErr != nil {
			return 0, endErr
		}
	case err != sql.ErrNoRows:
		return 0, err
	}

	row := d.sql.QueryRow(
		`INSERT INTO missions(name, theatre, source, started_at) VALUES($1, $2, $3, $4) RETURNING id`,
		name, theatre, source, time.Now().UnixMilli(),
	)
	return lastInsertID(row)
}

// CountMissions returns the number of missions for the given source.
func (d *DB) CountMissions(source string) (int, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM missions WHERE source = $1`, source).Scan(&n)
	return n, err
}

// UpgradeMissionSource marks an existing mission as simulated when it is still
// tagged live. It never downgrades.
func (d *DB) UpgradeMissionSource(id int64, source string) error {
	if source != SourceTest || id == 0 {
		return nil
	}
	_, err := d.sql.Exec(
		`UPDATE missions SET source = $1 WHERE id = $2 AND source = $3`,
		SourceTest, id, SourceLive,
	)
	return err
}

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
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`)
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
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		nullInt(missionID), s.UnitID, nullStr(s.Type), nullStr(s.Category),
		nullStr(s.Coalition), s.Lat, s.Lng, s.Alt, boolInt(s.Ownship),
		time.Now().UnixMilli(),
	)
	return err
}

// Heatmap aggregates positions or losses into a grid.
//
// Unlike SQLite, PostgreSQL cannot GROUP BY a SELECT alias and has no
// round(double, int): the expressions are repeated in a subquery and rounded via
// numeric before being cast back to double precision.
func (d *DB) Heatmap(missionID int64, source string, grid float64, limit int) ([]db.HeatPoint, error) {
	if grid <= 0 {
		grid = 0.05
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}

	table := "track_positions"
	if source == "losses" {
		table = "losses"
	}

	filter := ""
	// Only two grid parameters: the query references $1 and $2. Passing four
	// (as the SQLite version did with its repeated `?`) leaves $3/$4 unreferenced
	// and PostgreSQL cannot infer their type ("could not determine data type of
	// parameter $3").
	args := []any{grid, grid}
	if missionID > 0 {
		filter = " WHERE mission_id = $3"
		args = append(args, missionID)
	}
	args = append(args, limit)
	limitPos := len(args)

	rows, err := d.sql.Query(`
		SELECT glat, glng, COUNT(*) AS weight FROM (
			SELECT round((lat / $1)::numeric, 0)::double precision AS glat,
			       round((lng / $2)::numeric, 0)::double precision AS glng
			FROM `+table+filter+`
		) g
		GROUP BY glat, glng
		ORDER BY weight DESC
		LIMIT $`+itoa(limitPos), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []db.HeatPoint
	for rows.Next() {
		var p db.HeatPoint
		if err := rows.Scan(&p.Lat, &p.Lng, &p.Weight); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Trails returns the tracks of the most active units in the scope.
func (d *DB) Trails(missionID int64, limitUnits, maxPointsPerUnit int) (map[string][]db.TrailPoint, error) {
	if limitUnits <= 0 || limitUnits > 50 {
		limitUnits = 10
	}
	if maxPointsPerUnit <= 0 || maxPointsPerUnit > 2000 {
		maxPointsPerUnit = 400
	}

	where := " WHERE category IN ('plane','heli')"
	var args []any
	if missionID > 0 {
		where += " AND mission_id = $1"
		args = append(args, missionID)
	}

	args = append(args, limitUnits)
	rows, err := d.sql.Query(`
		SELECT unit_id FROM track_positions`+where+`
		GROUP BY unit_id ORDER BY COUNT(*) DESC LIMIT $`+itoa(len(args)), args...)
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

	out := make(map[string][]db.TrailPoint, len(unitIDs))
	for _, id := range unitIDs {
		pts, err := d.trailFor(missionID, id, maxPointsPerUnit)
		if err != nil {
			return nil, err
		}
		out[id] = pts
	}
	return out, nil
}

func (d *DB) trailFor(missionID int64, unitID string, maxPoints int) ([]db.TrailPoint, error) {
	where := " WHERE unit_id = $1"
	args := []any{unitID}
	if missionID > 0 {
		where += " AND mission_id = $2"
		args = append(args, missionID)
	}
	args = append(args, maxPoints)

	rows, err := d.sql.Query(`
		SELECT lat, lng, COALESCE(alt,0), COALESCE(speed,0), COALESCE(g,0), real_ts
		FROM track_positions`+where+`
		ORDER BY real_ts ASC LIMIT $`+itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pts []db.TrailPoint
	for rows.Next() {
		var p db.TrailPoint
		if err := rows.Scan(&p.Lat, &p.Lng, &p.Alt, &p.Speed, &p.G, &p.RealTS); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// PruneTracking deletes tracking data older than the given age.
func (d *DB) PruneTracking(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()
	var total int64
	for _, table := range []string{"track_positions", "losses"} {
		res, err := d.sql.Exec(`DELETE FROM `+table+` WHERE real_ts < $1`, cutoff)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// SaveDebrief stores a parsed debrief and returns it with its assigned id.
func (d *DB) SaveDebrief(rec model.Debrief) (model.Debrief, error) {
	parsed := marshalJSON(rec.Parsed)
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UnixMilli()
	}

	row := d.sql.QueryRow(
		`INSERT INTO debriefs(mission_id, mission, theatre, raw, parsed, size, created_at)
		 VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		nullInt(rec.MissionID), nullStr(rec.Mission), nullStr(rec.Theatre),
		nullStr(rec.Raw), string(parsed), rec.Size, rec.CreatedAt,
	)
	id, err := lastInsertID(row)
	rec.ID = id
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
		 FROM debriefs ORDER BY id DESC LIMIT $1`, limit)
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
		 FROM debriefs WHERE id = $1`, id)

	var rec model.Debrief
	var parsed string
	if err := row.Scan(&rec.ID, &rec.MissionID, &rec.Mission, &rec.Theatre,
		&parsed, &rec.Size, &rec.CreatedAt, &rec.Raw); err != nil {
		return rec, err
	}
	if err := unmarshalJSON(parsed, &rec.Parsed); err != nil {
		return rec, err
	}
	return rec, nil
}
