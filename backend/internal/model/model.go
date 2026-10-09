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
	// Options maps DCS mission difficulty/view options (e.g. optionsView), kept
	// with the session.
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

// UnmarshalJSON decodes a Message, tolerating the shapes DCS actually sends.
// In particular the player roster arrives as a JSON object ({}) when the server
// has no connected player, because the Lua serializer cannot tell an empty table
// from an empty object; the strict slice decode used to reject the whole line,
// dropping the message. A one-element object keyed by the player id is treated
// the same way, for an older hook that iterated with pairs().
func (m *Message) UnmarshalJSON(data []byte) error {
	type alias Message
	aux := struct {
		Players json.RawMessage `json:"players"`
		Options json.RawMessage `json:"options"`
		*alias
	}{alias: (*alias)(m)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// An empty table is serialized as [] on the Lua side (an empty table is a
	// list there). Mission options are a map, so an empty [] means "no options"
	// rather than a decoding error.
	if len(aux.Options) > 0 {
		var opts map[string]any
		if err := json.Unmarshal(aux.Options, &opts); err == nil {
			m.Options = opts
		}
	}

	if len(aux.Players) == 0 {
		return nil
	}

	var list []Player
	if err := json.Unmarshal(aux.Players, &list); err == nil {
		m.Players = list
		return nil
	}

	// `{}` from an empty roster, or an object keyed by player id.
	var byKey map[string]Player
	if err := json.Unmarshal(aux.Players, &byKey); err == nil {
		if len(byKey) == 0 {
			m.Players = []Player{}
			return nil
		}
		m.Players = make([]Player, 0, len(byKey))
		for _, p := range byKey {
			m.Players = append(m.Players, p)
		}
		return nil
	}

	// Let the strict error surface (a truly malformed roster).
	var strict []Player
	return json.Unmarshal(aux.Players, &strict)
}

// Command is a message the backend pushes to the DCS hook over the same TCP
// connection the hook uses to report. The hook executes it on receipt.
type Command struct {
	Type string `json:"type"` // always "command"
	// Command selects the action: "chat" injects a chat message into DCS.
	Command string `json:"command"`
	// Message is the chat text (command = "chat").
	Message string `json:"message,omitempty"`
	// From is the sender shown in DCS ("Server" by default).
	From string `json:"from,omitempty"`
}

// CommandChat is the command id that injects a chat message.
const CommandChat = "chat"

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
	Country   string `json:"country,omitempty"` // DCS slot country, when available
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
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Theatre string `json:"theatre,omitempty"`
	// Source is "live" for a session recorded from DCS, or "test" for one
	// produced by the test tools. Statistics exclude "test" by default.
	Source    string `json:"source,omitempty"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt,omitempty"`
	Winner    string `json:"winner,omitempty"`
}
