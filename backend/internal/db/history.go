package db

import (
	"encoding/json"

	"dcsmanager/internal/model"
)

// EventsSince returns persisted events with an id strictly greater than sinceID,
// oldest first. It is the incremental counterpart of RecentEvents: a consumer
// stores the largest id it has seen and asks for everything after it, so a full
// history can be mirrored without gaps or duplicates. limit caps a single batch.
func (d *DB) EventsSince(sinceID int64, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, event, args, COALESCE(detail, ''), COALESCE(t, 0), real_ts
		 FROM events WHERE id > ? ORDER BY id ASC LIMIT ?`, sinceID, limit)
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

// ChatSince is the incremental counterpart of RecentChat.
func (d *DB) ChatSince(sinceID int64, limit int) ([]model.Chat, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, "from", message, real_ts FROM chat WHERE id > ? ORDER BY id ASC LIMIT ?`, sinceID, limit)
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

// MissionsSince returns missions with an id strictly greater than sinceID,
// oldest first. It lets a consumer discover new missions incrementally. It does
// not report later changes to an existing mission (a mission ending sets
// ended_at/winner): a consumer that needs those also refreshes the most recent
// missions, which are few and are returned by the non-incremental form.
func (d *DB) MissionsSince(sinceID int64, limit int) ([]model.Mission, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := d.sql.Query(
		`SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
		        COALESCE(ended_at,0), COALESCE(winner,'')
		 FROM missions WHERE id > ? ORDER BY id ASC LIMIT ?`, sinceID, limit)
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
