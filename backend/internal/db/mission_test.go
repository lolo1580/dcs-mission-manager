package db

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dcsmanager/internal/model"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// TestConcurrentEnsureMissionCreatesOne locks the fix for the race that left
// several missions open at once: the get-or-create is serialised and backed by a
// unique index, so many concurrent callers end up with a single mission.
func TestConcurrentEnsureMissionCreatesOne(t *testing.T) {
	d := openTestDB(t)

	const n = 32
	ids := make([]int64, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			id, err := d.EnsureMissionTagged("Concurrent", "Caucasus", SourceLive)
			if err != nil {
				t.Errorf("ensure: %v", err)
				return
			}
			ids[i] = id
		}(i)
	}
	close(start)
	wg.Wait()

	var open int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM missions WHERE ended_at IS NULL`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 1 {
		t.Fatalf("concurrent ensure left %d open missions, want 1", open)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("callers got different missions: %v", ids)
		}
	}
}

// TestStartMissionClosesStaleOpenMission locks the transition rule: a new mission
// announced while a previous one is still open closes the stale one instead of
// merging the new flight into it.
func TestStartMissionClosesStaleOpenMission(t *testing.T) {
	d := openTestDB(t)

	first, err := d.StartMission("Previous", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.StartMission("New flight", "Syria", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("a different mission start must open a new mission")
	}
	if open := d.OpenMissionID(); open != second {
		t.Fatalf("open mission = %d, want the new one %d", open, second)
	}
	// The first is closed.
	var ended *int64
	if err := d.sql.QueryRow(`SELECT ended_at FROM missions WHERE id = ?`, first).Scan(&ended); err != nil {
		t.Fatal(err)
	}
	if ended == nil {
		t.Fatal("the previous mission should have been closed")
	}
}

// TestStartMissionRepeatedIsIdempotent checks a repeated start of the same flight
// (a reconnect) does not split it into two missions.
func TestStartMissionRepeatedIsIdempotent(t *testing.T) {
	d := openTestDB(t)

	first, err := d.StartMission("Same", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	again, err := d.StartMission("Same", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatalf("a repeated start should keep the same mission: %d != %d", again, first)
	}
	var count int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM missions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("want a single mission, got %d", count)
	}
}

// TestUpsertPlayerKeepsNewUCID locks the identity fix: a name-only row matched by
// name adopts the UCID once it is known, instead of staying anonymous and later
// being duplicated after a rename.
func TestUpsertPlayerKeepsNewUCID(t *testing.T) {
	d := openTestDB(t)

	// Recorded first without a UCID...
	id1, err := d.UpsertPlayer("", "Pilot")
	if err != nil {
		t.Fatal(err)
	}
	// ...then seen again with one.
	id2, err := d.UpsertPlayer("new-ucid", "Pilot")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("the same name should resolve to one player: %d vs %d", id1, id2)
	}
	var ucid string
	if err := d.sql.QueryRow(`SELECT COALESCE(ucid,'') FROM players WHERE id = ?`, id1).Scan(&ucid); err != nil {
		t.Fatal(err)
	}
	if ucid != "new-ucid" {
		t.Fatalf("the UCID should have been recorded, got %q", ucid)
	}
}

// TestStartMissionClosesEmptyMissionAtStart locks the stale-session fix: an
// interrupted mission with no data must not last until the next start.
func TestStartMissionClosesEmptyMissionAtStart(t *testing.T) {
	d := openTestDB(t)
	first, err := d.StartMission("Empty", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-48 * time.Hour).UnixMilli()
	if _, err := d.sql.Exec(`UPDATE missions SET started_at = ?, last_activity_at = 0 WHERE id = ?`, start, first); err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartMission("Next", "Caucasus", SourceLive); err != nil {
		t.Fatal(err)
	}
	var ended int64
	var abandoned bool
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, first).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended != start {
		t.Fatalf("empty mission ended at %d, want its start %d", ended, start)
	}
	if !abandoned {
		t.Fatal("an empty interrupted mission must be marked abandoned")
	}
}

// TestStartMissionClosesAtLastActivity checks that a stale mission with real
// data keeps the time it was actually active, not the next mission's start.
func TestStartMissionClosesAtLastActivity(t *testing.T) {
	d := openTestDB(t)
	first, err := d.StartMission("Flown", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-48 * time.Hour).UnixMilli()
	activity := start + 25*60*1000
	if _, err := d.sql.Exec(`UPDATE missions SET started_at = ?, last_activity_at = 0 WHERE id = ?`, start, first); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveEvent(first, model.Event{Event: "takeoff", RealTS: activity}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartMission("Next", "Caucasus", SourceLive); err != nil {
		t.Fatal(err)
	}
	var ended int64
	var abandoned bool
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, first).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended != activity {
		t.Fatalf("mission ended at %d, want last activity %d", ended, activity)
	}
	if !abandoned {
		t.Fatal("a mission closed after a long silence must be marked abandoned")
	}
}

// TestStartMissionSplitsStaleSameName covers a manager that stayed online: a
// new flight with the same name days later must not be merged into the stale
// row just because the name and theatre match.
func TestStartMissionSplitsStaleSameName(t *testing.T) {
	d := openTestDB(t)
	first, err := d.StartMission("Same", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	if _, err := d.sql.Exec(`UPDATE missions SET started_at = ?, last_activity_at = 0 WHERE id = ?`, old, first); err != nil {
		t.Fatal(err)
	}
	second, err := d.StartMission("Same", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("a stale mission with the same name must still be split")
	}
	var ended int64
	var abandoned bool
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, first).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended != old || !abandoned {
		t.Fatalf("stale mission = (%d, %v), want (%d, true)", ended, abandoned, old)
	}
}

// TestEndOpenMissionRecentKeepsNow checks an ordinary mission end still uses the
// end time instead of clamping every mission to its last snapshot.
func TestEndOpenMissionRecentKeepsNow(t *testing.T) {
	d := openTestDB(t)
	id, err := d.StartMission("Live", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	if err := d.SaveEvent(id, model.Event{Event: "landing", RealTS: now}); err != nil {
		t.Fatal(err)
	}
	if err := d.EndOpenMission("blue"); err != nil {
		t.Fatal(err)
	}
	var ended int64
	var abandoned bool
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, id).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended < now {
		t.Fatalf("ended at %d, want at least %d", ended, now)
	}
	if abandoned {
		t.Fatal("a mission ended while active must not be abandoned")
	}
}

// TestRepairBackfillsLastActivity checks the startup repair recovers the newest
// child timestamp for missions written before last_activity_at existed.
func TestRepairBackfillsLastActivity(t *testing.T) {
	d := openTestDB(t)
	id, err := d.StartMission("Backfill", "Caucasus", SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveEvent(id, model.Event{Event: "takeoff", RealTS: 12345}); err != nil {
		t.Fatal(err)
	}
	// Simulate a row written by an older version.
	if _, err := d.sql.Exec(`UPDATE missions SET last_activity_at = 0 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if err := d.repairMissionDurations(); err != nil {
		t.Fatal(err)
	}
	var last int64
	if err := d.sql.QueryRow(`SELECT last_activity_at FROM missions WHERE id = ?`, id).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if last != 12345 {
		t.Fatalf("last_activity_at = %d, want the event timestamp 12345", last)
	}
}

