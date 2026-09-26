// Package ingest persists live messages to the database. It is the bridge
// between the TCP listener (package tcp) and durable storage (package db).
package ingest

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"dcsmm/internal/db"
	"dcsmm/internal/live"
	"dcsmm/internal/model"
)

// Writer persists DCS messages, ensuring a mission is open and resolving player
// identities before writing statistics.
type Writer struct {
	db   *db.DB
	live *live.Store

	mu        sync.Mutex
	missionID int64
}

// New creates a writer backed by the given database and live store.
func New(database *db.DB, store *live.Store) *Writer {
	return &Writer{db: database, live: store}
}

// Handle persists a single message. Errors are logged, never returned: losing a
// database write must not break the live feed.
func (w *Writer) Handle(m model.Message) {
	switch m.Type {
	case "mission":
		w.handleMission(m)
	case "event":
		w.handleEvent(m)
	case "chat":
		w.handleChat(m)
	case "players":
		w.handlePlayers(m)
	}
}

func (w *Writer) handleMission(m model.Message) {
	if m.Phase == "start" {
		theatre := m.Theatre
		if theatre == "" {
			theatre = "Caucasus"
		}
		id, err := w.db.EnsureMission(m.Name, theatre)
		if err != nil {
			log.Printf("ingest: ensure mission: %v", err)
			return
		}
		w.setMissionID(id)
		return
	}
	if m.Phase == "end" {
		if err := w.db.EndOpenMission(m.Winner); err != nil {
			log.Printf("ingest: end mission: %v", err)
		}
		w.setMissionID(0)
	}
}

func (w *Writer) handleEvent(m model.Message) {
	if err := w.db.SaveEvent(w.missionIDFor(m), model.Event{
		Event:  m.Event,
		Args:   m.Args,
		Detail: m.Detail,
		T:      m.T,
		RealTS: time.Now().UnixMilli(),
	}); err != nil {
		log.Printf("ingest: save event: %v", err)
	}
}

func (w *Writer) handleChat(m model.Message) {
	if err := w.db.SaveChat(w.missionIDFor(m), model.Chat{
		From:    m.From,
		Message: m.Message,
		RealTS:  time.Now().UnixMilli(),
	}); err != nil {
		log.Printf("ingest: save chat: %v", err)
	}
}

func (w *Writer) handlePlayers(m model.Message) {
	missionID := w.missionIDFor(m)
	for _, p := range m.Players {
		pid, err := w.db.UpsertPlayer(p.UCID, p.Name)
		if err != nil {
			log.Printf("ingest: upsert player %q: %v", p.Name, err)
			continue
		}
		// Only snapshot players actually doing something (in a slot or with
		// stats), to avoid flooding the table with idle spectators.
		if p.Slot == "" && p.Score == 0 && p.KillsCar+p.KillsAir+p.KillsShip == 0 {
			continue
		}
		if err := w.db.SaveStats(missionID, pid, p); err != nil {
			log.Printf("ingest: save stats %q: %v", p.Name, err)
		}
	}
}

// missionIDFor returns the id of the open mission, opening a default one when
// DCS did not announce a mission start (for example a mid-mission reconnect).
func (w *Writer) missionIDFor(m model.Message) int64 {
	if id := w.currentMissionID(); id != 0 {
		return id
	}
	name := m.Name
	if name == "" {
		name = "Mission inconnue"
	}
	id, err := w.db.EnsureMission(name, "Caucasus")
	if err != nil {
		log.Printf("ingest: ensure mission: %v", err)
		return 0
	}
	w.setMissionID(id)
	return id
}

func (w *Writer) currentMissionID() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.missionID != 0 {
		return w.missionID
	}
	return 0
}

func (w *Writer) setMissionID(id int64) {
	w.mu.Lock()
	w.missionID = id
	w.mu.Unlock()
}

// EventLabel renders a compact human label for an event, used in logs and by
// the UI when a structured rendering is not available.
func EventLabel(e model.Event) string {
	if len(e.Args) == 0 {
		return e.Event
	}
	parts := make([]string, 0, len(e.Args))
	for _, a := range e.Args {
		parts = append(parts, toString(a))
	}
	return e.Event + " " + strings.Join(parts, ", ")
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// JSON numbers decode as float64; drop a trailing .0 for readability.
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}
