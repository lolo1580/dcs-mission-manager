package postgres

import (
	"database/sql"
	"encoding/json"
	"strconv"

	"dcsmanager/internal/model"
)

// scanEvents decodes rows selected as (id, event, args, detail, t, real_ts).
func scanEvents(rows *sql.Rows) ([]model.Event, error) {
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

// scanChat decodes rows selected as (id, "from", message, real_ts).
func scanChat(rows *sql.Rows) ([]model.Chat, error) {
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

// scanDebrief decodes one debrief metadata row and its parsed JSON.
func scanDebrief(rows *sql.Rows) (model.Debrief, error) {
	var rec model.Debrief
	var parsed string
	if err := rows.Scan(&rec.ID, &rec.MissionID, &rec.Mission, &rec.Theatre,
		&parsed, &rec.Size, &rec.CreatedAt); err != nil {
		return rec, err
	}
	if err := unmarshalJSON(parsed, &rec.Parsed); err != nil {
		return rec, err
	}
	return rec, nil
}

func marshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func unmarshalJSON(s string, dst any) error {
	return json.Unmarshal([]byte(s), dst)
}

func itoa(n int) string { return strconv.Itoa(n) }
