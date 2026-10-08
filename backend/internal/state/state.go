// Package state keeps the last known state of every unit reported by DCS.
//
// It is safe for concurrent use and expires units that have not been updated
// within a configurable TTL (DCS may stop reporting a unit without notice).
package state

import (
	"sync"
	"time"
)

// Unit is a single tracked object.
type Unit struct {
	// ID uniquely identifies the unit during a mission. DCS runtime id for
	// world objects, "ownship" for the local player.
	ID string `json:"id"`
	// Type is the DCS type identifier, e.g. "F-16C_50", "T-72B", "SA-10".
	Type string `json:"type"`
	// Label is an optional human name (pilot name for the ownship).
	Label string `json:"label,omitempty"`
	// Category is one of plane, heli, ground, ship, structure, other.
	Category string `json:"category"`
	// Coalition is blue, red or neutral.
	Coalition string `json:"coalition"`
	// Country is the DCS country name, when available.
	Country string `json:"country,omitempty"`

	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Alt     float64 `json:"alt"`
	Heading float64 `json:"heading"`

	// Telemetry fields, only populated for the ownship (Export.lua).
	Speed float64 `json:"speed,omitempty"` // m/s
	G     float64 `json:"g,omitempty"`     // load factor
	AoA   float64 `json:"aoa,omitempty"`   // radians

	// Ownship is true for the local player's aircraft.
	Ownship bool `json:"ownship"`

	// AgeMs is the time since the last update, filled in Snapshot.
	AgeMs int64 `json:"ageMs"`

	UpdatedAt time.Time `json:"-"`
}

// Store is a concurrency-safe in-memory unit store.
type Store struct {
	mu    sync.RWMutex
	units map[string]*Unit
	ttl   time.Duration
	max   int
	// lastUpdate is when a unit or export heartbeat last arrived. Unit positions
	// alone cannot show whether DCS is running: spectators and restricted servers
	// may export no positions while the simulation still advances.
	lastUpdate time.Time
	// everUpdated records whether any unit has ever been stored.
	everUpdated bool
}

// New creates a store whose entries expire after ttl. A non-positive ttl
// disables expiration. A non-positive max disables the size cap.
func New(ttl time.Duration, max int) *Store {
	return &Store{
		units: make(map[string]*Unit),
		ttl:   ttl,
		max:   max,
	}
}

// FeedStopped reports whether a previously active export feed has gone quiet.
// Silence can mean a pause, a stopped mission, or a broken export connection.
//
// A caller must not treat a stopped feed as a mass destruction: doing so records
// every unit as lost, which is exactly what a pause used to produce.
func (s *Store) FeedStopped() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.everUpdated {
		return false // no evidence that an active feed has stopped
	}
	if s.ttl <= 0 {
		return false // no TTL configured: cannot judge
	}
	return time.Since(s.lastUpdate) > s.ttl
}

// Touch records an export heartbeat without creating a fictitious map unit.
func (s *Store) Touch() {
	s.mu.Lock()
	s.lastUpdate = time.Now()
	s.everUpdated = true
	s.mu.Unlock()
}

// FeedAge returns how long ago telemetry last arrived, and whether any ever did.
func (s *Store) FeedAge() (time.Duration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.everUpdated {
		return 0, false
	}
	return time.Since(s.lastUpdate), true
}

// SimulateSilence makes the feed look as if nothing had arrived for the given
// duration. It exists so the pause behaviour can be tested without waiting.
func (s *Store) SimulateSilence(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUpdate = time.Now().Add(-d)
	s.everUpdated = true
}

// Update inserts or replaces the state of a unit. It returns false if the unit
// was rejected (empty id, or the store is full and the unit is unknown).
func (s *Store) Update(u *Unit) bool {
	if u == nil || u.ID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, known := s.units[u.ID]; !known && s.max > 0 && len(s.units) >= s.max {
		return false
	}
	now := time.Now()
	u.UpdatedAt = now
	s.lastUpdate = now
	s.everUpdated = true
	s.units[u.ID] = u
	return true
}

// UpdateAll updates several units and returns how many were accepted.
func (s *Store) UpdateAll(units []Unit) int {
	n := 0
	for i := range units {
		if s.Update(&units[i]) {
			n++
		}
	}
	return n
}

// Remove deletes a unit by id.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	delete(s.units, id)
	s.mu.Unlock()
}

// Snapshot returns a copy of the current units, dropping expired entries and
// filling in each unit's age in milliseconds.
func (s *Store) Snapshot() []Unit {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Unit, 0, len(s.units))
	for id, u := range s.units {
		age := now.Sub(u.UpdatedAt)
		if s.ttl > 0 && age > s.ttl {
			delete(s.units, id)
			continue
		}
		cp := *u
		cp.AgeMs = age.Milliseconds()
		out = append(out, cp)
	}
	return out
}

// Count returns the number of live units.
func (s *Store) Count() int {
	return len(s.Snapshot())
}
