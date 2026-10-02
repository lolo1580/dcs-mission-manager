package db

import (
	"testing"

	"dcsmanager/internal/model"
)

// TestEnsureMissionTagged verifies that sessions are tagged at creation and that
// an already-open mission is upgraded when the session turns out to be
// simulated. The upgrade matters: a mission can be announced before the first
// test packet arrives, and it must not stay counted as real afterwards.
func TestEnsureMissionTagged(t *testing.T) {
	d := openTemp(t)

	// A live mission is created as live.
	live, err := d.EnsureMissionTagged("Real", "Caucasus", SourceLive)
	if err != nil {
		t.Fatalf("ensure live: %v", err)
	}
	if got := missionSource(t, d, live); got != SourceLive {
		t.Fatalf("want source %q, got %q", SourceLive, got)
	}

	// The same open mission is returned, but upgraded to test on request.
	same, err := d.EnsureMissionTagged("Real", "Caucasus", SourceTest)
	if err != nil {
		t.Fatalf("ensure test: %v", err)
	}
	if same != live {
		t.Fatalf("want the same open mission %d, got %d", live, same)
	}
	if got := missionSource(t, d, live); got != SourceTest {
		t.Fatalf("open mission should have been upgraded to %q, got %q", SourceTest, got)
	}

	// The default helper must not downgrade a test mission back to live.
	if _, err := d.EnsureMission("Real", "Caucasus"); err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	if got := missionSource(t, d, live); got != SourceTest {
		t.Fatalf("a test mission must not become live again, got %q", got)
	}

	// An unknown source falls back to live rather than being written as-is.
	if _, err := d.EnsureMissionTagged("Bogus", "Caucasus", "nonsense"); err != nil {
		t.Fatalf("ensure bogus: %v", err)
	}
	if got := missionSource(t, d, live); got != SourceTest {
		t.Fatalf("source unchanged expected, got %q", got)
	}
}

// TestPurgeSourceRemovesOnlyThatSource is the core guarantee: purging test data
// must never touch a real session.
func TestPurgeSourceRemovesOnlyThatSource(t *testing.T) {
	d := openTemp(t)

	// One live mission with a sample, then one test mission with a sample.
	liveID, err := d.EnsureMission("Live", "Caucasus")
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	if err := d.SaveSamples(liveID, sampleFor("live-unit")); err != nil {
		t.Fatalf("live samples: %v", err)
	}
	if err := d.EndOpenMission("blue"); err != nil {
		t.Fatalf("end: %v", err)
	}

	testID, err := d.EnsureMissionTagged("Test", "Caucasus", SourceTest)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if err := d.SaveSamples(testID, sampleFor("test-unit")); err != nil {
		t.Fatalf("test samples: %v", err)
	}
	if err := d.EndOpenMission("red"); err != nil {
		t.Fatalf("end: %v", err)
	}

	res, err := d.PurgeSource(SourceTest)
	if err != nil {
		t.Fatalf("purge test: %v", err)
	}
	if res.Missions != 1 {
		t.Fatalf("want 1 mission removed, got %d", res.Missions)
	}
	if got := res.Deleted["track_positions"]; got != 1 {
		t.Fatalf("want 1 sample removed, got %d", got)
	}

	// The live mission and its data must be intact.
	if n, err := d.CountMissions(SourceLive); err != nil || n != 1 {
		t.Fatalf("live missions: want 1, got %d (err=%v)", n, err)
	}
	if n, err := d.CountMissions(SourceTest); err != nil || n != 0 {
		t.Fatalf("test missions: want 0, got %d (err=%v)", n, err)
	}
	remaining, err := d.Trails(liveID, 10, 100)
	if err != nil {
		t.Fatalf("trails: %v", err)
	}
	if _, ok := remaining["live-unit"]; !ok {
		t.Fatal("the live unit's track should have survived the purge")
	}
}

