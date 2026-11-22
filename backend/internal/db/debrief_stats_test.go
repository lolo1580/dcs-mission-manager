package db

import (
	"path/filepath"
	"testing"

	"dcsmanager/internal/model"
)

// TestSaveDebriefImportsPilotStats covers a solo flight where the live hook sent
// no player snapshots: the debrief itself must populate the statistics tables.
func TestSaveDebriefImportsPilotStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	missionID, err := d.EnsureMission("tempMission", "Caucasus")
	if err != nil {
		t.Fatal(err)
	}
	rec := model.Debrief{
		MissionID: missionID,
		Mission:   "tempMission",
		Parsed: model.DebriefData{
			Callsign: "Cellar",
			Summary: model.DebriefSummary{
				Takeoffs: 1,
				Landings: 1,
				Pilots:   []string{"Cellar"},
			},
			Events: []model.DebriefEvent{
				{Type: "takeoff", InitiatorPilot: "Cellar", InitiatorUnitType: "FA-18C_hornet", InitiatorCoalition: 2},
				{Type: "land", InitiatorPilot: "Cellar", InitiatorUnitType: "FA-18C_hornet", InitiatorCoalition: 2},
			},
		},
	}
	if _, err := d.SaveDebrief(rec); err != nil {
		t.Fatal(err)
	}

	var players, stats, landings int
	if err := d.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&players); err != nil {
		t.Fatal(err)
	}
	if err := d.QueryRow(`SELECT COUNT(*) FROM player_stats`).Scan(&stats); err != nil {
		t.Fatal(err)
	}
	if err := d.QueryRow(`SELECT landings FROM player_stats`).Scan(&landings); err != nil {
		t.Fatal(err)
	}
	if players != 1 || stats != 1 || landings != 1 {
		t.Fatalf("players=%d stats=%d landings=%d", players, stats, landings)
	}

	// Backfilling the same debrief, or reopening the database, must not duplicate
	// the snapshot.
	if err := d.BackfillDebriefStats(); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.QueryRow(`SELECT COUNT(*) FROM player_stats`).Scan(&stats); err != nil {
		t.Fatal(err)
	}
	if stats != 1 {
		t.Fatalf("reopening duplicated stats: %d", stats)
	}
}
