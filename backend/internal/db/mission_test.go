package db

import (
	"path/filepath"
	"sync"
	"testing"
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