// TestRepairMissionDurations fixes databases written before the stale-close
// rule and leaves legitimate long missions alone.
func TestRepairMissionDurations(t *testing.T) {
	d := openTestDB(t)
	old := time.Now().Add(-72 * time.Hour).UnixMilli()
	res, err := d.sql.Exec(
		`INSERT INTO missions(name, theatre, source, started_at, ended_at, last_activity_at)
		 VALUES('Stale', 'Caucasus', 'live', ?, ?, 0)`,
		old, old+48*3600*1000,
	)
	if err != nil {
		t.Fatal(err)
	}
	staleID, _ := res.LastInsertId()

	// A real two-hour flight with activity five minutes before its end.
	res, err = d.sql.Exec(
		`INSERT INTO missions(name, theatre, source, started_at, ended_at, last_activity_at)
		 VALUES('Real', 'Caucasus', 'live', ?, ?, ?)`,
		old, old+2*3600*1000, old+2*3600*1000-5*60*1000,
	)
	if err != nil {
		t.Fatal(err)
	}
	realID, _ := res.LastInsertId()

	if err := d.repairMissionDurations(); err != nil {
		t.Fatal(err)
	}

	var ended int64
	var abandoned bool
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, staleID).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended != old || !abandoned {
		t.Fatalf("stale mission = (%d, %v), want (%d, true)", ended, abandoned, old)
	}
	if err := d.sql.QueryRow(`SELECT ended_at, abandoned FROM missions WHERE id = ?`, realID).Scan(&ended, &abandoned); err != nil {
		t.Fatal(err)
	}
	if ended != old+2*3600*1000 || abandoned {
		t.Fatalf("real mission = (%d, %v), want its original end and false", ended, abandoned)
	}
}
