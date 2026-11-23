package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"dcsmanager/internal/model"
)

// EnsureMission returns the id of the open (not ended) mission, creating one
// from the given details if none exists. New missions are tagged SourceLive;
// use EnsureMissionTagged when the session is known to be simulated.
func (d *DB) EnsureMission(name, theatre string) (int64, error) {
	return d.EnsureMissionTagged(name, theatre, SourceLive)
}

// OpenMissionID returns the current open mission id, or 0 if none.
func (d *DB) OpenMissionID() int64 {
	var id int64
	if err := d.sql.QueryRow(`SELECT id FROM missions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		return 0
	}
	return id
}

// EndOpenMission closes the open mission and records the winner. A normal end
// arrives while the session is alive, so "now" is used; if the end message was
// delayed by an interruption, closeMission clamps the duration to the last
// recorded activity instead.
func (d *DB) EndOpenMission(winner string) error {
	id := d.OpenMissionID()
	if id == 0 {
		return nil
	}
	return d.closeMission(id, winner, time.Now().UnixMilli())
}

// missionIsStale reports whether an open mission has been silent long enough to
// be considered interrupted rather than merely paused.
func missionIsStale(now, startedAt, lastActivity int64) bool {
	stale := staleMissionAfter.Milliseconds()
	if lastActivity > 0 {
		return now-lastActivity > stale
	}
	return now-startedAt > stale
}

// closeMission closes one open mission. When the mission has been silent for
// longer than staleMissionAfter, the end is set to its last activity (or its
// start when it never produced data) and the mission is marked abandoned. This
// is what prevents a mission left open by a DCS crash from lasting several days
// until the next flight begins.
func (d *DB) closeMission(id int64, winner string, at int64) error {
	if id == 0 {
		return nil
	}
	stale := staleMissionAfter.Milliseconds()
	_, err := d.sql.Exec(
		`UPDATE missions SET
			ended_at = CASE
				WHEN COALESCE(last_activity_at,0) = 0 THEN started_at
				WHEN ? - last_activity_at > ? THEN last_activity_at
				ELSE ?
			END,
			abandoned = CASE
				WHEN COALESCE(last_activity_at,0) = 0 THEN 1
				WHEN ? - last_activity_at > ? THEN 1
				ELSE 0
			END,
			winner = ?
		WHERE id = ? AND ended_at IS NULL`,
		at, stale, at, at, stale, winner, id,
	)
	return err
}

// SaveEvent persists a game event, linking it to the given mission (0 = none).
func (d *DB) SaveEvent(missionID int64, e model.Event) error {
	args, _ := json.Marshal(e.Args)
	_, err := d.sql.Exec(
		`INSERT INTO events(mission_id, event, args, detail, t, real_ts) VALUES(?, ?, ?, ?, ?, ?)`,
		nullInt(missionID), e.Event, string(args), nullRaw(e.Detail), e.T, e.RealTS,
	)
	if err == nil {
		_ = d.touchMission(missionID, e.RealTS)
	}
	return err
}

// SaveChat persists a chat message.
func (d *DB) SaveChat(missionID int64, c model.Chat) error {
	_, err := d.sql.Exec(
		`INSERT INTO chat(mission_id, "from", message, real_ts) VALUES(?, ?, ?, ?)`,
		nullInt(missionID), c.From, c.Message, c.RealTS,
	)
	if err == nil {
		_ = d.touchMission(missionID, c.RealTS)
	}
	return err
}

// touchMission records the newest activity for a mission. It is best-effort:
// losing it only affects a later stale-mission close, and the startup repair
// backfills it from the child tables.
func (d *DB) touchMission(missionID, ts int64) error {
	if missionID == 0 {
		return nil
	}
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	_, err := d.sql.Exec(
		`UPDATE missions SET last_activity_at = ? WHERE id = ? AND last_activity_at < ?`,
		ts, missionID, ts,
	)
	return err
}

// UpsertPlayer records a player identity (by UCID when available) and returns
// its row id.
func (d *DB) UpsertPlayer(ucid, name string) (int64, error) {
	now := time.Now().UnixMilli()

	if ucid != "" {
		var id int64
		err := d.sql.QueryRow(`SELECT id FROM players WHERE ucid = ?`, ucid).Scan(&id)
		switch err {
		case nil:
			_, updErr := d.sql.Exec(`UPDATE players SET name = ?, last_seen = ? WHERE id = ?`, name, now, id)
			return id, updErr
		case sql.ErrNoRows:
			// fall through to insert
		default:
			return 0, err
		}
	}

	var id int64
	err := d.sql.QueryRow(`SELECT id FROM players WHERE name = ? AND (ucid IS NULL OR ucid = '')`, name).Scan(&id)
	if err == nil {
		// A name-only row matched. If we now know the UCID, record it: leaving it
		// empty made the identity fragile the moment the player renamed, since the
		// next lookup would find nothing and create a second identity.
		if ucid != "" {
			_, updErr := d.sql.Exec(`UPDATE players SET ucid = ?, last_seen = ? WHERE id = ?`, ucid, now, id)
			return id, updErr
		}
		_, updErr := d.sql.Exec(`UPDATE players SET last_seen = ? WHERE id = ?`, now, id)
		return id, updErr
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := d.sql.Exec(
		`INSERT INTO players(ucid, name, first_seen, last_seen) VALUES(?, ?, ?, ?)`,
		nullStr(ucid), name, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SaveStats records a player's statistics snapshot for a mission.
func (d *DB) SaveStats(missionID, playerID int64, p model.Player) error {
	now := time.Now().UnixMilli()
	_, err := d.sql.Exec(
		`INSERT INTO player_stats(mission_id, player_id, dcs_player_id, side, slot, unit_type, country, ping, crashes,
			kills_car, kills_air, kills_ship, score, landings, ejects, real_ts)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(missionID), nullInt(playerID), p.ID, p.Side, p.Slot, nullStr(p.UnitType), nullStr(p.Country), p.Ping, p.Crashes,
		p.KillsCar, p.KillsAir, p.KillsShip, p.Score, p.Landings, p.Ejects,
		now,
	)
	if err == nil {
		_ = d.touchMission(missionID, now)
	}
	return err
}

// RecentEvents returns the most recent events, newest first.
func (d *DB) RecentEvents(limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT id, event, args, COALESCE(detail, ''), COALESCE(t, 0), real_ts
		 FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Event
	for rows.Next() {
		var e model.Event
		var args, detail string
		if err := rows.Scan(&e.ID, &e.Event, &args, &detail, &e.T, &e.RealTS); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(args), &e.Args)
		if detail != "" {
			e.Detail = json.RawMessage(detail)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecentChat returns the most recent chat messages, newest first.
func (d *DB) RecentChat(limit int) ([]model.Chat, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT id, "from", message, real_ts FROM chat ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Chat
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.ID, &c.From, &c.Message, &c.RealTS); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Missions returns recent missions, newest first.
func (d *DB) Missions(limit int) ([]model.Mission, error) {
	return d.MissionsWithSource("", limit)
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullRaw(v json.RawMessage) any {
	if len(v) == 0 {
		return nil
	}
	return string(v)
}
