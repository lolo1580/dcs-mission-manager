package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"dcsmanager/internal/model"
)

// MissionExport is one mission and everything the server needs to display it:
// events, chat, player snapshots and debriefs. It is the upload unit used by
// the desktop synchroniser.
type MissionExport struct {
	ExternalID int64  `json:"externalId"`
	Name       string `json:"name"`
	Theatre    string `json:"theatre,omitempty"`
	Source     string `json:"source,omitempty"`
	StartedAt  int64  `json:"startedAt"`
	EndedAt    int64  `json:"endedAt,omitempty"`
	Winner     string `json:"winner,omitempty"`

	Events   []model.Event   `json:"events,omitempty"`
	Chat     []model.Chat    `json:"chat,omitempty"`
	Players  []model.Player  `json:"players,omitempty"`
	Debriefs []DebriefExport `json:"debriefs,omitempty"`
}

// DebriefExport is a debrief without its raw text: the parsed summary is enough
// for the web application, and raw debriefs can be large.
type DebriefExport struct {
	ExternalID int64             `json:"externalId"`
	Mission    string            `json:"mission,omitempty"`
	Theatre    string            `json:"theatre,omitempty"`
	Parsed     model.DebriefData `json:"parsed"`
	Size       int               `json:"size"`
	CreatedAt  int64             `json:"createdAt"`
}

