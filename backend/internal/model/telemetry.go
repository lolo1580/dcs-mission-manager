package model

// Telemetry is a full ownship state sample sent by Export.lua.
//
// Availability depends on the server's `allow_ownship_export` setting: in
// multiplayer, only your own aircraft (and, on a server, the aircraft it owns)
// can be exported this way.
type Telemetry struct {
	// Type is always "telemetry".
	Type string `json:"type"`

	UnitID    string `json:"unitId,omitempty"`
	Name      string `json:"name,omitempty"`
	UnitType  string `json:"unitType,omitempty"`
	Coalition string `json:"coalition,omitempty"`

	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	AltASL  float64 `json:"altAsl"`
	AltAGL  float64 `json:"altAgl,omitempty"`
	Heading float64 `json:"heading"`

	IAS   float64 `json:"ias,omitempty"` // indicated airspeed, m/s
	TAS   float64 `json:"tas,omitempty"` // true airspeed, m/s
	Mach  float64 `json:"mach,omitempty"`
	AoA   float64 `json:"aoa,omitempty"` // radians
	Pitch float64 `json:"pitch,omitempty"`
	Bank  float64 `json:"bank,omitempty"`
	Yaw   float64 `json:"yaw,omitempty"`
	G     float64 `json:"g,omitempty"`   // load factor, G
	VSI   float64 `json:"vsi,omitempty"` // vertical speed, m/s

	ModelTime float64 `json:"modelTime,omitempty"`
}

// Sample is one persisted position/telemetry point. Non-ownship aircraft only
// fill the position fields.
type Sample struct {
	ID        int64  `json:"-"`
	MissionID int64  `json:"-"`
	UnitID    string `json:"unitId"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	Coalition string `json:"coalition,omitempty"`
	Category  string `json:"category,omitempty"`

	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Alt     float64 `json:"alt"`
	Heading float64 `json:"heading"`
	Speed   float64 `json:"speed,omitempty"` // m/s (TAS when available)
	G       float64 `json:"g,omitempty"`
	AoA     float64 `json:"aoa,omitempty"`
	Ownship bool    `json:"ownship,omitempty"`

	RealTS int64 `json:"realTs"`
}
