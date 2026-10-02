// Package debrief turns a DCS debrief.log into a structured, queryable summary.
//
// debrief.log is a Lua data dump written by DCS at the end of a mission. It
// contains the mission file, the final world state and an ordered list of
// events (takeoff, land, kills, ejections, mission end…).
package debrief

import (
	"sort"
	"strconv"
	"strings"

	"dcsmanager/internal/lua"
	"dcsmanager/internal/model"
)

// Debrief is the parsed summary of one mission's debrief file.
type Debrief struct {
	MissionFilePath string  `json:"missionFilePath"`
	Callsign        string  `json:"callsign"`
	Result          int     `json:"result"`
	MissionTime     float64 `json:"missionTime"`
	MissionFileMark int64   `json:"missionFileMark"`

	Events     []Event `json:"events"`
	WorldState []Unit  `json:"worldState"`
}

// Event is one entry of the debrief's event log.
type Event struct {
	Type               string  `json:"type"`
	T                  float64 `json:"t"`
	EventID            int64   `json:"eventId"`
	LinkedEventID      int64   `json:"linkedEventId"`
	InitiatorPilot     string  `json:"initiatorPilot,omitempty"`
	InitiatorUnitType  string  `json:"initiatorUnitType,omitempty"`
	InitiatorCoalition int     `json:"initiatorCoalition,omitempty"`
	InitiatorMissionID string  `json:"initiatorMissionId,omitempty"`
	Target             string  `json:"target,omitempty"`
	TargetMissionID    string  `json:"targetMissionId,omitempty"`
	Weapon             string  `json:"weapon,omitempty"`
	Place              string  `json:"place,omitempty"`
	PlaceDisplayName   string  `json:"placeDisplayName,omitempty"`
	Comment            string  `json:"comment,omitempty"`
	// Raw keeps the original key/values for fields we do not model, so nothing
	// is lost for later analysis.
	Raw map[string]any `json:"-"`
}

