// Package udp receives telemetry from the DCS Lua scripts over UDP.
//
// The Export.lua script produces an activity heartbeat and two unit messages:
//
//	{"type":"heartbeat"}
//
//	{"type":"ownship","name":"Player","unitType":"F-16C_50","coalition":"blue",
//	 "lat":41.5,"lng":41.8,"alt":5000,"heading":123,"modelTime":42.0}
//
//	{"type":"world","count":123,"units":[
//	   {"id":"42","type":"T-72B","coalition":"red","country":"Russia",
//	    "lat":42.1,"lng":41.2,"alt":120,"heading":270}, ...]}
package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"dcsmanager/internal/category"
	"dcsmanager/internal/state"
)

// Message is a telemetry datagram sent by the DCS side.
type Message struct {
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	UnitType  string  `json:"unitType"`
	Coalition string  `json:"coalition"`
	Country   string  `json:"country"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	ModelTime float64 `json:"modelTime"`
	Units     []World `json:"units"`

	// Ownship telemetry (same "ownship" message, filled by Export.lua when the
	// server allows ownship export).
	TAS    float64 `json:"tas"`
	IAS    float64 `json:"ias"`
	Mach   float64 `json:"mach"`
	AoA    float64 `json:"aoa"`
	G      float64 `json:"g"`
	AltAGL float64 `json:"altAgl"`
}

// World is one entry of a "world" message.
type World struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Coalition string  `json:"coalition"`
	Country   string  `json:"country"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	Speed     float64 `json:"speed,omitempty"`
}

// Listener turns UDP datagrams into store updates.
type Listener struct {
	store      *state.Store
	classifier *category.Classifier
	// detectSource reports whether a payload was recognised as coming from the
	// test tools. Optional; nil disables detection.
	detectSource func(payload []byte) bool
}

// NewListener creates a listener updating store, classifying types with c.
func NewListener(store *state.Store, c *category.Classifier) *Listener {
	return &Listener{store: store, classifier: c}
}

// SetSourceDetector installs a callback invoked with the raw payload of every
// datagram that marks a unit as the local player. It lets the caller notice the
// test tools (which speak the same protocol as DCS) and tag the session
// accordingly. The callback must be cheap and non-blocking.
func (l *Listener) SetSourceDetector(fn func(payload []byte) bool) {
	l.detectSource = fn
}

// Listen opens a UDP socket bound to addr.
func Listen(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}

const ownshipID = "ownship"

// Serve reads datagrams from conn until it is closed, updating the store for
// each valid message. It is meant to run in its own goroutine.
func (l *Listener) Serve(conn *net.UDPConn) {
	buf := make([]byte, 1<<20) // world snapshots are larger than a single unit
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			// A closed socket is an expected shutdown path.
			log.Printf("udp: read: %v", err)
			return
		}

		var m Message
		if err := json.Unmarshal(buf[:n], &m); err != nil {
			log.Printf("udp: decode: %v", err)
			continue
		}
		l.dispatch(buf[:n], &m)
	}
}

// dispatch handles one decoded datagram. payload is the raw datagram, passed to
// the source detector before interpretation.
func (l *Listener) dispatch(payload []byte, m *Message) {
	// Detect simulated traffic before handling: only the ownship message is
	// checked, so the (much larger) world snapshots carry no cost.
	if l.detectSource != nil && m.Type == "ownship" {
		l.detectSource(payload)
	}
	l.handle(m)
}

func (l *Listener) handle(m *Message) {
	switch m.Type {
	case "heartbeat":
		l.store.Touch()
	case "ownship":
		l.store.Update(&state.Unit{
			ID:        ownshipID,
			Type:      m.UnitType,
			Label:     m.Name,
			Category:  l.classifier.Classify(m.UnitType),
			Coalition: m.Coalition,
			Country:   m.Country,
			Lat:       m.Lat,
			Lng:       m.Lng,
			Alt:       m.Alt,
			Heading:   m.Heading,
			Speed:     speedOf(m),
			G:         m.G,
			AoA:       m.AoA,
			Ownship:   true,
		})
	case "world":
		units := make([]state.Unit, 0, len(m.Units))
		for _, w := range m.Units {
			if w.ID == "" {
				continue
			}
			units = append(units, state.Unit{
				ID:        w.ID,
				Type:      w.Type,
				Category:  l.classifier.Classify(w.Type),
				Coalition: w.Coalition,
				Country:   w.Country,
				Lat:       w.Lat,
				Lng:       w.Lng,
				Alt:       w.Alt,
				Heading:   w.Heading,
				Speed:     w.Speed,
			})
		}
		l.store.UpdateAll(units)
	case "":
		log.Printf("udp: message without type, ignored")
	default:
		log.Printf("udp: unknown message type %q", m.Type)
	}
}

// FormatUnit is a small helper used by tests and logs.
func FormatUnit(u state.Unit) string {
	return fmt.Sprintf("%s/%s %s (%.4f, %.4f)", u.ID, u.Type, u.Coalition, u.Lat, u.Lng)
}

// speedOf returns the best available speed for an ownship message, preferring
// true airspeed and falling back to indicated airspeed.
func speedOf(m *Message) float64 {
	if m.TAS > 0 {
		return m.TAS
	}
	return m.IAS
}
