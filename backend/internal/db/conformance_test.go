// Package db_test holds a conformance suite that exercises the db.Store
// contract against the SQLite store. The manager is local-only, so SQLite is the
// single implementation; the suite still pins the contract down so a change to
// the store cannot silently break its consumers.
package db_test

import (
	"path/filepath"
	"testing"

	"dcsmanager/internal/db"
	"dcsmanager/internal/model"
)

type namedStore struct {
	name  string
	store db.Store
}

// stores returns every configured implementation to test the contract against.
func stores(t *testing.T) []namedStore {
	t.Helper()

	sqlite, err := db.Open(filepath.Join(t.TempDir(), "conformance.db"))
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	return []namedStore{{name: "sqlite", store: sqlite}}
}

// forEachStore runs fn against every implementation under its own subtest.
func forEachStore(t *testing.T, fn func(t *testing.T, s db.Store)) {
	t.Helper()
	for _, ns := range stores(t) {
		t.Run(ns.name, func(t *testing.T) { fn(t, ns.store) })
	}
}

func TestConformanceMissionLifecycle(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		id, err := s.EnsureMission("Free Flight", "Caucasus")
		if err != nil || id == 0 {
			t.Fatalf("EnsureMission = %d, %v", id, err)
		}
		// A second call returns the same open mission.
		id2, err := s.EnsureMission("Other", "Caucasus")
		if err != nil || id2 != id {
			t.Fatalf("second EnsureMission = %d, want %d (%v)", id2, id, err)
		}
		if open := s.OpenMissionID(); open != id {
			t.Fatalf("OpenMissionID = %d, want %d", open, id)
		}
		if err := s.EndOpenMission("red"); err != nil {
			t.Fatal(err)
		}
		if open := s.OpenMissionID(); open != 0 {
			t.Fatalf("OpenMissionID after end = %d, want 0", open)
		}
		missions, err := s.Missions(10)
		if err != nil || len(missions) != 1 {
			t.Fatalf("Missions = %d, %v", len(missions), err)
		}
		if missions[0].Winner != "red" || missions[0].EndedAt == 0 {
			t.Fatalf("closed mission not updated: %+v", missions[0])
		}
	})
}

func TestConformanceEventsAndChatCursor(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		mid, err := s.EnsureMission("M", "Caucasus")
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if err := s.SaveEvent(mid, model.Event{Event: "kill", Args: []any{i, "F-16C_50"}, RealTS: int64(i)}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.SaveChat(mid, model.Chat{From: "p", Message: "hi", RealTS: 1}); err != nil {
			t.Fatal(err)
		}

		all, err := s.EventsSince(0, 100)
		if err != nil || len(all) != 3 {
			t.Fatalf("EventsSince(0) = %d, %v", len(all), err)
		}
		for i := 1; i < len(all); i++ {
			if all[i].ID <= all[i-1].ID {
				t.Fatalf("events not oldest first: %+v", all)
			}
		}
		after, err := s.EventsSince(all[0].ID, 100)
		if err != nil || len(after) != 2 {
			t.Fatalf("EventsSince(cursor) = %d, %v", len(after), err)
		}
		// args round-trip.
		if len(after[0].Args) != 2 || after[0].Args[1] != "F-16C_50" {
			t.Fatalf("args did not round-trip: %+v", after[0].Args)
		}

		chat, err := s.ChatSince(0, 100)
		if err != nil || len(chat) != 1 || chat[0].Message != "hi" {
			t.Fatalf("ChatSince = %+v, %v", chat, err)
		}
	})
}

