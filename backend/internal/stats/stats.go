// Package stats computes advanced statistics from stored missions, events and
// player snapshots.
//
// Two scopes are supported everywhere:
//
//   - "mission": statistics for a single mission;
//   - "career":  statistics aggregated across all missions, keyed by UCID so a
//     player keeps their history even after renaming.
//
// Aggregation partly runs in Go because DCS event arguments are stored as JSON
// and their meaning depends on the event type; doing it here keeps the rules
// explicit and testable.
package stats

import (
	"encoding/json"

	"dcsmanager/internal/category"
	"dcsmanager/internal/db"
)

// Scope selects how statistics are aggregated.
type Scope struct {
	// MissionID restricts to one mission when Mode is "mission".
	MissionID int64
	// Mode is "mission" or "career".
	Mode string
	// IncludeTest also counts missions recorded from the test tools. Off by
	// default: simulated sessions must never flatter a real career.
	IncludeTest bool
}

// filter returns the SQL fragment and args restricting a query to the scope.
//
// Every statistics query funnels through here, so this is the one place that
// decides whether test sessions are visible. Because the fragment is built into
// the WHERE clause, it also holds for the sub-queries used by the per-player and
// per-coalition aggregates.
func (sc Scope) filter(column string) (string, []any) {
	frag := ""
	var args []any

	if sc.Mode == "mission" && sc.MissionID > 0 {
		frag += " AND " + column + " = ?"
		args = append(args, sc.MissionID)
	}
	if !sc.IncludeTest {
		// Rows with a NULL mission id (unattributed data) are excluded too,
		// since NULL never matches an IN sub-query.
		frag += " AND " + column + " IN (SELECT id FROM missions WHERE source <> 'test')"
	}
	return frag, args
}

// Service exposes statistics queries.
type Service struct {
	db         db.Store
	classifier *category.Classifier
}

// New creates a stats service.
func New(database db.Store, classifier *category.Classifier) *Service {
	return &Service{db: database, classifier: classifier}
}

// --- models ----------------------------------------------------------------

// PilotStats is a player's aggregated performance.
type PilotStats struct {
	UCID       string  `json:"ucid"`
	Name       string  `json:"name"`
	Missions   int     `json:"missions"`
	Score      int     `json:"score"`
	KillsAir   int     `json:"killsAir"`
	KillsCar   int     `json:"killsCar"`
	KillsShip  int     `json:"killsShip"`
	Kills      int     `json:"kills"`
	Deaths     int     `json:"deaths"`
	Crashes    int     `json:"crashes"`
	Ejections  int     `json:"ejections"`
	Landings   int     `json:"landings"`
	FriendlyFF int     `json:"friendlyFire"`
	AvgPing    float64 `json:"avgPing"`
	KD         float64 `json:"kd"`
}

// WeaponStats describes the effectiveness of one weapon.
type WeaponStats struct {
	Weapon        string         `json:"weapon"`
	Kills         int            `json:"kills"`
	FriendlyFire  int            `json:"friendlyFire"`
	VictimsByType map[string]int `json:"victimsByType"`
	KillersByType map[string]int `json:"killersByType"`
}

// EngineStats describes how a DCS unit type performs (exact type id).
type EngineStats struct {
	TypeID   string  `json:"typeId"`
	Category string  `json:"category"`
	Kills    int     `json:"kills"`
	Deaths   int     `json:"deaths"`
	Sorties  int     `json:"sorties"`
	KD       float64 `json:"kd"`
}

// CoalitionStats is a coalition's aggregate performance.
type CoalitionStats struct {
	Coalition string `json:"coalition"`
	Score     int    `json:"score"`
	Kills     int    `json:"kills"`
	Players   int    `json:"players"`
}

// NetworkStats is one player's connection quality.
type NetworkStats struct {
	Name    string  `json:"name"`
	Samples int     `json:"samples"`
	AvgPing float64 `json:"avgPing"`
	MaxPing int     `json:"maxPing"`
}

