// Package tcp receives newline-delimited JSON messages from the DCS Hooks script,
// and pushes commands back to it.
//
// The Lua side opens a TCP connection to the backend and streams one JSON object
// per line: game events, player rosters, chat and mission transitions. The same
// connection is the command channel: the backend writes one JSON line back, and
// the hook executes it (for example a chat message injected into DCS).
package tcp

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"dcsmanager/internal/live"
	"dcsmanager/internal/model"
)

// Listener accepts DCS hook connections, feeds the live store, and can push
// commands back down the same connections.
type Listener struct {
	live *live.Store

	// OnOptions, when set, receives the mission difficulty/view options, which
	// are kept with the session.
	OnOptions func(map[string]any)

	// OnMessage, when set, is called for every received message (for persistence
	// and broadcasting). It must not block.
	OnMessage func(model.Message)

	mu    sync.Mutex
	conns map[*client]struct{}
	// undelivered counts commands that found no connected hook, so an operator
	// can tell "DCS is not running" from "the command was refused".
	undelivered int
}

// client is one hook connection, with a mutex so the read loop and a command
// writer never interleave their writes.
type client struct {
	mu   sync.Mutex
	conn net.Conn
}

// write sends one already-serialized line (including its trailing newline).
func (c *client) write(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.conn.Write(b)
	return err
}

// NewListener creates a listener writing into store.
func NewListener(store *live.Store) *Listener {
	return &Listener{live: store, conns: make(map[*client]struct{})}
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
	c := &client{conn: conn}
	l.addClient(c)
	defer func() {
		l.removeClient(c)
		conn.Close()
	}()
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

func (l *Listener) addClient(c *client) {
	l.mu.Lock()
	l.conns[c] = struct{}{}
	l.mu.Unlock()
}

func (l *Listener) removeClient(c *client) {
	l.mu.Lock()
	delete(l.conns, c)
	l.mu.Unlock()
}

// Connected reports how many hook connections are currently open. The UI uses it
// to tell whether DCS can receive a command.
func (l *Listener) Connected() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.conns)
}

// SendCommand serializes v as one JSON line and writes it to every connected
// hook. It returns the number of connections the command reached; zero means no
// hook is connected (DCS not running, or the scripts not installed).
//
// A failed write drops that client: the read loop will notice the closed socket
// and clean it up.
func (l *Listener) SendCommand(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("tcp: marshal command: %v", err)
		return 0
	}
	b = append(b, '\n')

	l.mu.Lock()
	clients := make([]*client, 0, len(l.conns))
	for c := range l.conns {
		clients = append(clients, c)
	}
	if len(clients) == 0 {
		l.undelivered++
	}
	l.mu.Unlock()

	sent := 0
	for _, c := range clients {
		if err := c.write(b); err != nil {
			log.Printf("tcp: command write: %v", err)
			continue
		}
		sent++
	}
	return sent
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
	case "debrief":
		// Handled downstream (chunk reassembly); nothing to do in the live store.
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
		if len(m.Options) > 0 && l.OnOptions != nil {
			l.OnOptions(m.Options)
		}
	default:
		log.Printf("tcp: unknown message type %q", m.Type)
	}

	if l.OnMessage != nil {
		l.OnMessage(*m)
	}
}