// ExportMissions returns recent live missions with their detail, newest first.
func (d *DB) ExportMissions(limit int) ([]MissionExport, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := d.sql.Query(
		`SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
		        COALESCE(ended_at,0), COALESCE(winner,'')
		 FROM missions WHERE source = ? ORDER BY id DESC LIMIT ?`,
		SourceLive, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var missions []MissionExport
	for rows.Next() {
		var m MissionExport
		if err := rows.Scan(&m.ExternalID, &m.Name, &m.Theatre, &m.Source, &m.StartedAt,
			&m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		missions = append(missions, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for i := range missions {
		id := missions[i].ExternalID
		events, err := d.missionEvents(id)
		if err != nil {
			return nil, err
		}
		chat, err := d.missionChat(id)
		if err != nil {
			return nil, err
		}
		players, err := d.missionPlayers(id)
		if err != nil {
			return nil, err
		}
		debriefs, err := d.missionDebriefs(id)
		if err != nil {
			return nil, err
		}
		missions[i].Events = events
		missions[i].Chat = chat
		missions[i].Players = players
		missions[i].Debriefs = debriefs
	}
	return missions, nil
}

func (d *DB) missionEvents(missionID int64) ([]model.Event, error) {
	rows, err := d.sql.Query(
		`SELECT event, args, detail, t, real_ts FROM events WHERE mission_id = ? ORDER BY id`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var (
			e      model.Event
			args   sql.NullString
			detail sql.NullString
			t      sql.NullFloat64
		)
		if err := rows.Scan(&e.Event, &args, &detail, &t, &e.RealTS); err != nil {
			return nil, err
		}
		if args.Valid && args.String != "" {
			_ = json.Unmarshal([]byte(args.String), &e.Args)
		}
		if detail.Valid && detail.String != "" {
			e.Detail = json.RawMessage(detail.String)
		}
		if t.Valid {
			e.T = t.Float64
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) missionChat(missionID int64) ([]model.Chat, error) {
	rows, err := d.sql.Query(
		`SELECT "from", message, real_ts FROM chat WHERE mission_id = ? ORDER BY id`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Chat
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.From, &c.Message, &c.RealTS); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) missionPlayers(missionID int64) ([]model.Player, error) {
	rows, err := d.sql.Query(
		`SELECT COALESCE(p.ucid,''), p.name, COALESCE(s.side,0), COALESCE(s.slot,''),
		        COALESCE(s.unit_type,''), COALESCE(s.country,''), COALESCE(s.ping,0),
		        COALESCE(s.crashes,0), COALESCE(s.kills_car,0), COALESCE(s.kills_air,0),
		        COALESCE(s.kills_ship,0), COALESCE(s.score,0), COALESCE(s.landings,0),
		        COALESCE(s.ejects,0), s.real_ts
		 FROM player_stats s JOIN players p ON p.id = s.player_id
		 WHERE s.mission_id = ? ORDER BY s.id`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Player
	for rows.Next() {
		var p model.Player
		if err := rows.Scan(&p.UCID, &p.Name, &p.Side, &p.Slot, &p.UnitType, &p.Country,
			&p.Ping, &p.Crashes, &p.KillsCar, &p.KillsAir, &p.KillsShip, &p.Score,
			&p.Landings, &p.Ejects, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) missionDebriefs(missionID int64) ([]DebriefExport, error) {
	rows, err := d.sql.Query(
		`SELECT id, COALESCE(mission,''), COALESCE(theatre,''), parsed, size, created_at
		 FROM debriefs WHERE mission_id = ? ORDER BY id`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DebriefExport
	for rows.Next() {
		var (
			rec    DebriefExport
			parsed string
		)
		if err := rows.Scan(&rec.ExternalID, &rec.Mission, &rec.Theatre, &parsed, &rec.Size, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(parsed), &rec.Parsed); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ImportMission upserts one uploaded mission and replaces its child records. It
// is idempotent: re-uploading the same desktop mission cleans the previous rows
// before inserting the new snapshot.
func (d *DB) ImportMission(ownerID int64, clientID string, m MissionExport) (int64, error) {
	if m.Name == "" {
		return 0, errors.New("mission has no name")
	}
	if m.Source == "" {
		m.Source = SourceLive
	}
	if m.StartedAt == 0 {
		m.StartedAt = time.Now().UnixMilli()
	}

	var existing int64
	err := d.sql.QueryRow(
		`SELECT id FROM missions WHERE owner_id = ? AND client_id = ? AND external_id = ?`,
		ownerID, clientID, m.ExternalID,
	).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		result, err := d.sql.Exec(
			`INSERT INTO missions(name, theatre, source, started_at, ended_at, winner,
			   owner_id, client_id, external_id)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.Name, nullStr(m.Theatre), m.Source, m.StartedAt, nullInt(m.EndedAt),
			nullStr(m.Winner), ownerID, clientID, m.ExternalID,
		)
		if err != nil {
			return 0, err
		}
		existing, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	case err != nil:
		return 0, err
	default:
		if _, err := d.sql.Exec(
			`UPDATE missions SET name = ?, theatre = ?, source = ?, started_at = ?,
			   ended_at = ?, winner = ? WHERE id = ?`,
			m.Name, nullStr(m.Theatre), m.Source, m.StartedAt, nullInt(m.EndedAt),
			nullStr(m.Winner), existing,
		); err != nil {
			return 0, err
		}
		if _, err := d.sql.Exec(`DELETE FROM events WHERE mission_id = ?`, existing); err != nil {
			return 0, err
		}
		if _, err := d.sql.Exec(`DELETE FROM chat WHERE mission_id = ?`, existing); err != nil {
			return 0, err
		}
		if _, err := d.sql.Exec(`DELETE FROM player_stats WHERE mission_id = ?`, existing); err != nil {
			return 0, err
		}
	}

	for _, event := range m.Events {
		if event.RealTS == 0 {
			event.RealTS = m.StartedAt
		}
		if err := d.SaveEvent(existing, event); err != nil {
			return 0, err
		}
	}
	for _, message := range m.Chat {
		if message.RealTS == 0 {
			message.RealTS = m.StartedAt
		}
		if err := d.SaveChat(existing, message); err != nil {
			return 0, err
		}
	}
	for _, player := range m.Players {
		playerID, err := d.UpsertPlayer(player.UCID, player.Name)
		if err != nil {
			return 0, err
		}
		if err := d.SaveStats(existing, playerID, player); err != nil {
			return 0, err
		}
	}
	for _, rec := range m.Debriefs {
		if err := d.importDebrief(existing, rec); err != nil {
			return 0, err
		}
	}
	return existing, nil
}

func (d *DB) importDebrief(missionID int64, rec DebriefExport) error {
	parsed, err := json.Marshal(rec.Parsed)
	if err != nil {
		return err
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UnixMilli()
	}
	var existing int64
	err = d.sql.QueryRow(
		`SELECT id FROM debriefs WHERE mission_id = ? AND external_id = ?`,
		missionID, rec.ExternalID,
	).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = d.sql.Exec(
			`INSERT INTO debriefs(mission_id, mission, theatre, raw, parsed, size, created_at, external_id)
			 VALUES(?, ?, ?, '', ?, ?, ?, ?)`,
			missionID, nullStr(rec.Mission), nullStr(rec.Theatre), string(parsed), rec.Size, rec.CreatedAt, rec.ExternalID,
		)
		return err
	case err != nil:
		return err
	default:
		_, err = d.sql.Exec(
			`UPDATE debriefs SET mission = ?, theatre = ?, parsed = ?, size = ?, created_at = ? WHERE id = ?`,
			nullStr(rec.Mission), nullStr(rec.Theatre), string(parsed), rec.Size, rec.CreatedAt, existing,
		)
		return err
	}
}