// Unit is one entry of the debrief's final world state.
type Unit struct {
	UnitID    int     `json:"unitId"`
	Type      string  `json:"type"`
	Coalition string  `json:"coalition"`
	Country   int     `json:"country"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	Speed     float64 `json:"speed"`
	Life      float64 `json:"life"`
	Dead      bool    `json:"dead"`
	GroupID   int     `json:"groupId,omitempty"`
	PilotName string  `json:"pilotName,omitempty"`
}

// Parse reads debrief.log content and returns a structured summary.
func Parse(data []byte) (*Debrief, error) {
	root, err := lua.Parse(data)
	if err != nil {
		return nil, err
	}

	d := &Debrief{
		MissionFilePath: stringOf(root["mission_file_path"]),
		Callsign:        stringOf(root["callsign"]),
		Result:          int(numberOf(root["result"])),
		MissionTime:     numberOf(root["mission_time"]),
		MissionFileMark: int64(numberOf(root["mission_file_mark"])),
	}

	for _, raw := range sliceOf(root["events"]) {
		if m, ok := raw.(map[string]any); ok {
			d.Events = append(d.Events, parseEvent(m))
		}
	}
	for _, raw := range sliceOf(root["world_state"]) {
		if m, ok := raw.(map[string]any); ok {
			d.WorldState = append(d.WorldState, parseUnit(m))
		}
	}

	sort.SliceStable(d.Events, func(i, j int) bool { return d.Events[i].T < d.Events[j].T })
	return d, nil
}

func parseEvent(m map[string]any) Event {
	return Event{
		Type:               stringOf(m["type"]),
		T:                  numberOf(m["t"]),
		EventID:            int64(numberOf(m["event_id"])),
		LinkedEventID:      int64(numberOf(m["linked_event_id"])),
		InitiatorPilot:     stringOf(m["initiatorPilotName"]),
		InitiatorUnitType:  stringOf(m["initiator_unit_type"]),
		InitiatorCoalition: int(numberOf(m["initiator_coalition"])),
		InitiatorMissionID: stringOf(m["initiatorMissionID"]),
		Target:             stringOf(m["target"]),
		TargetMissionID:    stringOf(m["targetMissionID"]),
		Weapon:             stringOf(m["weapon"]),
		Place:              stringOf(m["place"]),
		PlaceDisplayName:   stringOf(m["placeDisplayName"]),
		Comment:            stringOf(m["comment"]),
		Raw:                m,
	}
}

func parseUnit(m map[string]any) Unit {
	return Unit{
		UnitID:    int(numberOf(m["unitId"])),
		Type:      stringOf(m["type"]),
		Coalition: stringOf(m["coalition"]),
		Country:   int(numberOf(m["country"])),
		X:         numberOf(m["x"]),
		Y:         numberOf(m["y"]),
		Alt:       numberOf(m["alt"]),
		Heading:   numberOf(m["heading"]),
		Speed:     numberOf(m["speed"]),
		Life:      numberOf(m["life"]),
		Dead:      boolOf(m["dead"]),
		GroupID:   int(numberOf(m["groupId"])),
		PilotName: stringOf(m["pilotName"]),
	}
}

// Summary aggregates a debrief for display.
type Summary struct {
	Takeoffs   int            `json:"takeoffs"`
	Landings   int            `json:"landings"`
	Kills      int            `json:"kills"`
	Deaths     int            `json:"deaths"`
	Ejections  int            `json:"ejections"`
	Crashes    int            `json:"crashes"`
	Pilots     []string       `json:"pilots"`
	ByType     map[string]int `json:"byType"`
	MissionEnd string         `json:"missionEnd,omitempty"`
}

// Summarise computes aggregate counts from the event log.
func (d *Debrief) Summarise() Summary {
	s := Summary{ByType: make(map[string]int)}
	pilots := map[string]bool{}

	for _, e := range d.Events {
		if e.InitiatorPilot != "" {
			pilots[e.InitiatorPilot] = true
		}
		s.ByType[e.Type]++
		switch strings.ToLower(e.Type) {
		case "takeoff":
			s.Takeoffs++
		case "land", "landing":
			s.Landings++
		case "kill", "shot down":
			s.Kills++
		case "crash":
			s.Crashes++
		case "eject", "ejection":
			s.Ejections++
		case "pilot dead", "pilot_death", "dead":
			s.Deaths++
		case "mission end":
			s.MissionEnd = e.Comment
		}
	}

	for p := range pilots {
		s.Pilots = append(s.Pilots, p)
	}
	sort.Strings(s.Pilots)
	return s
}

// ToModel converts the parsed debrief into the transfer model used by the API
// and the database, so those layers do not depend on this package.
func (d *Debrief) ToModel() model.DebriefData {
	sum := d.Summarise()

	events := make([]model.DebriefEvent, 0, len(d.Events))
	for _, e := range d.Events {
		events = append(events, model.DebriefEvent{
			Type:               e.Type,
			T:                  e.T,
			EventID:            e.EventID,
			InitiatorPilot:     e.InitiatorPilot,
			InitiatorUnitType:  e.InitiatorUnitType,
			InitiatorCoalition: e.InitiatorCoalition,
			Target:             e.Target,
			Weapon:             e.Weapon,
			Place:              e.Place,
			PlaceDisplayName:   e.PlaceDisplayName,
			Comment:            e.Comment,
		})
	}

	units := make([]model.DebriefUnit, 0, len(d.WorldState))
	for _, u := range d.WorldState {
		units = append(units, model.DebriefUnit{
			UnitID:    u.UnitID,
			Type:      u.Type,
			Coalition: u.Coalition,
			Country:   u.Country,
			X:         u.X,
			Y:         u.Y,
			Alt:       u.Alt,
			Speed:     u.Speed,
			Dead:      u.Dead,
		})
	}

	return model.DebriefData{
		MissionFilePath: d.MissionFilePath,
		Callsign:        d.Callsign,
		Result:          d.Result,
		MissionTime:     d.MissionTime,
		Events:          events,
		WorldState:      units,
		Summary: model.DebriefSummary{
			Takeoffs:   sum.Takeoffs,
			Landings:   sum.Landings,
			Kills:      sum.Kills,
			Deaths:     sum.Deaths,
			Ejections:  sum.Ejections,
			Crashes:    sum.Crashes,
			Pilots:     sum.Pilots,
			ByType:     sum.ByType,
			MissionEnd: sum.MissionEnd,
		},
	}
}

// --- helpers ---------------------------------------------------------------

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

func numberOf(v any) float64 {
	f, _ := v.(float64)
	return f
}

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

func sliceOf(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		// Mixed tables are stored with numeric keys; recover them ordered.
		keys := make([]int, 0, len(t))
		byKey := make(map[int]any, len(t))
		for k, val := range t {
			n, err := strconv.Atoi(k)
			if err != nil {
				continue
			}
			keys = append(keys, n)
			byKey[n] = val
		}
		sort.Ints(keys)
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, byKey[k])
		}
		return out
	default:
		return nil
	}
}
