package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"dcsmanager/internal/model"
)

func TestMigrateOldPlayerStatsAddsCountry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.Exec(`CREATE TABLE player_stats (
		id INTEGER PRIMARY KEY, mission_id INTEGER, player_id INTEGER,
		dcs_player_id INTEGER, side INTEGER, slot TEXT, unit_type TEXT,
		ping INTEGER, crashes INTEGER, kills_car INTEGER, kills_air INTEGER,
		kills_ship INTEGER, score INTEGER, landings INTEGER, ejects INTEGER,
		real_ts INTEGER NOT NULL) `)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(path)
	if err != nil {
		t.Fatalf("opening previous database: %v", err)
	}
	defer upgraded.Close()
	mission, _ := upgraded.EnsureMission("Flight", "Caucasus")
	pilot, _ := upgraded.UpsertPlayer("ucid", "Pilot")
	if err := upgraded.SaveStats(mission, pilot, model.Player{ID: 1, Name: "Pilot", UnitType: "F-16C_50", Country: "USA"}); err != nil {
		t.Fatal(err)
	}
	var country string
	if err := upgraded.QueryRow(`SELECT country FROM player_stats LIMIT 1`).Scan(&country); err != nil || country != "USA" {
		t.Fatalf("country after migration = %q, %v", country, err)
	}
}

func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestMissionLifecycle(t *testing.T) {
	d := openTemp(t)

	id, err := d.EnsureMission("Free Caucasus", "Caucasus")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a mission id")
	}

	// A second call must return the same open mission.
	id2, err := d.EnsureMission("Other", "Caucasus")
	if err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if id2 != id {
		t.Fatalf("want same open mission %d, got %d", id, id2)
	}

	if err := d.EndOpenMission("red"); err != nil {
		t.Fatalf("end: %v", err)
	}
	if got := d.OpenMissionID(); got != 0 {
		t.Fatalf("no open mission expected, got %d", got)
	}

	id3, err := d.EnsureMission("Next", "Caucasus")
	if err != nil {
		t.Fatalf("ensure 3: %v", err)
	}
	if id3 == id {
		t.Fatal("a new mission should have been created")
	}
}

func TestSaveAndReadEvents(t *testing.T) {
	d := openTemp(t)
	mid, _ := d.EnsureMission("M", "Caucasus")

	if err := d.SaveEvent(mid, model.Event{Event: "kill", Args: []any{1.0, "T-72B"}, T: 12.5, RealTS: 1000}); err != nil {
		t.Fatalf("save: %v", err)
	}
	events, err := d.RecentEvents(10)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Event != "kill" || len(events[0].Args) != 2 || events[0].T != 12.5 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestUpsertPlayerByUCID(t *testing.T) {
	d := openTemp(t)

	id1, err := d.UpsertPlayer("ucid-123", "Bob")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Same UCID, new name: same row, name updated (career survives renames).
	id2, err := d.UpsertPlayer("ucid-123", "Bobby")
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("want same player row %d, got %d", id1, id2)
	}
}

func TestUpsertPlayerWithoutUCID(t *testing.T) {
	d := openTemp(t)

	id1, _ := d.UpsertPlayer("", "Anon")
	id2, _ := d.UpsertPlayer("", "Anon")
	if id1 != id2 {
		t.Fatalf("name-only players should match by name: %d vs %d", id1, id2)
	}
	id3, _ := d.UpsertPlayer("", "Someone else")
	if id3 == id1 {
		t.Fatal("different names should create different rows")
	}
}

func TestStatsAndMissions(t *testing.T) {
	d := openTemp(t)
	mid, _ := d.EnsureMission("M", "Syria")
	pid, _ := d.UpsertPlayer("u1", "Pilot")

	if err := d.SaveStats(mid, pid, model.Player{
		ID: 1, Name: "Pilot", Side: 2, Slot: "F-16C", Ping: 42,
		KillsAir: 3, Score: 150, Landings: 1,
	}); err != nil {
		t.Fatalf("save stats: %v", err)
	}

	missions, err := d.Missions(10)
	if err != nil {
		t.Fatalf("missions: %v", err)
	}
	if len(missions) != 1 || missions[0].Name != "M" || missions[0].Theatre != "Syria" {
		t.Fatalf("unexpected missions: %+v", missions)
	}
}

func TestSaveChat(t *testing.T) {
	d := openTemp(t)
	if err := d.SaveChat(0, model.Chat{From: "Bob", Message: "hello", RealTS: 5}); err != nil {
		t.Fatalf("save chat: %v", err)
	}
	chat, err := d.RecentChat(10)
	if err != nil {
		t.Fatalf("read chat: %v", err)
	}
	if len(chat) != 1 || chat[0].Message != "hello" {
		t.Fatalf("unexpected chat: %+v", chat)
	}
}
