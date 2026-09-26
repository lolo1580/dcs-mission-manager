// Package tcp receives newline-delimited JSON messages from the DCS Hooks script.
//
// The Lua side opens a TCP connection to the backend and streams one JSON object
// per line: game events, player rosters, chat and mission transitions. This is
// also the channel the backend uses to push commands back to DCS (Phase 2+).
package tcp

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"time"

	"dcsmm/internal/live"
	"dcsmm/internal/model"
)

// Listener accepts DCS hook connections and feeds the live store.
type Listener struct {
	live *live.Store

	// OnEvent, when set, is called for every received message (for persistence
	// and broadcasting). It must not block.
	OnMessage func(model.Message)
}

// NewListener creates a listener writing into store.
func NewListener(store *live.Store) *Listener {
	return &Listener{live: store}
}

// Listen opens a TCP socket bound to addr.
func Listen(addr string) (*net.TCPListener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", tcpAddr)
}

// Serve accepts connections until the listener is closed. Each connection is
// handled in its own goroutine, so DCS can reconnect without blocking.
func (l *Listener) Serve(ln *net.TCPListener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("tcp: accept: %v", err)
			return
		}
		go l.handle(conn)
	}
}

func (l *Listener) handle(conn net.Conn) {
	defer conn.Close()
	log.Printf("tcp: dcs connected from %s", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m model.Message
		if err := json.Unmarshal(line, &m); err != nil {
			log.Printf("tcp: decode: %v", err)
			continue
		}
		l.dispatch(&m)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("tcp: read: %v", err)
	}
	log.Printf("tcp: dcs disconnected from %s", conn.RemoteAddr())
}

func (l *Listener) dispatch(m *model.Message) {
	switch m.Type {
	case "event":
		l.live.AddEvent(model.Event{
			Event:  m.Event,
			Args:   m.Args,
			Detail: m.Detail,
			T:      m.T,
			RealTS: time.Now().UnixMilli(),
		})
	case "players":
		l.live.SetPlayers(m.Players)
	case "chat":
		l.live.AddChat(model.Chat{
			From:    m.From,
			Message: m.Message,
			RealTS:  time.Now().UnixMilli(),
		})
	case "mission":
		now := time.Now().UnixMilli()
		if m.Phase == "start" {
			l.live.StartMission(model.Mission{
				Name:      m.Name,
				Theatre:   m.Theatre,
				StartedAt: now,
			})
		} else if m.Phase == "end" {
			l.live.EndMission(m.Winner, now)
		}
	default:
		log.Printf("tcp: unknown message type %q", m.Type)
	}

	if l.OnMessage != nil {
		l.OnMessage(*m)
	}
}
