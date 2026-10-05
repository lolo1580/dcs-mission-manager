package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the plugin's PostgreSQL persistence. It is intentionally small: the
// plugin stores snapshots and reads them back, it does not aggregate.
//
// Every row is scoped by an `instance` name (the manager the row came from), so
// several plugin instances can safely share one database.
type Store struct {
	pool *pgxpool.Pool
}

// ErrUnknownMetric is returned when a requested metric is not one the overview
// snapshot can expose. The metric name is whitelisted because it is inlined in
// the SQL (a jsonb key cannot be a bound parameter in every position).
var ErrUnknownMetric = errors.New("unknown metric")

// overviewMetrics are the numeric fields of an overview snapshot that can be
// turned into a time series.
var overviewMetrics = map[string]bool{
	"kills":        true,
	"deaths":       true,
	"crashes":      true,
	"ejections":    true,
	"friendlyFire": true,
	"events":       true,
	"missions":     true,
	"players":      true,
}

// OpenStore connects to PostgreSQL and verifies the connection.
func OpenStore(ctx context.Context, dbURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Migrate applies the embedded schema. Every statement is idempotent.
func (s *Store) Migrate(ctx context.Context, schema string) error {
	_, err := s.pool.Exec(ctx, schema)
	return err
}

// Snapshot is one stored API payload.
type Snapshot struct {
	Instance   string          `json:"instance"`
	Kind       string          `json:"kind"`
	Scope      string          `json:"scope"`
	CapturedAt time.Time       `json:"capturedAt"`
	Payload    json.RawMessage `json:"payload"`
}

// InsertSnapshot stores one API snapshot and reports whether a row was written.
// It skips the insert when the payload is byte-identical to the most recent one
// for the same (instance, kind, scope), so periodic polling does not accumulate
// identical rows.
func (s *Store) InsertSnapshot(ctx context.Context, instance, kind, scope string, includeTest bool, payload []byte) (bool, error) {
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])

	var prev string
	err := s.pool.QueryRow(ctx, `
		SELECT payload_hash FROM stat_snapshots
		WHERE instance = $1 AND kind = $2 AND scope = $3
		ORDER BY captured_at DESC
		LIMIT 1`, instance, kind, scope).Scan(&prev)
	switch {
	case err == nil && prev == hash:
		return false, nil
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return false, err
	}

	// The explicit ::text::jsonb cast keeps the encoding unambiguous: the body
	// is sent as text, then parsed by PostgreSQL.
	_, err = s.pool.Exec(ctx, `
		INSERT INTO stat_snapshots(instance, kind, scope, include_test, payload, payload_hash)
		VALUES($1, $2, $3, $4, $5::text::jsonb, $6)`,
		instance, kind, scope, includeTest, string(payload), hash)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Latest returns the most recent snapshot for every (kind, scope) of one
// instance.
func (s *Store) Latest(ctx context.Context, instance string) ([]Snapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (kind, scope) instance, kind, scope, captured_at, payload
		FROM stat_snapshots
		WHERE instance = $1
		ORDER BY kind, scope, captured_at DESC`, instance)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		var sn Snapshot
		var raw []byte
		if err := rows.Scan(&sn.Instance, &sn.Kind, &sn.Scope, &sn.CapturedAt, &raw); err != nil {
			return nil, err
		}
		sn.Payload = json.RawMessage(raw)
		out = append(out, sn)
	}
	return out, rows.Err()
}

// LatestPayload returns the raw JSON of the most recent snapshot for one key.
func (s *Store) LatestPayload(ctx context.Context, instance, kind, scope string) ([]byte, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT payload FROM stat_snapshots
		WHERE instance = $1 AND kind = $2 AND scope = $3
		ORDER BY captured_at DESC
		LIMIT 1`, instance, kind, scope).Scan(&raw)
	return raw, err
}

// Point is one value of a time series.
type Point struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

