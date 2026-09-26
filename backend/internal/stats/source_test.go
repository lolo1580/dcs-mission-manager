package stats

import (
	"testing"

	"dcsmm/internal/db"
	"dcsmm/internal/model"
)

// seedMission records a mission with one player, a kill and a sample, tagged
// with the given source.
func seedMission(t *testing.T, d *db.DB, name, source, unitID string) int64 {
	t.Helper()

	id, err := d.EnsureMissionTagged(name, "Caucasus", source)
	if err != nil {
		t.Fatalf("mission %s: %v", name, err)
	}
	pid, err := d.UpsertPlayer("ucid-"+unitID, "Pilot-"+unitID)
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	if err := d.SaveStats(id, pid, model.Player{
		ID: 1, UCID: "ucid-" + unitID, Name: "Pilot-" + unitID,
		Side: 2, Slot: "F-16C_50", UnitType: "F-16C_50", Score: 100, KillsAir: 1,
	}); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if err := d.SaveEvent(id, model.Event{
		Event:  "kill",
		Args:   []any{1.0, "F-16C_50", 2.0, 2.0, "Su-27", 1.0, "AIM-120C"},
		RealTS: 1,
	}); err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := d.SaveSamples(id, []model.Sample{{
		UnitID: unitID, Type: "F-16C_50", Category: "plane", Coalition: "blue",
		Lat: 42, Lng: 41, Alt: 3000, RealTS: 1,
	}}); err != nil {
		t.Fatalf("samples: %v", err)
	}
	if err := d.EndOpenMission("blue"); err != nil {
		t.Fatalf("end: %v", err)
	}
	return id
}

// TestTestMissionsExcludedFromStats is the guarantee that matters: a session
// recorded from the test tools must not appear in statistics, while a real one
// is counted normally.
func TestTestMissionsExcludedFromStats(t *testing.T) {
	svc, database := setup(t)
	seedMission(t, database, "Real mission", db.SourceLive, "real-unit")
	seedMission(t, database, "Simulated mission", db.SourceTest, "test-unit")

	// Default: test sessions are invisible.
	over, err := svc.Overview(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if over.Missions != 1 {
		t.Fatalf("career should count only the real mission, got %d", over.Missions)
	}
	if over.Kills != 1 {
		t.Fatalf("career should count only the real kill, got %d", over.Kills)
	}

	pilots, err := svc.Pilots(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("pilots: %v", err)
	}
	if len(pilots) != 1 {
		t.Fatalf("career should list only the real pilot, got %d", len(pilots))
	}
	if pilots[0].Name != "Pilot-real-unit" {
		t.Fatalf("unexpected pilot in career: %q", pilots[0].Name)
	}

	// The test session is reachable explicitly, so it is excluded, not lost.
	withTest, err := svc.Overview(Scope{Mode: "career", IncludeTest: true})
	if err != nil {
		t.Fatalf("overview+test: %v", err)
	}
	if withTest.Missions != 2 || withTest.Kills != 2 {
		t.Fatalf("includeTest should count both sessions, got missions=%d kills=%d",
			withTest.Missions, withTest.Kills)
	}
}

// TestMissionScopeCannotLeakTestData checks that asking for one mission by id
// still applies the test filter. Without this, a stale UI link could surface a
// simulated mission in a statistics view.
func TestMissionScopeCannotLeakTestData(t *testing.T) {
	svc, database := setup(t)
	testID := seedMission(t, database, "Simulated mission", db.SourceTest, "test-unit")

	over, err := svc.Overview(Scope{Mode: "mission", MissionID: testID})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if over.Kills != 0 {
		t.Fatalf("a test mission must not yield kills in mission scope, got %d", over.Kills)
	}

	// With the explicit opt-in it is visible again.
	over2, err := svc.Overview(Scope{Mode: "mission", MissionID: testID, IncludeTest: true})
	if err != nil {
		t.Fatalf("overview+test: %v", err)
	}
	if over2.Kills != 1 {
		t.Fatalf("includeTest should reveal the test mission, got %d kills", over2.Kills)
	}
}
