package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"time"

	"dcsmanager/internal/model"
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
	if err != nil {
		return rec, err
	}
	_ = d.touchMission(rec.MissionID, rec.CreatedAt)
	// A solo or offline flight may have no live player snapshots. The debrief is
	// then the only source of pilot/landing data, so import it for the statistics
	// views. Importing is idempotent per mission.
	if err := d.importDebriefStats(rec); err != nil {
		log.Printf("db: debrief #%d statistics import: %v", rec.ID, err)
	}
	return rec, nil
}

// BackfillDebriefStats imports pilot statistics for debriefs saved before the
// import existed. Missions that already have live player_stats are left alone.
func (d *DB) BackfillDebriefStats() error {
	recs, err := d.Debriefs(200)
	if err != nil {
		return err
	}
	for _, rec := range recs {
		if err := d.importDebriefStats(rec); err != nil {
			return err
		}
	}
	return nil
}

// importDebriefStats turns the debrief summary into player_stats rows. DCS
// debriefs carry the pilot names and the takeoff/land counts; live statistics
// are preferred when they exist, so the import is skipped for a mission that
// already has snapshots.
func (d *DB) importDebriefStats(rec model.Debrief) error {
	if rec.MissionID == 0 {
		return nil
	}
	// The live session often arrives as "tempMission" before the debrief names
	// the .miz. Replace only generic names, never a user-visible real mission.
	if name := missionNameFromDebrief(rec); name != "" {
		_, _ = d.sql.Exec(`UPDATE missions SET name = ?
			WHERE id = ? AND (name = '' OR name = 'tempMission'
			OR name = 'Unknown mission' OR name = 'Session without mission')`, name, rec.MissionID)
	}
	var existing int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM player_stats WHERE mission_id = ?`, rec.MissionID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	pilots := rec.Parsed.Summary.Pilots
	if len(pilots) == 0 && strings.TrimSpace(rec.Parsed.Callsign) != "" {
		pilots = []string{rec.Parsed.Callsign}
	}
	if len(pilots) == 0 {
		return nil
	}

	unitType, side := "", 0
	for _, ev := range rec.Parsed.Events {
		if ev.InitiatorUnitType != "" {
			unitType = ev.InitiatorUnitType
			side = ev.InitiatorCoalition
			break
		}
	}

	for _, name := range pilots {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		playerID, err := d.UpsertPlayer("", name)
		if err != nil {
			return err
		}
		p := model.Player{Name: name, UnitType: unitType, Side: side}
		if len(pilots) == 1 {
			// The summary is mission-wide. With one pilot it is that pilot's.
			p.Landings = rec.Parsed.Summary.Landings
			p.Crashes = rec.Parsed.Summary.Crashes
			p.Ejects = rec.Parsed.Summary.Ejections
			p.KillsAir = rec.Parsed.Summary.Kills
		} else {
			// With several pilots, only per-event counts are attributable.
			for _, ev := range rec.Parsed.Events {
				if !strings.EqualFold(ev.InitiatorPilot, name) {
					continue
				}
				switch strings.ToLower(ev.Type) {
				case "land":
					p.Landings++
				case "crash":
					p.Crashes++
				case "eject", "ejection":
					p.Ejects++
				case "shot down", "kill", "killed":
					p.KillsAir++
				}
			}
		}
		if err := d.SaveStats(rec.MissionID, playerID, p); err != nil {
			return err
		}
	}
	return nil
}

// missionNameFromDebrief derives a readable name from the .miz path when the
// live session only had a placeholder.
func missionNameFromDebrief(rec model.Debrief) string {
	raw := strings.TrimSpace(rec.Parsed.MissionFilePath)
	if raw == "" {
		return ""
	}
	base := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return strings.TrimSpace(base)
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
