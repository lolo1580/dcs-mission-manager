package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store reads the manager's PostgreSQL database. It is a READER: the pool is
// opened with default_transaction_read_only=on, so no statement it runs can
// modify the manager's data — "read-only" is enforced by PostgreSQL, not just by
// convention. Point it at a dedicated SELECT-only role for defence in depth.
//
// It aggregates directly on the manager's own tables (missions, events, players,
// player_stats), which is possible because the manager can run on PostgreSQL
// (DCSMANAGER_DB_DRIVER=postgres). There is no mirror and no snapshot: the
// manager is the single writer, the plugin a reader.
type Store struct {
	pool *pgxpool.Pool
	// includeTest, when false, excludes simulated missions from every query —
	// the same policy the manager applies to its own statistics.
	includeTest bool
}

// OpenStore connects read-only to the manager's PostgreSQL database. timeout
// bounds every statement so a slow query cannot hold a connection forever.
func OpenStore(ctx context.Context, dsn string, includeTest bool, timeout time.Duration) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("MANAGER_DATABASE_URL is required (the manager must run with DCSMANAGER_DB_DRIVER=postgres)")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	// A statement deadline, so a heavy aggregation cannot pin a pool connection
	// indefinitely. pgx sends this as the statement_timeout GUC.
	if timeout > 0 {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(timeout.Milliseconds(), 10)
	}
	// Read-only at the session level. This is a defence in depth, not the whole
	// story: a role with write grants can SET it off, so a SELECT-only database
	// role is the real guarantee (see the README).
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, includeTest: includeTest}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// missionFilter returns the SQL fragment and arguments restricting a query on a
// mission column to the configured test policy. The column is always a literal
// from this package, never user input.
//
// Readers go through the manager's read-only views (v_stats_*), which carry a
// `source` column, so the test policy is applied here rather than by trusting
// the underlying tables.
func (s *Store) missionFilter(column string) string {
	if s.includeTest {
		return ""
	}
	return " AND " + column + " IN (SELECT id FROM v_stats_missions WHERE source <> 'test')"
}

// --- models (mirroring the manager's stats JSON shapes) -----------------------

// Overview is the top-level dashboard payload.
type Overview struct {
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

// PilotStats is a player's aggregated performance (career-wide, by UCID).
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

// EngineStats describes how a DCS unit type performs.
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

// Mission is the manager's mission row shape.
type Mission struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Theatre   string `json:"theatre,omitempty"`
	Source    string `json:"source,omitempty"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt,omitempty"`
	Winner    string `json:"winner,omitempty"`
}

// Counts reports how many rows the manager holds, for the dashboard footer.
type Counts struct {
	Missions  int64 `json:"missions"`
	Players   int64 `json:"players"`
	Events    int64 `json:"events"`
	Debriefs  int64 `json:"debriefs"`
	Positions int64 `json:"positions"`
}

// --- queries -----------------------------------------------------------------

