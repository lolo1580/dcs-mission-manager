package db

import (
	"database/sql"
	"time"

	"dcsmanager/internal/model"
)

// Store is the persistence contract the rest of the manager depends on. It is
// implemented today by the pure-Go SQLite store (*DB); a second implementation
// (PostgreSQL) can be added without touching the consumers, which all talk to
// this interface rather than to *DB.
//
// The interface is deliberately the union of what the consumers need, not every
// method *DB has: SQL() stays off it on purpose (it is an escape hatch for tests
// and migrations), and the analytics queries go through Query/QueryRow instead.
type Store interface {
	// Lifecycle.
	Close() error

	// Missions.
	EnsureMission(name, theatre string) (int64, error)
	EnsureMissionTagged(name, theatre, source string) (int64, error)
	StartMission(name, theatre, source string) (int64, error)
	OpenMissionID() int64
	EndOpenMission(winner string) error
	UpgradeMissionSource(id int64, source string) error
	CountMissions(source string) (int, error)
	Missions(limit int) ([]model.Mission, error)
	MissionsWithSource(source string, limit int) ([]model.Mission, error)
	MissionsSince(sinceID int64, limit int) ([]model.Mission, error)

	// Events, chat and statistics snapshots.
	SaveEvent(missionID int64, e model.Event) error
	SaveChat(missionID int64, c model.Chat) error
	SaveStats(missionID, playerID int64, p model.Player) error
	RecentEvents(limit int) ([]model.Event, error)
	RecentChat(limit int) ([]model.Chat, error)
	EventsSince(sinceID int64, limit int) ([]model.Event, error)
	ChatSince(sinceID int64, limit int) ([]model.Chat, error)

	// Player identities.
	UpsertPlayer(ucid, name string) (int64, error)

	// Position tracking.
	SaveSamples(missionID int64, samples []model.Sample) error
	SaveLoss(missionID int64, s model.Sample) error
	Heatmap(missionID int64, source string, grid float64, limit int) ([]HeatPoint, error)
	Trails(missionID int64, limitUnits, maxPointsPerUnit int) (map[string][]TrailPoint, error)
	PruneTracking(olderThan time.Duration) (int64, error)

	// Debriefs.
	SaveDebrief(rec model.Debrief) (model.Debrief, error)
	Debriefs(limit int) ([]model.Debrief, error)
	Debrief(id int64) (model.Debrief, error)

	// Maintenance.
	PurgeMission(id int64) (PurgeResult, error)
	PurgeSource(source string) (PurgeResult, error)
	PurgeAll() (PurgeResult, error)

	// Querier lets the statistics package run its own read queries. It is on the
	// interface (rather than exposing the raw *sql.DB) so a non-SQLite
	// implementation can rebind placeholders to its own dialect.
	Querier
}

// Querier is the read-only SQL surface the analytics package needs. A SQLite
// implementation passes queries through unchanged; a PostgreSQL one rewrites the
// `?` placeholders to `$1, $2, …` before handing them to its driver.
type Querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Compile-time proof that the SQLite store satisfies the contract.
var _ Store = (*DB)(nil)

// Query runs a read query through the underlying handle. It exists so that
// consumers never reach for SQL() directly.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.sql.Query(query, args...)
}

// QueryRow runs a single-row read query through the underlying handle.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.sql.QueryRow(query, args...)
}