// Overview is the top-level dashboard payload.
type Overview struct {
	Scope      string           `json:"scope"`
	MissionID  int64            `json:"missionId,omitempty"`
	Missions   int              `json:"missions"`
	Players    int              `json:"players"`
	Events     int              `json:"events"`
	Kills      int              `json:"kills"`
	Deaths     int              `json:"deaths"`
	Crashes    int              `json:"crashes"`
	Ejections  int              `json:"ejections"`
	FriendlyFF int              `json:"friendlyFire"`
	Coalitions []CoalitionStats `json:"coalitions"`
}

// --- queries ---------------------------------------------------------------

// Pilots returns per-player statistics, best score first.
//
// Kills, score, crashes, ejections and landings come from the *latest* snapshot
// per mission, not from every snapshot: the hook resends the same cumulative
// counters every few seconds, so summing all rows multiplied a player's totals by
// the number of samples. One mission contributes its final counters; a career
// sums those finals across missions.
//
// Deaths and friendly-fire are derived from events: DCS event arguments carry the
// *DCS* player id, and the same id can belong to different people in different
// missions, so the id is resolved per mission.
func (s *Service) Pilots(sc Scope) ([]PilotStats, error) {
	where, args := sc.filter("ps.mission_id")

	rows, err := s.db.Query(`
		WITH latest AS (
			SELECT MAX(id) AS id
			FROM player_stats ps
			WHERE 1=1`+where+`
			GROUP BY ps.mission_id, ps.player_id
		)
		SELECT p.id, COALESCE(p.ucid,''), p.name,
		       COUNT(DISTINCT ps.mission_id),
		       COALESCE(SUM(ps.score),0),
		       COALESCE(SUM(ps.kills_air),0), COALESCE(SUM(ps.kills_car),0), COALESCE(SUM(ps.kills_ship),0),
		       COALESCE(SUM(ps.crashes),0), COALESCE(SUM(ps.ejects),0), COALESCE(SUM(ps.landings),0),
		       AVG(NULLIF(ps.ping,0))
		FROM player_stats ps
		JOIN players p ON p.id = ps.player_id
		WHERE ps.id IN (SELECT id FROM latest)
		GROUP BY p.id
		ORDER BY COALESCE(SUM(ps.score),0) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PilotStats
	byID := map[int64]int{} // player row id -> index in out
	for rows.Next() {
		var p PilotStats
		var id int64
		var avgPing *float64
		if err := rows.Scan(&id, &p.UCID, &p.Name, &p.Missions, &p.Score,
			&p.KillsAir, &p.KillsCar, &p.KillsShip,
			&p.Crashes, &p.Ejections, &p.Landings, &avgPing); err != nil {
			return nil, err
		}
		if avgPing != nil {
			p.AvgPing = *avgPing
		}
		p.Kills = p.KillsAir + p.KillsCar + p.KillsShip
		byID[id] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Deaths and friendly-fire, resolved per mission through the DCS player id.
	deaths, ff, err := s.eventCountsByPlayer(sc)
	if err != nil {
		return nil, err
	}
	for id, idx := range byID {
		out[idx].Deaths = deaths[id]
		out[idx].FriendlyFF = ff[id]
		if out[idx].Deaths > 0 {
			out[idx].KD = float64(out[idx].Kills) / float64(out[idx].Deaths)
		} else if out[idx].Kills > 0 {
			out[idx].KD = float64(out[idx].Kills)
		}
	}
	return out, nil
}

// eventCountsByPlayer counts deaths and friendly-fire per player row, resolving
// each event's DCS player id within its own mission. The same DCS id can belong
// to different players in different missions, so a career-wide id→name map would
// attribute one player's deaths to another.
func (s *Service) eventCountsByPlayer(sc Scope) (deaths, ff map[int64]int, err error) {
	// (mission_id, dcs_player_id) -> player row id.
	resolver := map[[2]int64]int64{}
	where, args := sc.filter("ps.mission_id")
	rows, err := s.db.Query(`
		SELECT DISTINCT ps.mission_id, ps.dcs_player_id, ps.player_id
		FROM player_stats ps
		WHERE ps.dcs_player_id IS NOT NULL AND ps.mission_id IS NOT NULL`+where, args...)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var missionID, dcsID, playerID int64
		if err := rows.Scan(&missionID, &dcsID, &playerID); err != nil {
			rows.Close()
			return nil, nil, err
		}
		resolver[[2]int64{missionID, dcsID}] = playerID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	eWhere, eArgs := sc.filter("mission_id")
	erows, err := s.db.Query(
		`SELECT mission_id, event, args FROM events
		 WHERE event IN ('pilot_death','friendly_fire') AND mission_id IS NOT NULL`+eWhere, eArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer erows.Close()

	deaths = map[int64]int{}
	ff = map[int64]int{}
	for erows.Next() {
		var missionID int64
		var kind, argsJSON string
		if err := erows.Scan(&missionID, &kind, &argsJSON); err != nil {
			return nil, nil, err
		}
		var a []any
		_ = json.Unmarshal([]byte(argsJSON), &a)

		playerID, ok := resolver[[2]int64{missionID, int64(num(a, 0))}]
		if !ok {
			continue
		}
		switch kind {
		case "pilot_death":
			// DCS: playerID, unit_missionID
			deaths[playerID]++
		case "friendly_fire":
			// DCS: playerID, weaponName, victimPlayerID
			ff[playerID]++
		}
	}
	return deaths, ff, erows.Err()
}

// Weapons aggregates weapon performance from kill and friendly-fire events.
func (s *Service) Weapons(sc Scope) ([]WeaponStats, error) {
	where, args := sc.filter("mission_id")
	rows, err := s.db.Query(
		`SELECT event, args FROM events WHERE event IN ('kill','friendly_fire')`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byWeapon := map[string]*WeaponStats{}
	for rows.Next() {
		var kind, argsJSON string
		if err := rows.Scan(&kind, &argsJSON); err != nil {
			return nil, err
		}
		var a []any
		_ = json.Unmarshal([]byte(argsJSON), &a)

		if kind == "kill" {
			// killerID, killerUnitType, killerSide, victimID, victimUnitType, victimSide, weapon
			weapon := str(a, 6)
			if weapon == "" {
				continue
			}
			w := ensureWeapon(byWeapon, weapon)
			w.Kills++
			if vt := str(a, 4); vt != "" {
				w.VictimsByType[vt]++
			}
			if kt := str(a, 1); kt != "" {
				w.KillersByType[kt]++
			}
		} else {
			// playerID, weaponName, victimPlayerID
			weapon := str(a, 1)
			if weapon == "" {
				continue
			}
			ensureWeapon(byWeapon, weapon).FriendlyFire++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]WeaponStats, 0, len(byWeapon))
	for _, w := range byWeapon {
		out = append(out, *w)
	}
	sortWeapons(out)
	return out, nil
}

// Engines aggregates per-unit-type performance (exact DCS type id).
func (s *Service) Engines(sc Scope) ([]EngineStats, error) {
	byType := map[string]*EngineStats{}
	ensure := func(typeID string) *EngineStats {
		if e, ok := byType[typeID]; ok {
			return e
		}
		e := &EngineStats{TypeID: typeID, Category: s.classifier.Classify(typeID)}
		byType[typeID] = e
		return e
	}

	where, args := sc.filter("mission_id")
	rows, err := s.db.Query(`SELECT args FROM events WHERE event = 'kill'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var argsJSON string
		if err := rows.Scan(&argsJSON); err != nil {
			return nil, err
		}
		var a []any
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if kt := str(a, 1); kt != "" {
			ensure(kt).Kills++
		}
		if vt := str(a, 4); vt != "" {
			ensure(vt).Deaths++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Uses: the unit types players occupied, from the resolved unit type.
	rows2, err := s.db.Query(
		`SELECT DISTINCT unit_type FROM player_stats WHERE unit_type IS NOT NULL AND unit_type <> ''`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var unitType string
		if err := rows2.Scan(&unitType); err != nil {
			return nil, err
		}
		ensure(unitType).Sorties++
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	out := make([]EngineStats, 0, len(byType))
	for _, e := range byType {
		if e.Deaths > 0 {
			e.KD = float64(e.Kills) / float64(e.Deaths)
		} else if e.Kills > 0 {
			e.KD = float64(e.Kills)
		}
		out = append(out, *e)
	}
	sortEngines(out)
	return out, nil
}

// Coalitions aggregates performance per coalition, from the latest snapshot of
// each player in each mission (see Pilots: summing every snapshot multiplied the
// totals by the sampling rate).
func (s *Service) Coalitions(sc Scope) ([]CoalitionStats, error) {
	where, args := sc.filter("ps.mission_id")
	rows, err := s.db.Query(`
		WITH latest AS (
			SELECT MAX(id) AS id
			FROM player_stats ps
			WHERE 1=1`+where+`
			GROUP BY ps.mission_id, ps.player_id
		)
		SELECT ps.side,
		       COALESCE(SUM(ps.score),0),
		       COALESCE(SUM(ps.kills_air + ps.kills_car + ps.kills_ship),0),
		       COUNT(DISTINCT ps.player_id)
		FROM player_stats ps
		WHERE ps.id IN (SELECT id FROM latest)
		GROUP BY ps.side`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CoalitionStats
	for rows.Next() {
		var side, score, kills, players int
		if err := rows.Scan(&side, &score, &kills, &players); err != nil {
			return nil, err
		}
		out = append(out, CoalitionStats{
			Coalition: sideName(side),
			Score:     score,
			Kills:     kills,
			Players:   players,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortCoalitions(out)
	return out, nil
}

// Network aggregates ping quality per player.
func (s *Service) Network(sc Scope) ([]NetworkStats, error) {
	where, args := sc.filter("ps.mission_id")
	rows, err := s.db.Query(`
		SELECT p.name, COUNT(*), AVG(NULLIF(ps.ping,0)), MAX(ps.ping)
		FROM player_stats ps
		JOIN players p ON p.id = ps.player_id
		WHERE 1=1`+where+`
		GROUP BY p.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NetworkStats
	for rows.Next() {
		var n NetworkStats
		var avg *float64
		if err := rows.Scan(&n.Name, &n.Samples, &avg, &n.MaxPing); err != nil {
			return nil, err
		}
		if avg != nil {
			n.AvgPing = *avg
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortNetwork(out)
	return out, nil
}

// Overview computes the dashboard summary.
func (s *Service) Overview(sc Scope) (Overview, error) {
	o := Overview{Scope: sc.Mode, MissionID: sc.MissionID}
	if o.Scope == "" {
		o.Scope = "career"
	}

	var err error
	o.Coalitions, err = s.Coalitions(sc)
	if err != nil {
		return o, err
	}

	where, args := sc.filter("mission_id")
	var kills, deaths, crashes, ejects, ff *int
	row := s.db.QueryRow(`
		SELECT COUNT(*),
		       SUM(CASE WHEN event='kill' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN event='pilot_death' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN event='crash' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN event='eject' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN event='friendly_fire' THEN 1 ELSE 0 END)
		FROM events WHERE 1=1`+where, args...)
	if err := row.Scan(&o.Events, &kills, &deaths, &crashes, &ejects, &ff); err != nil {
		return o, err
	}
	o.Kills, o.Deaths, o.Crashes, o.Ejections, o.FriendlyFF =
		deref(kills), deref(deaths), deref(crashes), deref(ejects), deref(ff)

	mWhere, mArgs := sc.filter("id")
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM missions WHERE 1=1`+mWhere, mArgs...).Scan(&o.Missions); err != nil {
		return o, err
	}
	// Players is a count of identities, not sessions: player rows survive a
	// purge and carry no source of their own, so it is not filtered here.
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&o.Players); err != nil {
		return o, err
	}
	return o, nil
}

// --- helpers ---------------------------------------------------------------

func ensureWeapon(m map[string]*WeaponStats, name string) *WeaponStats {
	if w, ok := m[name]; ok {
		return w
	}
	w := &WeaponStats{
		Weapon:        name,
		VictimsByType: map[string]int{},
		KillersByType: map[string]int{},
	}
	m[name] = w
	return w
}

func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func sideName(side int) string {
	switch side {
	case 1:
		return "red"
	case 2:
		return "blue"
	default:
		return "spectator"
	}
}

func str(a []any, i int) string {
	if i < 0 || i >= len(a) {
		return ""
	}
	s, _ := a[i].(string)
	return s
}

func num(a []any, i int) float64 {
	if i < 0 || i >= len(a) {
		return 0
	}
	f, _ := a[i].(float64)
	return f
}
