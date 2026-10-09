package stats

import (
	"testing"
	"time"

	"dcsmanager/internal/db"
)

func TestCareerInsightsMapsCountriesAndThirtyDayActivity(t *testing.T) {
	svc, database := setup(t)
	pilot, err := database.UpsertPlayer("pilot-ucid", "Viper")
	if err != nil {
		t.Fatal(err)
	}
	other, err := database.UpsertPlayer("other-ucid", "Wingman")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	yesterday := today.AddDate(0, 0, -1).Add(12 * time.Hour).UnixMilli()
	old := today.AddDate(0, 0, -35).Add(12 * time.Hour).UnixMilli()

	addMission := func(name, theatre, source, country string, at int64) int64 {
		t.Helper()
		mission, err := database.StartMission(name, theatre, source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL().Exec(`UPDATE missions SET started_at=? WHERE id=?`, at, mission); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 7; i++ {
			if _, err := database.SQL().Exec(`INSERT INTO player_stats(mission_id,player_id,unit_type,country,real_ts) VALUES(?,?,?,?,?)`,
				mission, pilot, "F-16C_50", country, at+int64(i*10_000)); err != nil {
				t.Fatal(err)
			}
		}
		return mission
	}
	first := addMission("First", "Caucasus", db.SourceLive, "USA", yesterday)
	// A disconnected half hour is not counted as cockpit time.
	if _, err := database.SQL().Exec(`INSERT INTO player_stats(mission_id,player_id,unit_type,country,real_ts) VALUES(?,?,?,?,?)`,
		first, pilot, "F-16C_50", "USA", yesterday+30*60_000); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL().Exec(`INSERT INTO player_stats(mission_id,player_id,unit_type,country,real_ts) VALUES(?,?,?,?,?)`,
		first, other, "F-16C_50", "Germany", yesterday+10_000); err != nil {
		t.Fatal(err)
	}
	addMission("Second", "Syria", db.SourceLive, "France", yesterday+60*60_000)
	addMission("Old", "Nevada", db.SourceLive, "USA", old)
	addMission("Fixture", "Syria", db.SourceTest, "Russia", yesterday+2*60*60_000)

	got, err := svc.CareerInsightsForPilot(Scope{Mode: "career"}, "pilot-ucid", "Viper", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Maps) != 3 || len(got.Countries) != 2 {
		t.Fatalf("maps=%+v countries=%+v", got.Maps, got.Countries)
	}
	if got.Countries[0].Name != "USA" || got.Countries[0].Missions != 2 {
		t.Fatalf("country ranking: %+v", got.Countries)
	}
	if len(got.Days) != 30 || got.Days[28].Minutes != 2 || got.Days[29].Minutes != 0 {
		t.Fatalf("daily activity: %+v", got.Days)
	}

	filtered, err := svc.CareerInsightsForPilot(Scope{Mode: "career", FromMs: today.AddDate(0, 0, -2).UnixMilli()}, "pilot-ucid", "Viper", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Maps) != 2 || len(filtered.Countries) != 2 || filtered.Days[28].Minutes != 2 {
		t.Fatalf("period breakdown vs fixed activity: %+v", filtered)
	}
}