// Overview aggregates the dashboard summary, mirroring the manager's Overview.
func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var o Overview
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM v_stats_missions WHERE 1=1`+s.missionFilter("id")).Scan(&o.Missions); err != nil {
		return o, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM v_stats_players`).Scan(&o.Players); err != nil {
		return o, err
	}

	// Events counters. SUM returns NULL on no rows, hence COALESCE.
	row := s.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN event='kill' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN event='pilot_death' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN event='crash' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN event='eject' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN event='friendly_fire' THEN 1 ELSE 0 END),0)
		FROM v_stats_events WHERE 1=1`+s.missionFilter("mission_id"))
	if err := row.Scan(&o.Events, &o.Kills, &o.Deaths, &o.Crashes, &o.Ejections, &o.FriendlyFF); err != nil {
		return o, err
	}

	coalitions, err := s.Coalitions(ctx)
	if err != nil {
		return o, err
	}
	o.Coalitions = coalitions
	return o, nil
}

// Pilots returns per-player career statistics, best score first, keyed by UCID.
//
// Like the manager, only the LATEST snapshot per (mission, player) is summed:
// the hook resends cumulative counters every few seconds, so summing every row
// would multiply a player's totals by the sampling rate. Deaths and friendly
// fire are derived from events, resolving the DCS player id per mission.
func (s *Store) Pilots(ctx context.Context) ([]PilotStats, error) {
	where := s.missionFilter("ps.mission_id")
	rows, err := s.pool.Query(ctx, `
		WITH latest AS (
			SELECT MAX(id) AS id
			FROM v_stats_player_stats ps
			WHERE 1=1`+where+`
			GROUP BY ps.mission_id, ps.player_id
		)
		SELECT p.id, COALESCE(p.ucid,''), p.name,
		       COUNT(DISTINCT ps.mission_id),
		       COALESCE(SUM(ps.score),0),
		       COALESCE(SUM(ps.kills_air),0), COALESCE(SUM(ps.kills_car),0), COALESCE(SUM(ps.kills_ship),0),
		       COALESCE(SUM(ps.crashes),0), COALESCE(SUM(ps.ejects),0), COALESCE(SUM(ps.landings),0),
		       AVG(NULLIF(ps.ping,0))
		FROM v_stats_player_stats ps
		JOIN v_stats_players p ON p.id = ps.player_id
		WHERE ps.id IN (SELECT id FROM latest)
		GROUP BY p.id, p.ucid, p.name
		ORDER BY COALESCE(SUM(ps.score),0) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PilotStats
	byID := map[int64]int{}
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

	deaths, ff, err := s.eventCountsByPlayer(ctx)
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
// each event's DCS player id within its own mission.
func (s *Store) eventCountsByPlayer(ctx context.Context) (deaths, ff map[int64]int, err error) {
	resolver := map[[2]int64]int64{}
	where := s.missionFilter("ps.mission_id")
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ps.mission_id, ps.dcs_player_id, ps.player_id
		FROM v_stats_player_stats ps
		WHERE ps.dcs_player_id IS NOT NULL AND ps.mission_id IS NOT NULL`+where)
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

	erows, err := s.pool.Query(ctx, `
		SELECT mission_id, event, args FROM v_stats_events
		WHERE event IN ('pilot_death','friendly_fire') AND mission_id IS NOT NULL`+s.missionFilter("mission_id"))
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
			deaths[playerID]++
		case "friendly_fire":
			ff[playerID]++
		}
	}
	return deaths, ff, erows.Err()
}

// Weapons aggregates weapon performance from kill and friendly-fire events.
func (s *Store) Weapons(ctx context.Context) ([]WeaponStats, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT event, args FROM v_stats_events WHERE event IN ('kill','friendly_fire')`+s.missionFilter("mission_id"))
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
func (s *Store) Engines(ctx context.Context) ([]EngineStats, error) {
	byType := map[string]*EngineStats{}
	ensure := func(typeID string) *EngineStats {
		if e, ok := byType[typeID]; ok {
			return e
		}
		e := &EngineStats{TypeID: typeID, Category: classify(typeID)}
		byType[typeID] = e
		return e
	}

	where := s.missionFilter("mission_id")
	rows, err := s.pool.Query(ctx, `SELECT args FROM v_stats_events WHERE event = 'kill'`+where)
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

	// Sorties: how many snapshots a player occupied each type. GROUP BY counts
	// them; SELECT DISTINCT would have made every type exactly 1.
	rows2, err := s.pool.Query(ctx,
		`SELECT unit_type, COUNT(*) FROM v_stats_player_stats
		 WHERE unit_type IS NOT NULL AND unit_type <> ''`+where+`
		 GROUP BY unit_type`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var unitType string
		var n int
		if err := rows2.Scan(&unitType, &n); err != nil {
			return nil, err
		}
		ensure(unitType).Sorties = n
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
// each player in each mission.
func (s *Store) Coalitions(ctx context.Context) ([]CoalitionStats, error) {
	where := s.missionFilter("ps.mission_id")
	rows, err := s.pool.Query(ctx, `
		WITH latest AS (
			SELECT MAX(id) AS id
			FROM v_stats_player_stats ps
			WHERE 1=1`+where+`
			GROUP BY ps.mission_id, ps.player_id
		)
		SELECT ps.side,
		       COALESCE(SUM(ps.score),0),
		       COALESCE(SUM(ps.kills_air + ps.kills_car + ps.kills_ship),0),
		       COUNT(DISTINCT ps.player_id)
		FROM v_stats_player_stats ps
		WHERE ps.id IN (SELECT id FROM latest)
		GROUP BY ps.side`)
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
		out = append(out, CoalitionStats{Coalition: sideName(side), Score: score, Kills: kills, Players: players})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortCoalitions(out)
	return out, nil
}

// Network aggregates ping quality per player.
func (s *Store) Network(ctx context.Context) ([]NetworkStats, error) {
	where := s.missionFilter("ps.mission_id")
	rows, err := s.pool.Query(ctx, `
		SELECT p.name, COUNT(*), AVG(NULLIF(ps.ping,0)), COALESCE(MAX(ps.ping),0)
		FROM v_stats_player_stats ps
		JOIN v_stats_players p ON p.id = ps.player_id
		WHERE 1=1`+where+`
		GROUP BY p.id, p.name`)
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

// MissionRow is one mission for the missions tab / export.
type MissionRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Theatre   string `json:"theatre"`
	Source    string `json:"source"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt"`
	Winner    string `json:"winner"`
}

// Missions returns the manager's missions, newest first.
func (s *Store) Missions(ctx context.Context, limit int) ([]MissionRow, error) {
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	q := `SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
	             COALESCE(ended_at,0), COALESCE(winner,'')
	      FROM v_stats_missions`
	if !s.includeTest {
		q += ` WHERE source <> 'test'`
	}
	q += ` ORDER BY id DESC LIMIT $1`

	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MissionRow
	for rows.Next() {
		var m MissionRow
		if err := rows.Scan(&m.ID, &m.Name, &m.Theatre, &m.Source, &m.StartedAt, &m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountTotals returns raw row counts for the dashboard footer.
func (s *Store) CountTotals(ctx context.Context) (Counts, error) {
	var c Counts
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM v_stats_missions WHERE 1=1`+s.missionFilter("id")+`),
			(SELECT COUNT(*) FROM v_stats_players),
			(SELECT COUNT(*) FROM v_stats_events WHERE 1=1`+s.missionFilter("mission_id")+`),
			(SELECT COUNT(*) FROM v_stats_debriefs WHERE 1=1`+s.missionFilter("mission_id")+`),
			(SELECT COUNT(*) FROM v_stats_track_positions WHERE 1=1`+s.missionFilter("mission_id")+`)`,
	).Scan(&c.Missions, &c.Players, &c.Events, &c.Debriefs, &c.Positions)
	return c, err
}

// Point is one value of a time series (from real event timestamps, so it needs
// no periodic sampling: the manager already stores every event).
type Point struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

// EventSeries returns a daily count of one event kind over the last N days. This
// replaces the old snapshot-based trends: the plugin reads the manager's own
// timestamps, so the series is exact and complete.
func (s *Store) EventSeries(ctx context.Context, eventKind string, days int) ([]Point, error) {
	if days <= 0 || days > 3650 {
		days = 30
	}
	// The cutoff is computed in Go and passed as a bigint: comparing it directly
	// to real_ts avoids any interval-typing surprise in PostgreSQL.
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()

	q := `SELECT to_timestamp((real_ts / 1000.0)::double precision)::date AS d, COUNT(*)`
	args := []any{cutoff}
	q += ` FROM v_stats_events WHERE real_ts >= $1`
	if eventKind != "" {
		q += ` AND event = $2`
		args = append(args, eventKind)
	}
	q += s.missionFilter("mission_id")
	q += ` GROUP BY d ORDER BY d ASC`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var d time.Time
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		out = append(out, Point{At: d, Value: float64(n)})
	}
	return out, rows.Err()
}

// --- helpers -----------------------------------------------------------------

func ensureWeapon(m map[string]*WeaponStats, name string) *WeaponStats {
	if w, ok := m[name]; ok {
		return w
	}
	w := &WeaponStats{Weapon: name, VictimsByType: map[string]int{}, KillersByType: map[string]int{}}
	m[name] = w
	return w
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

func sortWeapons(out []WeaponStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kills != out[j].Kills {
			return out[i].Kills > out[j].Kills
		}
		return out[i].Weapon < out[j].Weapon
	})
}

func sortEngines(out []EngineStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kills != out[j].Kills {
			return out[i].Kills > out[j].Kills
		}
		if out[i].Deaths != out[j].Deaths {
			return out[i].Deaths > out[j].Deaths
		}
		return out[i].TypeID < out[j].TypeID
	})
}

func sortCoalitions(out []CoalitionStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Coalition < out[j].Coalition
	})
}

func sortNetwork(out []NetworkStats) {
	sort.SliceStable(out, func(i, j int) bool { return out[i].AvgPing > out[j].AvgPing })
}