// TestPurgeMissionRemovesLinkedRows checks that a single mission purge sweeps
// every table linked to it and leaves other missions alone.
func TestPurgeMissionRemovesLinkedRows(t *testing.T) {
	d := openTemp(t)

	keepID, err := d.EnsureMission("Keep", "Caucasus")
	if err != nil {
		t.Fatalf("keep: %v", err)
	}
	if err := d.EndOpenMission("blue"); err != nil {
		t.Fatalf("end keep: %v", err)
	}
	dropID, err := d.EnsureMission("Drop", "Caucasus")
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := d.SaveSamples(dropID, sampleFor("gone")); err != nil {
		t.Fatalf("samples: %v", err)
	}
	if err := d.SaveEvent(dropID, eventFor()); err != nil {
		t.Fatalf("event: %v", err)
	}
	pid, err := d.UpsertPlayer("u-1", "Ghost")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	if err := d.SaveChat(dropID, chatFor()); err != nil {
		t.Fatalf("chat: %v", err)
	}
	_ = pid

	res, err := d.PurgeMission(dropID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if res.Missions != 1 {
		t.Fatalf("want 1 mission removed, got %d", res.Missions)
	}
	for _, table := range []string{"track_positions", "events", "chat"} {
		if res.Deleted[table] == 0 {
			t.Errorf("expected at least one row removed from %s", table)
		}
	}

	// The other mission must still be there.
	missions, err := d.Missions(10)
	if err != nil {
		t.Fatalf("missions: %v", err)
	}
	if len(missions) != 1 {
		t.Fatalf("want 1 surviving mission, got %d", len(missions))
	}
	if missions[0].ID != keepID {
		t.Fatalf("want mission %d to survive, got %d", keepID, missions[0].ID)
	}
}

// TestMigrationAddsSourceColumn reproduces a database written before the source
// column existed, and checks that opening it upgrades it in place.
func TestMigrationAddsSourceColumn(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/old.db"

	// Simulate the old schema: missions without "source".
	old, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := old.SQL().Exec(`DROP TABLE missions`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := old.SQL().Exec(`
		CREATE TABLE missions (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL,
			theatre    TEXT,
			started_at INTEGER NOT NULL,
			ended_at   INTEGER,
			winner     TEXT
		)`); err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if _, err := old.SQL().Exec(
		`INSERT INTO missions(name, theatre, started_at) VALUES('Legacy', 'Caucasus', 1)`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	_ = old.Close()

	// Reopening must migrate.
	migrated, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer migrated.Close()

	missions, err := migrated.Missions(10)
	if err != nil {
		t.Fatalf("missions: %v", err)
	}
	if len(missions) != 1 {
		t.Fatalf("want 1 mission, got %d", len(missions))
	}
	// The default fills existing rows with 'live'.
	if missions[0].Source != SourceLive {
		t.Fatalf("legacy mission should default to %q, got %q", SourceLive, missions[0].Source)
	}
}

func missionSource(t *testing.T, d *DB, id int64) string {
	t.Helper()
	var source string
	if err := d.SQL().QueryRow(`SELECT source FROM missions WHERE id = ?`, id).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	return source
}

// --- small fixtures ---------------------------------------------------------

func sampleFor(unitID string) []model.Sample {
	return []model.Sample{{
		UnitID:    unitID,
		Name:      unitID,
		Type:      "F-16C_50",
		Category:  "plane",
		Coalition: "blue",
		Lat:       42.0,
		Lng:       41.5,
		Alt:       3000,
		Heading:   90,
		Speed:     200,
		RealTS:    1,
	}}
}

func eventFor() model.Event {
	return model.Event{
		Event:  "kill",
		Args:   []any{1.0, "F-16C_50", 2.0, 2.0, "Su-27", 1.0, "AIM-120C"},
		RealTS: 1,
	}
}

func chatFor() model.Chat {
	return model.Chat{From: "Ghost", Message: "hello", RealTS: 1}
}
