// Package state keeps the last known state of every unit reported by DCS.
//
// It is safe for concurrent use and expires units that have not been updated
// within a configurable TTL (DCS may stop reporting a unit without notice).
package state

import (
	"sync"
	"time"
)

// Unit is a single tracked object (ownship for the Phase 0 proof of concept).
type Unit struct {
	Name      string    `json:"name"`
	UnitType  string    `json:"unitType"`
	Coalition string    `json:"coalition"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Alt       float64   `json:"alt"`
	Heading   float64   `json:"heading"`
	ModelTime float64   `json:"modelTime"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store is a concurrency-safe in-memory unit store.
type Store struct {
	mu    sync.Mutex
	units map[string]*Unit
	ttl   time.Duration
}

// New creates a store whose entries expire after ttl. A non-positive ttl
// disables expiration.
func New(ttl time.Duration) *Store {
	return &Store{
		units: make(map[string]*Unit),
		ttl:   ttl,
	}
}

// Update inserts or replaces the state of a unit. The unit is keyed by its name.
func (s *Store) Update(u *Unit) {
	if u == nil || u.Name == "" {
		return
	}
	u.UpdatedAt = time.Now()
	s.mu.Lock()
	s.units[u.Name] = u
	s.mu.Unlock()
}

// Snapshot returns a copy of the current units, dropping expired entries.
func (s *Store) Snapshot() []Unit {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Unit, 0, len(s.units))
	for name, u := range s.units {
		if s.ttl > 0 && now.Sub(u.UpdatedAt) > s.ttl {
			delete(s.units, name)
			continue
		}
		out = append(out, *u)
	}
	return out
}

// Count returns the number of live units.
func (s *Store) Count() int {
	return len(s.Snapshot())
}
