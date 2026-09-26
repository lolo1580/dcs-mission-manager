// Package live keeps the recent, in-memory state of the session: game events,
// connected players and chat. It is the hot path for the UI, while the database
// (package db) is the durable record.
package live

import (
	"sync"

	"dcsmm/internal/model"
)

// Store holds recent events, players and chat in a concurrency-safe way.
type Store struct {
	mu sync.RWMutex

	events      []model.Event
	eventsMax   int
	nextEventID int64

	chat       []model.Chat
	chatMax    int
	nextChatID int64

	players map[int]model.Player

	mission *model.Mission
}

// New creates a store capping events and chat history.
func New(eventsMax, chatMax int) *Store {
	if eventsMax <= 0 {
		eventsMax = 1000
	}
	if chatMax <= 0 {
		chatMax = 500
	}
	return &Store{
		eventsMax: eventsMax,
		chatMax:   chatMax,
		players:   make(map[int]model.Player),
	}
}

// AddEvent appends an event, assigning it an id, and returns the stored copy.
func (s *Store) AddEvent(e model.Event) model.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextEventID++
	e.ID = s.nextEventID
	s.events = append(s.events, e)
	if len(s.events) > s.eventsMax {
		s.events = s.events[len(s.events)-s.eventsMax:]
	}
	return e
}

// Events returns a copy of the recent events, oldest first.
func (s *Store) Events() []model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Event, len(s.events))
	copy(out, s.events)
	return out
}

// AddChat appends a chat message.
func (s *Store) AddChat(c model.Chat) model.Chat {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextChatID++
	c.ID = s.nextChatID
	s.chat = append(s.chat, c)
	if len(s.chat) > s.chatMax {
		s.chat = s.chat[len(s.chat)-s.chatMax:]
	}
	return c
}

// Chat returns a copy of the recent chat messages, oldest first.
func (s *Store) Chat() []model.Chat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Chat, len(s.chat))
	copy(out, s.chat)
	return out
}

// SetPlayers replaces the current player roster.
func (s *Store) SetPlayers(players []model.Player) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.players = make(map[int]model.Player, len(players))
	for _, p := range players {
		s.players[p.ID] = p
	}
}

// Players returns the current roster sorted by side then id.
func (s *Store) Players() []model.Player {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Player, 0, len(s.players))
	for _, p := range s.players {
		out = append(out, p)
	}
	// Stable ordering: red, blue, spectators, then by id.
	sortPlayers(out)
	return out
}

// StartMission records the beginning of a mission.
func (s *Store) StartMission(m model.Mission) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mission = &m
}

// EndMission closes the current mission.
func (s *Store) EndMission(winner string, at int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mission != nil {
		s.mission.Winner = winner
		s.mission.EndedAt = at
	}
}

// Mission returns the current mission, if any.
func (s *Store) Mission() (model.Mission, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mission == nil {
		return model.Mission{}, false
	}
	return *s.mission, true
}

func sortPlayers(p []model.Player) {
	// Insertion sort keeps the implementation dependency-free and the counts
	// are small (tens of players at most).
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && less(p[j], p[j-1]); j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}

func less(a, b model.Player) bool {
	rank := func(side int) int {
		switch side {
		case 1:
			return 0
		case 2:
			return 1
		default:
			return 2
		}
	}
	if ra, rb := rank(a.Side), rank(b.Side); ra != rb {
		return ra < rb
	}
	return a.ID < b.ID
}
