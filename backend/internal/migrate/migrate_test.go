package migrate_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/db"
	"dcsmanager/internal/db/postgres"
	"dcsmanager/internal/migrate"
	"dcsmanager/internal/model"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx" for the admin connection
)

// pgSchema prepares a dedicated schema for this test package, so it never
// collides with the conformance suite that uses `public` when both packages run
// in parallel. It returns a DSN scoped to that schema, and skips without a base
// DSN.
func pgSchema(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DCSMANAGER_TEST_POSTGRES_DSN")
	if base == "" {
		t.Skip("set DCSMANAGER_TEST_POSTGRES_DSN to run the migration integration test")
	}

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("admin open: %v", err)
	}
	defer admin.Close()
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "DROP SCHEMA IF EXISTS migrate_test CASCADE"); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA migrate_test"); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "search_path=migrate_test"
}

// seedSource builds a small SQLite database through the manager's own store, so
// the columns and encodings are exactly what a real install produces.
func seedSource(t *testing.T) (path string, want wantCounts) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "source.db")
	src, err := db.Open(path)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer src.Close()

	mid, err := src.EnsureMissionTagged("Free Flight", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := src.SaveEvent(mid, model.Event{Event: "kill", Args: []any{i, "F-16C_50"}, RealTS: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := src.SaveChat(mid, model.Chat{From: "pilot", Message: "hi", RealTS: 1}); err != nil {
		t.Fatal(err)
	}
	pid, err := src.UpsertPlayer("u-viper", "Viper")
	if err != nil {
		t.Fatal(err)
	}
	if err := src.SaveStats(mid, pid, model.Player{ID: 7, Name: "Viper", Side: 2, KillsAir: 3, Score: 120}); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveSamples(mid, []model.Sample{{UnitID: "u1", Type: "F-16C_50", Category: "plane", Lat: 41.2, Lng: 41.1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.SaveDebrief(model.Debrief{MissionID: mid, Mission: "Free Flight", Theatre: "Caucasus", Raw: "x", Size: 1}); err != nil {
		t.Fatal(err)
	}
	if err := src.EndOpenMission("red"); err != nil {
		t.Fatal(err)
	}

	return path, wantCounts{missions: 1, events: 3, chat: 1, players: 1, stats: 1, positions: 1, debriefs: 1}
}

type wantCounts struct {
	missions, events, chat, players, stats, positions, debriefs int
}

// TestMigrateCopiesEverything checks a full copy preserves row counts and ids,
// and that re-running is a no-op (idempotent).
func TestMigrateCopiesEverything(t *testing.T) {
	scoped := pgSchema(t)
	ctx := context.Background()

	sqlitePath, want := seedSource(t)

	// Create the destination schema by opening the PostgreSQL store once.
	dest, err := postgres.Open(scoped)
	if err != nil {
		t.Fatalf("postgres open: %v", err)
	}
	_ = dest.Close()

	first, err := migrate.Run(ctx, sqlitePath, scoped)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if first.Total == 0 {
		t.Fatal("migration copied nothing")
	}

	// Re-running must not duplicate anything.
	second, err := migrate.Run(ctx, sqlitePath, scoped)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.Total != 0 {
		t.Fatalf("second run copied %d rows, want 0 (idempotent)", second.Total)
	}

	// Verify through the PostgreSQL store.
	pg, err := postgres.Open(scoped)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer pg.Close()

	count := func(q string) int {
		t.Helper()
		var n int
		if err := pg.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", q, err)
		}
		return n
	}
	checks := map[string]int{
		"SELECT COUNT(*) FROM missions":        want.missions,
		"SELECT COUNT(*) FROM events":          want.events,
		"SELECT COUNT(*) FROM chat":            want.chat,
		"SELECT COUNT(*) FROM players":         want.players,
		"SELECT COUNT(*) FROM player_stats":    want.stats,
		"SELECT COUNT(*) FROM track_positions": want.positions,
		"SELECT COUNT(*) FROM debriefs":        want.debriefs,
	}
	for q, w := range checks {
		if got := count(q); got != w {
			t.Errorf("%s = %d, want %d", q, got, w)
		}
	}

	// The mission keeps its winner and end, and the event ids are preserved.
	missions, err := pg.Missions(10)
	if err != nil || len(missions) != 1 {
		t.Fatalf("Missions = %+v, %v", missions, err)
	}
	if missions[0].Winner != "red" || missions[0].EndedAt == 0 {
		t.Errorf("mission end not preserved: %+v", missions[0])
	}
	events, err := pg.EventsSince(0, 10)
	if err != nil || len(events) != 3 {
		t.Fatalf("EventsSince = %+v, %v", events, err)
	}
	if events[0].ID != 1 || events[2].ID != 3 {
		t.Errorf("event ids changed: %+v", events)
	}
}

// TestMigrateSequenceAdvanced proves the identity sequences are moved past the
// copied ids, so the manager's next insert does not collide.
func TestMigrateSequenceAdvanced(t *testing.T) {
	scoped := pgSchema(t)
	ctx := context.Background()
	sqlitePath, _ := seedSource(t)

	dest, err := postgres.Open(scoped)
	if err != nil {
		t.Fatal(err)
	}
	_ = dest.Close()

	if _, err := migrate.Run(ctx, sqlitePath, scoped); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pg, err := postgres.Open(scoped)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()

	// A fresh mission must get an id strictly greater than the copied one.
	id, err := pg.StartMission("After migration", "Syria", db.SourceLive)
	if err != nil {
		t.Fatalf("StartMission after migration: %v", err)
	}
	if id <= 1 {
		t.Fatalf("new mission id = %d, want > 1 (sequence not advanced)", id)
	}
}
