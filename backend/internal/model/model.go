// Package model holds the types exchanged between the DCS Lua scripts and the
// backend, and persisted to the database.
package model

import "encoding/json"

// Message is one newline-delimited JSON object received over TCP from DCS.
//
// The "type" field selects which of the payload fields below is meaningful.
type Message struct {
	Type string `json:"type"`

	// type = "event"
	Event  string          `json:"event,omitempty"`
	Args   []any           `json:"args,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
	T      float64         `json:"t,omitempty"`

	// type = "players"
	Players []Player `json:"players,omitempty"`

	// type = "chat"
	From    string `json:"from,omitempty"`
	Message string `json:"message,omitempty"`

	// type = "mission"
	Phase   string `json:"phase,omitempty"` // start | end
	Name    string `json:"name,omitempty"`
	Theatre string `json:"theatre,omitempty"`
	Winner  string `json:"winner,omitempty"`
	// Options maps DCS mission difficulty/view options (e.g. optionsView) so the
	// backend can honour the mission's fog-of-war settings.
	Options map[string]any `json:"options,omitempty"`

	// type = "debrief" — debrief.log is sent in chunks because it can be large.
	// Data is base64-encoded to survive any byte sequence intact.
	TransferID string `json:"transferId,omitempty"`
	Chunk      int    `json:"chunk,omitempty"`
	Chunks     int    `json:"chunks,omitempty"`
	Data       string `json:"data,omitempty"`
	Size       int    `json:"size,omitempty"`

	// type = "telemetry" — full ownship state (Export.lua).
	Telemetry *Telemetry `json:"telemetry,omitempty"`
}

// Player is a connected client as reported by net.get_player_info / net.get_stat.
type Player struct {
	ID   int    `json:"id"`
	UCID string `json:"ucid,omitempty"`
	Name string `json:"name"`
	Side int    `json:"side"` // 0 spectator, 1 red, 2 blue
	Slot string `json:"slot,omitempty"`
	// UnitType is the DCS type of the aircraft the player occupies, resolved by
	// the Lua hook via Sim.getAvailableSlots. Empty when in spectators.
	UnitType  string `json:"unitType,omitempty"`
	Ping      int    `json:"ping"`
	Crashes   int    `json:"crashes"`
	KillsCar  int    `json:"killsCar"`
	KillsAir  int    `json:"killsAir"`
	KillsShip int    `json:"killsShip"`
	Score     int    `json:"score"`
	Landings  int    `json:"landings"`
	Ejects    int    `json:"ejects"`
	UpdatedAt int64  `json:"updatedAt"` // unix ms
}

// SideName returns a readable coalition name.
func (p Player) SideName() string {
	switch p.Side {
	case 1:
		return "red"
	case 2:
		return "blue"
	default:
		return "spectator"
	}
}

// Event is a game event captured from onGameEvent and persisted.
type Event struct {
	ID     int64           `json:"id"`
	Event  string          `json:"event"`
	Args   []any           `json:"args,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
	T      float64         `json:"t,omitempty"`
	RealTS int64           `json:"realTs"`
}

// Chat is a chat message captured from onChatMessage.
type Chat struct {
	ID      int64  `json:"id"`
	From    string `json:"from"`
	Message string `json:"message"`
	RealTS  int64  `json:"realTs"`
}

// Mission records a mission run.
type Mission struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Theatre   string `json:"theatre,omitempty"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt,omitempty"`
	Winner    string `json:"winner,omitempty"`
}
