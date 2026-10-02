package dcsdata

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dcsmanager/internal/lua"
)

// AircraftCareer is one airframe's record in the player's logbook.
type AircraftCareer struct {
	// Type is the DCS aircraft type (M-2000C, F-16C_50…), as DCS keys it.
	Type        string  `json:"type"`
	FlightHours float64 `json:"flightHours"`
	Landings    int     `json:"landings"`
	Deaths      int     `json:"deaths"`
	Ejections   int     `json:"ejections"`
	Refuelings  int     `json:"refuelings"`
	Nighttime   int     `json:"nighttime"`
	Daytime     int     `json:"daytime"`
	AAKills     int     `json:"aaKills"`
	AGKills     int     `json:"agKills"`
	Naval       int     `json:"naval"`
	Static      int     `json:"static"`
	TotalScore  int     `json:"totalScore"`
}

// PlayerCareer is one pilot profile in the logbook.
type PlayerCareer struct {
	Name         string `json:"name"`
	Rank         string `json:"rank,omitempty"`
	Squadron     string `json:"squadron,omitempty"`
	Invulnerable bool   `json:"invulnerable,omitempty"`
	// Awards lists the medal ids DCS recorded for this player.
	Awards []int `json:"awards,omitempty"`
	// Aggregate holds the career totals DCS keeps outside the per-aircraft
	// tables (flightHours, missionsCount, landings, aaKills…), left raw because
	// DCS's set of keys varies with the module.
	Aggregate map[string]any `json:"aggregate"`
	// Aircraft is the per-airframe breakdown, sorted by flight hours.
	Aircraft []AircraftCareer `json:"aircraft"`
	// TotalFlightHours is the sum of the per-aircraft hours, a convenient summary.
	TotalFlightHours float64 `json:"totalFlightHours"`
}

// Logbook is the parsed MissionEditor/logbook.lua.
type Logbook struct {
	CurrentPlayer string         `json:"currentPlayer,omitempty"`
	Players       []PlayerCareer `json:"players"`
}

// LogbookPath returns MissionEditor/logbook.lua inside a Saved Games folder.
func LogbookPath(savedGames string) string {
	return filepath.Join(savedGames, "MissionEditor", "logbook.lua")
}

// LoadLogbook parses the player's career logbook. A missing file is not an error.
func LoadLogbook(savedGames string) (Logbook, error) {
	data, err := os.ReadFile(LogbookPath(savedGames))
	if err != nil {
		if os.IsNotExist(err) {
			return Logbook{}, nil
		}
		return Logbook{}, err
	}
	root, err := lua.Parse(data)
	if err != nil {
		return Logbook{}, err
	}
	return parseLogbook(root), nil
}

func parseLogbook(root map[string]any) Logbook {
	lb, _ := root["logbook"].(map[string]any)
	if lb == nil {
		return Logbook{}
	}
	out := Logbook{CurrentPlayer: str(lb["currentPlayerName"])}

	for _, entry := range asList(lb["players"]) {
		p, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		career := PlayerCareer{
			Name:         str(p["name"]),
			Rank:         str(p["rankName"]),
			Squadron:     str(p["squadronName"]),
			Invulnerable: truthy(p["invulnerable"]),
			Awards:       numericKeys(p["awards"]),
			Aggregate:    map[string]any{},
		}

		if stats, ok := p["statistics"].(map[string]any); ok {
			for key, value := range stats {
				// A table under statistics is one airframe's record; anything else
				// is a career total DCS keeps flat.
				if sub, ok := value.(map[string]any); ok {
					career.Aircraft = append(career.Aircraft, parseAircraft(key, sub))
					continue
				}
				career.Aggregate[key] = value
			}
		}

		sort.SliceStable(career.Aircraft, func(i, j int) bool {
			return career.Aircraft[i].FlightHours > career.Aircraft[j].FlightHours
		})
		for _, a := range career.Aircraft {
			career.TotalFlightHours += a.FlightHours
		}
		out.Players = append(out.Players, career)
	}

	sort.SliceStable(out.Players, func(i, j int) bool {
		return strings.ToLower(out.Players[i].Name) < strings.ToLower(out.Players[j].Name)
	})
	return out
}

func parseAircraft(typ string, m map[string]any) AircraftCareer {
	return AircraftCareer{
		Type:        typ,
		FlightHours: num(m["flightHours"]),
		Landings:    int(num(m["landings"])),
		Deaths:      int(num(m["deaths"])),
		Ejections:   int(num(m["ejections"])),
		Refuelings:  int(num(m["refuelings"])),
		Nighttime:   int(num(m["nighttime"])),
		Daytime:     int(num(m["daytime"])),
		AAKills:     int(num(m["aaKills"])),
		AGKills:     int(num(m["agKills"])),
		Naval:       int(num(m["naval"])),
		Static:      int(num(m["static"])),
		TotalScore:  int(num(m["totalScore"])),
	}
}

// numericKeys returns the numeric keys of a table, sorted ascending. DCS uses
// them for award ids ([2] = {}, [22] = {}).
func numericKeys(v any) []int {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	var out []int
	for k := range m {
		if n, ok := numericKeyValue(k); ok {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

func numericKeyValue(k string) (int, bool) {
	n := 0
	if k == "" {
		return 0, false
	}
	for _, r := range k {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// num reads a number, tolerating the string form DCS sometimes writes.
func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// truthy reports whether a DCS boolean cell is set.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	default:
		return false
	}
}
