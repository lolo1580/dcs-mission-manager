package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// These tests exercise the real PostgreSQL SQL (jsonb casts, ON CONFLICT,
// DISTINCT ON, batch upserts, cursors). They run only when a database is
// provided, so `go test ./...` stays green without one:
//
//	$env:PLUGIN_TEST_DATABASE_URL = "postgres://dcs:dcs@localhost:5432/stats_test?sslmode=disable"
//	go test ./...
//
// Point it at a THROWAWAY database: the test drops the plugin tables first.

func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	url := os.Getenv("PLUGIN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PLUGIN_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	s, err := OpenStore(ctx, url)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(s.Close)

	if _, err := s.pool.Exec(ctx,
		`DROP TABLE IF EXISTS stat_snapshots, sync_runs, sync_cursor, events, chat, missions`); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := s.Migrate(ctx, schemaSQL); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s, ctx
}

func TestStoreSnapshotDedupAndLatest(t *testing.T) {
	s, ctx := testStore(t)

	ok, err := s.InsertSnapshot(ctx, "a", "overview", "career", false, []byte(`{"kills":1}`))
	if err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	// Identical payload: skipped.
	ok, err = s.InsertSnapshot(ctx, "a", "overview", "career", false, []byte(`{"kills":1}`))
	if err != nil || ok {
		t.Fatalf("duplicate insert should be skipped: ok=%v err=%v", ok, err)
	}
	// Changed payload: stored.
	ok, err = s.InsertSnapshot(ctx, "a", "overview", "career", false, []byte(`{"kills":2}`))
	if err != nil || !ok {
		t.Fatalf("changed insert: ok=%v err=%v", ok, err)
	}

	latest, err := s.Latest(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || latest[0].Kind != "overview" || latest[0].Scope != "career" {
		t.Fatalf("latest = %+v", latest)
	}

	raw, err := s.LatestPayload(ctx, "a", "overview", "career")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("payload not valid json: %v", err)
	}
	if payload["kills"].(float64) != 2 {
		t.Fatalf("latest payload should be kills=2, got %v", payload["kills"])
	}

	// Another instance is isolated.
	other, err := s.Latest(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("instance b should be empty, got %+v", other)
	}
}

func TestStoreOverviewSeries(t *testing.T) {
	s, ctx := testStore(t)
	for _, v := range []string{`{"kills":1}`, `{"kills":3}`, `{"kills":2}`} {
		if _, err := s.InsertSnapshot(ctx, "a", "overview", "career", false, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}

	points, err := s.OverviewSeries(ctx, "a", "career", "kills", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("points = %d, want 3", len(points))
	}
	// Oldest first.
	if points[0].Value != 1 || points[2].Value != 2 {
		t.Fatalf("series order = %+v", points)
	}

	// An unknown metric is refused rather than injected into SQL.
	if _, err := s.OverviewSeries(ctx, "a", "career", "malicious; DROP", 10); !errors.Is(err, ErrUnknownMetric) {
		t.Fatalf("unknown metric should be rejected, got %v", err)
	}
}

func TestStoreCursorMonotonic(t *testing.T) {
	s, ctx := testStore(t)

	if id, err := s.Cursor(ctx, "a", "events"); err != nil || id != 0 {
		t.Fatalf("initial cursor = %d, %v", id, err)
	}
	if err := s.SetCursor(ctx, "a", "events", 10); err != nil {
		t.Fatal(err)
	}
	// A smaller value must not rewind.
	if err := s.SetCursor(ctx, "a", "events", 5); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Cursor(ctx, "a", "events"); id != 10 {
		t.Fatalf("cursor = %d, want 10 (never rewinds)", id)
	}
	// A larger value advances.
	if err := s.SetCursor(ctx, "a", "events", 20); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Cursor(ctx, "a", "events"); id != 20 {
		t.Fatalf("cursor = %d, want 20", id)
	}
}

func TestStoreUpsertEventsAndCounts(t *testing.T) {
	s, ctx := testStore(t)

	events := []Event{
		{ID: 1, Event: "kill", Args: []any{1, "F-16C_50", 2, 2, "Su-27", 1, "AIM-120C"}, RealTS: 100},
		{ID: 2, Event: "friendly_fire", Args: []any{1, "Mk-82", 2}, RealTS: 101},
	}
	n, err := s.UpsertEvents(ctx, "a", events)
	if err != nil || n != 2 {
		t.Fatalf("upsert = %d, %v", n, err)
	}
	// Idempotent.
	if _, err := s.UpsertEvents(ctx, "a", events); err != nil {
		t.Fatal(err)
	}

	counts, err := s.Counts(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if counts.Events != 2 {
		t.Fatalf("events = %d, want 2 (idempotent)", counts.Events)
	}

	// Filter by event kind.
	kills, err := s.Events(ctx, "a", "kill", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(kills) != 1 || kills[0].Event != "kill" {
		t.Fatalf("filtered events = %+v", kills)
	}
	// args round-trips as a JSON array.
	if len(kills[0].Args) != 7 || kills[0].Args[6] != "AIM-120C" {
		t.Fatalf("args did not round-trip: %+v", kills[0].Args)
	}
	// Instance isolation.
	if c, _ := s.Counts(ctx, "b"); c.Events != 0 {
		t.Fatalf("instance b events = %d, want 0", c.Events)
	}
}

func TestStoreUpsertChatAndMissions(t *testing.T) {
	s, ctx := testStore(t)

	if _, err := s.UpsertChat(ctx, "a", []Chat{{ID: 1, From: "pilot", Message: "hello", RealTS: 5}}); err != nil {
		t.Fatal(err)
	}

	// A mission is created open, then ends: the second upsert must update it.
	if _, err := s.UpsertMissions(ctx, "a", []Mission{
		{ID: 1, Name: "M1", Theatre: "Caucasus", Source: "live", StartedAt: 10},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertMissions(ctx, "a", []Mission{
		{ID: 1, Name: "M1", Theatre: "Caucasus", Source: "live", StartedAt: 10, EndedAt: 99, Winner: "red"},
	}); err != nil {
		t.Fatal(err)
	}

	missions, err := s.Missions(ctx, "a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 {
		t.Fatalf("missions = %d, want 1", len(missions))
	}
	if missions[0].EndedAt != 99 || missions[0].Winner != "red" {
		t.Fatalf("mission end not updated: %+v", missions[0])
	}

	counts, _ := s.Counts(ctx, "a")
	if counts.Chat != 1 || counts.Missions != 1 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestStoreInstancesAndSyncRuns(t *testing.T) {
	s, ctx := testStore(t)

	if _, err := s.InsertSnapshot(ctx, "a", "overview", "career", false, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertSnapshot(ctx, "b", "overview", "career", false, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	instances, err := s.Instances(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 2 || instances[0] != "b" {
		t.Fatalf("instances = %v, want b first", instances)
	}

	if err := s.RecordSync(ctx, "a", false, "boom"); err != nil {
		t.Fatal(err)
	}
	last, err := s.LastSync(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if last.OK || last.Error != "boom" {
		t.Fatalf("last sync = %+v", last)
	}
}
