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

	"dcsmanager/internal/db"
	"dcsmanager/internal/live"
	"dcsmanager/internal/model"
)

// Writer persists DCS messages, ensuring a mission is open and resolving player
// identities before writing statistics.
type Writer struct {
	db   *db.DB
	live *live.Store

	mu sync.Mutex
	// sourceFn reports the source of the running session ("live" or "test").
	// It is a function rather than a value because the live UDP path may only
	// discover that a session is simulated after it has started.
	sourceFn func() string
	// missionID is the mission written to, or 0 when none is open.
	missionID int64
}

// New creates a writer backed by the given database and live store.
func New(database *db.DB, store *live.Store) *Writer {
	return &Writer{db: database, live: store}
}

// SetSourceFunc installs the callback reporting the running session's source.
// Until it is set, sessions are attributed to SourceLive.
func (w *Writer) SetSourceFunc(fn func() string) {
	w.mu.Lock()
	w.sourceFn = fn
	w.mu.Unlock()
}

func (w *Writer) currentSource() string {
	w.mu.Lock()
	fn := w.sourceFn
	w.mu.Unlock()
	if fn == nil {
		return db.SourceLive
	}
	if s := fn(); db.ValidSource(s) {
		return s
	}
	return db.SourceLive
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
		id, err := w.db.EnsureMissionTagged(m.Name, theatre, w.currentSource())
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
	missionID := w.missionIDFor(m)
	w.upgradeSource(missionID)
	if err := w.db.SaveEvent(missionID, model.Event{
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
	missionID := w.missionIDFor(m)
	w.upgradeSource(missionID)
	if err := w.db.SaveChat(missionID, model.Chat{
		From:    m.From,
		Message: m.Message,
		RealTS:  time.Now().UnixMilli(),
	}); err != nil {
		log.Printf("ingest: save chat: %v", err)
	}
}

func (w *Writer) handlePlayers(m model.Message) {
	missionID := w.missionIDFor(m)
	w.upgradeSource(missionID)
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

// upgradeSource promotes a mission to "test" when the running session has been
// recognised as simulated. It is called before writing, because the mission may
// have been opened while it still looked real.
func (w *Writer) upgradeSource(missionID int64) {
	if missionID == 0 {
		return
	}
	if err := w.db.UpgradeMissionSource(missionID, w.currentSource()); err != nil {
		log.Printf("ingest: upgrade mission source: %v", err)
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
		name = "Unknown mission"
	}
	id, err := w.db.EnsureMissionTagged(name, "Caucasus", w.currentSource())
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
