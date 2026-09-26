package model

// Debrief is the persisted, parsed summary of one mission's debrief.log.
type Debrief struct {
	ID        int64  `json:"id"`
	MissionID int64  `json:"missionId,omitempty"`
	Mission   string `json:"mission,omitempty"`
	Theatre   string `json:"theatre,omitempty"`
	// Raw is the original debrief.log text, kept for re-parsing.
	Raw string `json:"-"`
	// Parsed is the structured summary (events, world state, aggregates).
	Parsed DebriefData `json:"parsed"`
	// Size is the byte size of the original file.
	Size int `json:"size"`
	// CreatedAt is unix ms.
	CreatedAt int64 `json:"createdAt"`
}

// DebriefData is the JSON-serialised shape of a parsed debrief, kept separate
// from package debrief so the database layer has no dependency on the parser.
type DebriefData struct {
	MissionFilePath string         `json:"missionFilePath"`
	Callsign        string         `json:"callsign"`
	Result          int            `json:"result"`
	MissionTime     float64        `json:"missionTime"`
	Events          []DebriefEvent `json:"events"`
	WorldState      []DebriefUnit  `json:"worldState"`
	Summary         DebriefSummary `json:"summary"`
}

// DebriefEvent is one event line of a debrief.
type DebriefEvent struct {
	Type               string  `json:"type"`
	T                  float64 `json:"t"`
	EventID            int64   `json:"eventId"`
	InitiatorPilot     string  `json:"initiatorPilot,omitempty"`
	InitiatorUnitType  string  `json:"initiatorUnitType,omitempty"`
	InitiatorCoalition int     `json:"initiatorCoalition,omitempty"`
	Target             string  `json:"target,omitempty"`
	Weapon             string  `json:"weapon,omitempty"`
	Place              string  `json:"place,omitempty"`
	PlaceDisplayName   string  `json:"placeDisplayName,omitempty"`
	Comment            string  `json:"comment,omitempty"`
}

// DebriefUnit is one entry of the final world state.
type DebriefUnit struct {
	UnitID    int     `json:"unitId"`
	Type      string  `json:"type"`
	Coalition string  `json:"coalition"`
	Country   int     `json:"country"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Alt       float64 `json:"alt"`
	Speed     float64 `json:"speed"`
	Dead      bool    `json:"dead"`
}

// DebriefSummary holds aggregate counts.
type DebriefSummary struct {
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
