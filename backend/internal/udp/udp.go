// Package udp receives telemetry from the DCS Lua scripts over UDP.
//
// The wire format is one JSON object per datagram, for example:
//
//	{"type":"ownship","name":"Player","unitType":"F-16C_50","coalition":"blue",
//	 "lat":41.5,"lng":41.8,"alt":5000,"heading":123,"modelTime":42.0}
package udp

import (
	"encoding/json"
	"log"
	"net"

	"dcsmm/internal/state"
)

// Message is a telemetry datagram sent by the DCS side.
type Message struct {
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	UnitType  string  `json:"unitType"`
	Coalition string  `json:"coalition"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	ModelTime float64 `json:"modelTime"`
}

// Listen opens a UDP socket bound to addr.
func Listen(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}

// Serve reads datagrams from conn until it is closed, updating store for each
// valid message. It is meant to run in its own goroutine.
func Serve(conn *net.UDPConn, store *state.Store) {
	buf := make([]byte, 65535)
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

		store.Update(&state.Unit{
			Name:      m.Name,
			UnitType:  m.UnitType,
			Coalition: m.Coalition,
			Lat:       m.Lat,
			Lng:       m.Lng,
			Alt:       m.Alt,
			Heading:   m.Heading,
			ModelTime: m.ModelTime,
		})
	}
}