func TestConformancePlayersAndStats(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		mid, _ := s.EnsureMission("M", "Syria")
		pid, err := s.UpsertPlayer("ucid-1", "Bob")
		if err != nil || pid == 0 {
			t.Fatalf("UpsertPlayer = %d, %v", pid, err)
		}
		// Same UCID returns the same row id.
		pid2, err := s.UpsertPlayer("ucid-1", "Bobby")
		if err != nil || pid2 != pid {
			t.Fatalf("UpsertPlayer(again) = %d, want %d (%v)", pid2, pid, err)
		}
		if err := s.SaveStats(mid, pid, model.Player{ID: 7, Name: "Bob", Side: 2, KillsAir: 3, Score: 100}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestConformanceTrackingAndHeatmap(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		mid, _ := s.EnsureMission("M", "Caucasus")
		samples := []model.Sample{
			{UnitID: "u1", Type: "F-16C_50", Category: "plane", Lat: 41.20, Lng: 41.10, Alt: 3000, Ownship: true},
			{UnitID: "u1", Type: "F-16C_50", Category: "plane", Lat: 41.24, Lng: 41.14, Alt: 3200, Ownship: true},
			{UnitID: "u2", Type: "Su-27", Category: "plane", Lat: 41.22, Lng: 41.12, Alt: 3100},
		}
		if err := s.SaveSamples(mid, samples); err != nil {
			t.Fatalf("SaveSamples: %v", err)
		}
		if err := s.SaveLoss(mid, model.Sample{UnitID: "u2", Type: "Su-27", Category: "plane", Lat: 41.22, Lng: 41.12}); err != nil {
			t.Fatalf("SaveLoss: %v", err)
		}

		// Heatmap exercises the most dialect-sensitive SQL (GROUP BY over a
		// computed grid, rounded to the grid step).
		points, err := s.Heatmap(mid, "losses", 0.05, 100)
		if err != nil {
			t.Fatalf("Heatmap: %v", err)
		}
		if len(points) != 1 || points[0].Weight != 1 {
			t.Fatalf("heatmap = %+v", points)
		}

		trails, err := s.Trails(mid, 10, 100)
		if err != nil {
			t.Fatalf("Trails: %v", err)
		}
		if len(trails["u1"]) != 2 {
			t.Fatalf("trail u1 = %d points, want 2", len(trails["u1"]))
		}

		if n, err := s.PruneTracking(0); err != nil || n == 0 {
			t.Fatalf("PruneTracking = %d, %v", n, err)
		}
	})
}

func TestConformanceMissionsSinceAndPurge(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		for i := 0; i < 3; i++ {
			name := "M" + string(rune('A'+i))
			if _, err := s.StartMission(name, "Caucasus", db.SourceLive); err != nil {
				t.Fatal(err)
			}
		}
		all, err := s.MissionsSince(0, 100)
		if err != nil || len(all) != 3 {
			t.Fatalf("MissionsSince(0) = %d, %v", len(all), err)
		}
		after, err := s.MissionsSince(all[0].ID, 100)
		if err != nil || len(after) != 2 {
			t.Fatalf("MissionsSince(cursor) = %d, %v", len(after), err)
		}

		live, err := s.CountMissions(db.SourceLive)
		if err != nil || live != 3 {
			t.Fatalf("CountMissions = %d, %v", live, err)
		}
		res, err := s.PurgeAll()
		if err != nil {
			t.Fatal(err)
		}
		if res.Missions != 3 {
			t.Fatalf("PurgeAll removed %d missions, want 3", res.Missions)
		}
		if n, _ := s.CountMissions(db.SourceLive); n != 0 {
			t.Fatalf("missions remain after purge: %d", n)
		}
	})
}

func TestConformanceDebrief(t *testing.T) {
	forEachStore(t, func(t *testing.T, s db.Store) {
		mid, _ := s.EnsureMission("M", "Caucasus")
		saved, err := s.SaveDebrief(model.Debrief{
			MissionID: mid, Mission: "M", Theatre: "Caucasus",
			Raw: "log", Size: 3,
			Parsed: model.DebriefData{Callsign: "Viper", Summary: model.DebriefSummary{Kills: 2}},
		})
		if err != nil || saved.ID == 0 {
			t.Fatalf("SaveDebrief = %+v, %v", saved, err)
		}
		list, err := s.Debriefs(10)
		if err != nil || len(list) != 1 || list[0].Parsed.Callsign != "Viper" {
			t.Fatalf("Debriefs = %+v, %v", list, err)
		}
		one, err := s.Debrief(saved.ID)
		if err != nil || one.Raw != "log" || one.Parsed.Summary.Kills != 2 {
			t.Fatalf("Debrief = %+v, %v", one, err)
		}
	})
}
