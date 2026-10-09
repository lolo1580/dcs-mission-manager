package stats

import (
	"time"
)

// CareerPlace counts missions in which the pilot occupied an aircraft on a map
// or in a country. A mission may count for two countries if the pilot switched
// slots during it.
type CareerPlace struct {
	Name     string `json:"name"`
	Missions int    `json:"missions"`
}

// CareerDay is estimated time in an aircraft for one local calendar day.
type CareerDay struct {
	Date    string `json:"date"`
	Minutes int    `json:"minutes"`
}

type CareerInsights struct {
	Maps      []CareerPlace `json:"maps"`
	Countries []CareerPlace `json:"countries"`
	Days      []CareerDay   `json:"days"`
}

// CareerInsightsForPilot uses the selected pilot's UCID (or an anonymous
// callsign) for the career breakdown. Map/country counts follow the selected
// period. The activity calendar always covers the last 30 local days.
func (s *Service) CareerInsightsForPilot(sc Scope, ucid, name string, now time.Time) (CareerInsights, error) {
	result := CareerInsights{
		Maps: []CareerPlace{}, Countries: []CareerPlace{}, Days: make([]CareerDay, 30),
	}
	localNow := now.In(time.Local)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local)
	firstDay := today.AddDate(0, 0, -29)
	dayIndex := make(map[string]int, 30)
	for i := range result.Days {
		date := firstDay.AddDate(0, 0, i).Format("2006-01-02")
		result.Days[i].Date = date
		dayIndex[date] = i
	}
	if ucid == "" && name == "" {
		return result, nil
	}

	identity, identityArgs := pilotIdentity(ucid, name)
	where, scopeArgs := sc.filter("ps.mission_id")
	for _, grouping := range []struct {
		column string
		out    *[]CareerPlace
	}{
		{"m.theatre", &result.Maps},
		{"ps.country", &result.Countries},
	} {
		query := `SELECT ` + grouping.column + `, COUNT(DISTINCT ps.mission_id)
			FROM player_stats ps
			JOIN players p ON p.id = ps.player_id
			JOIN missions m ON m.id = ps.mission_id
			WHERE ` + identity + ` AND (COALESCE(ps.unit_type,'') <> '' OR COALESCE(ps.slot,'') <> '')
			  AND COALESCE(` + grouping.column + `,'') <> ''` + where + `
			GROUP BY ` + grouping.column + ` ORDER BY 2 DESC, 1 ASC LIMIT 8`
		args := append(append([]any{}, identityArgs...), scopeArgs...)
		rows, err := s.db.Query(query, args...)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var place CareerPlace
			if err := rows.Scan(&place.Name, &place.Missions); err != nil {
				rows.Close()
				return result, err
			}
			*grouping.out = append(*grouping.out, place)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, err
		}
		rows.Close()
	}

	// The Lua hook normally sends player snapshots every five seconds. Count
	// only the observed gaps up to 15 seconds, so a disconnect, long pause, or
	// manager restart never becomes hours of invented playtime.
	const maxGapMs int64 = 15_000
	rows, err := s.db.Query(`SELECT ps.mission_id, ps.real_ts,
		COALESCE(ps.unit_type,'') <> '' OR COALESCE(ps.slot,'') <> ''
		FROM player_stats ps
		JOIN players p ON p.id = ps.player_id
		JOIN missions m ON m.id = ps.mission_id
		WHERE `+identity+` AND m.source <> 'test'
		  AND ps.real_ts >= ? AND ps.real_ts <= ?
		ORDER BY ps.mission_id, ps.real_ts, ps.id`, append(append([]any{}, identityArgs...), firstDay.UnixMilli()-maxGapMs, now.UnixMilli())...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var lastMission, lastAt int64
	var lastInCockpit bool
	minutesMs := make([]int64, 30)
	for rows.Next() {
		var mission, at int64
		var inCockpit bool
		if err := rows.Scan(&mission, &at, &inCockpit); err != nil {
			return result, err
		}
		if mission == lastMission && lastInCockpit && inCockpit && at > lastAt && at-lastAt <= maxGapMs {
			start := lastAt
			if start < firstDay.UnixMilli() {
				start = firstDay.UnixMilli()
			}
			for start < at {
				instant := time.UnixMilli(start).In(time.Local)
				date := instant.Format("2006-01-02")
				nextDay := time.Date(instant.Year(), instant.Month(), instant.Day()+1, 0, 0, 0, 0, time.Local).UnixMilli()
				end := at
				if nextDay < end {
					end = nextDay
				}
				if index, ok := dayIndex[date]; ok {
					minutesMs[index] += end - start
				}
				start = end
			}
		}
		lastMission, lastAt, lastInCockpit = mission, at, inCockpit
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	for i, milliseconds := range minutesMs {
		result.Days[i].Minutes = int((milliseconds + 30_000) / 60_000)
	}
	return result, nil
}

func pilotIdentity(ucid, name string) (string, []any) {
	if ucid != "" {
		return "p.ucid = ?", []any{ucid}
	}
	return "p.name = ? AND COALESCE(p.ucid,'') = ''", []any{name}
}