// OverviewSeries extracts one numeric metric from the overview snapshots of a
// scope, oldest first. This is exactly the kind of trend the manager's own UI
// cannot show, because it keeps no history.
func (s *Store) OverviewSeries(ctx context.Context, instance, scope, metric string, limit int) ([]Point, error) {
	if !overviewMetrics[metric] {
		return nil, ErrUnknownMetric
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT captured_at, COALESCE((payload ->> '`+metric+`')::numeric, 0)
		FROM stat_snapshots
		WHERE instance = $1 AND kind = 'overview' AND scope = $2
		ORDER BY captured_at ASC
		LIMIT $3`, instance, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.At, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Instances lists the manager instances present in the database, so a dashboard
// can switch between them. The requested default is placed first when present.
func (s *Store) Instances(ctx context.Context, prefer string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT instance FROM stat_snapshots ORDER BY instance`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Put the preferred instance first.
	if prefer != "" {
		for i, name := range out {
			if name == prefer {
				out = append([]string{name}, append(out[:i:i], out[i+1:]...)...)
				break
			}
		}
	}
	return out, nil
}

// SyncRun records the outcome of one polling pass.
type SyncRun struct {
	At    time.Time `json:"at"`
	OK    bool      `json:"ok"`
	Error string    `json:"error,omitempty"`
}

// RecordSync appends the outcome of a polling pass.
func (s *Store) RecordSync(ctx context.Context, instance string, ok bool, errMsg string) error {
	var e *string
	if errMsg != "" {
		e = &errMsg
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_runs(instance, ok, error) VALUES($1, $2, $3)`, instance, ok, e)
	return err
}

// LastSync returns the most recent polling outcome, if any.
func (s *Store) LastSync(ctx context.Context, instance string) (SyncRun, error) {
	var r SyncRun
	var errMsg *string
	err := s.pool.QueryRow(ctx, `
		SELECT at, ok, error FROM sync_runs
		WHERE instance = $1
		ORDER BY at DESC LIMIT 1`, instance).Scan(&r.At, &r.OK, &errMsg)
	if err != nil {
		return r, err
	}
	if errMsg != nil {
		r.Error = *errMsg
	}
	return r, nil
}

// --- incremental mirror ------------------------------------------------------

// Cursor returns the stored cursor for a feed ("events", "chat" or "missions"),
// 0 if none.
func (s *Store) Cursor(ctx context.Context, instance, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT last_id FROM sync_cursor WHERE instance = $1 AND name = $2`, instance, name).Scan(&id)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// SetCursor advances a cursor. GREATEST guarantees it never rewinds, so a late
// or out-of-order batch cannot make the plugin fetch the same rows again.
func (s *Store) SetCursor(ctx context.Context, instance, name string, id int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_cursor(instance, name, last_id, updated_at) VALUES($1, $2, $3, now())
		ON CONFLICT (instance, name) DO UPDATE
		SET last_id = GREATEST(sync_cursor.last_id, EXCLUDED.last_id),
		    updated_at = now()`, instance, name, id)
	return err
}

// UpsertEvents mirrors a batch of manager events, idempotently.
func (s *Store) UpsertEvents(ctx context.Context, instance string, events []Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		args, _ := json.Marshal(e.Args)
		var detail any
		if len(e.Detail) > 0 {
			detail = string(e.Detail)
		}
		batch.Queue(`
			INSERT INTO events(instance, id, event, args, detail, t, real_ts)
			VALUES($1, $2, $3, $4::text::jsonb, $5::text::jsonb, $6, $7)
			ON CONFLICT (instance, id) DO UPDATE
			SET event = EXCLUDED.event, args = EXCLUDED.args,
			    detail = EXCLUDED.detail, t = EXCLUDED.t, real_ts = EXCLUDED.real_ts`,
			instance, e.ID, e.Event, string(args), detail, e.T, e.RealTS)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range events {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

// UpsertChat mirrors a batch of chat messages, idempotently.
func (s *Store) UpsertChat(ctx context.Context, instance string, chat []Chat) (int, error) {
	if len(chat) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, c := range chat {
		batch.Queue(`
			INSERT INTO chat(instance, id, "from", message, real_ts)
			VALUES($1, $2, $3, $4, $5)
			ON CONFLICT (instance, id) DO UPDATE
			SET "from" = EXCLUDED."from", message = EXCLUDED.message, real_ts = EXCLUDED.real_ts`,
			instance, c.ID, c.From, c.Message, c.RealTS)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range chat {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(chat), nil
}

// UpsertMissions mirrors a batch of missions, idempotently.
func (s *Store) UpsertMissions(ctx context.Context, instance string, missions []Mission) (int, error) {
	if len(missions) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, m := range missions {
		batch.Queue(`
			INSERT INTO missions(instance, id, name, theatre, source, started_at, ended_at, winner)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (instance, id) DO UPDATE
			SET name = EXCLUDED.name, theatre = EXCLUDED.theatre, source = EXCLUDED.source,
			    started_at = EXCLUDED.started_at, ended_at = EXCLUDED.ended_at, winner = EXCLUDED.winner`,
			instance, m.ID, m.Name, nullable(m.Theatre), m.Source, m.StartedAt, m.EndedAt, nullable(m.Winner))
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range missions {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(missions), nil
}

// Counts reports how many events, chat messages and missions are mirrored for an
// instance. Used by health and the dashboard.
type Counts struct {
	Events   int64 `json:"events"`
	Chat     int64 `json:"chat"`
	Missions int64 `json:"missions"`
}

// Counts returns the mirrored row counts for one instance.
func (s *Store) Counts(ctx context.Context, instance string) (Counts, error) {
	var c Counts
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM events   WHERE instance = $1),
			(SELECT COUNT(*) FROM chat     WHERE instance = $1),
			(SELECT COUNT(*) FROM missions WHERE instance = $1)`, instance).
		Scan(&c.Events, &c.Chat, &c.Missions)
	return c, err
}

// Missions returns mirrored missions for an instance, newest first.
func (s *Store) Missions(ctx context.Context, instance string, limit int) ([]Mission, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, COALESCE(theatre,''), COALESCE(source,'live'), started_at,
		       COALESCE(ended_at,0), COALESCE(winner,'')
		FROM missions WHERE instance = $1
		ORDER BY started_at DESC LIMIT $2`, instance, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Mission
	for rows.Next() {
		var m Mission
		if err := rows.Scan(&m.ID, &m.Name, &m.Theatre, &m.Source, &m.StartedAt, &m.EndedAt, &m.Winner); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Events returns mirrored events for an instance, newest first, optionally
// filtered by event kind. Used by the export endpoint.
func (s *Store) Events(ctx context.Context, instance, eventKind string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100000 {
		limit = 10000
	}
	q := `SELECT id, event, args, detail, t, real_ts FROM events WHERE instance = $1`
	args := []any{instance}
	if eventKind != "" {
		q += ` AND event = $2`
		args = append(args, eventKind)
	}
	q += ` ORDER BY id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var argsJSON, detailJSON []byte
		if err := rows.Scan(&e.ID, &e.Event, &argsJSON, &detailJSON, &e.T, &e.RealTS); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(argsJSON, &e.Args)
		if len(detailJSON) > 0 {
			e.Detail = json.RawMessage(detailJSON)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
