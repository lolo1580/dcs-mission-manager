package stats

import (
	"path/filepath"
	"testing"

	"dcsmm/internal/category"
	"dcsmm/internal/db"
	"dcsmm/internal/model"
)

func setup(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database, category.New("")), database
}

// seed populates a mission with two players and a few events.
func seed(t *testing.T, d *db.DB) int64 {
	t.Helper()
	missionID, err := d.EnsureMission("Test Stats", "Caucasus")
	if err != nil {
		t.Fatalf("mission: %v", err)
	}

	players := []model.Player{
		{ID: 1, UCID: "u-viper", Name: "Viper", Side: 2, Slot: "F-16C_50", UnitType: "F-16C_50", Ping: 40,
			Score: 300, KillsAir: 3, KillsCar: 0, KillsShip: 0, Landings: 2},
		{ID: 2, UCID: "u-flanker", Name: "Flanker", Side: 1, Slot: "Su-27", UnitType: "Su-27", Ping: 90,
			Score: 150, KillsAir: 1, KillsCar: 1, KillsShip: 0, Ejects: 1},
	}
	for _, p := range players {
		pid, err := d.UpsertPlayer(p.UCID, p.Name)
		if err != nil {
			t.Fatalf("player: %v", err)
		}
		if err := d.SaveStats(missionID, pid, p); err != nil {
			t.Fatalf("stats: %v", err)
		}
	}

	events := []model.Event{
		// Viper shoots down Flanker with an AIM-120C.
		{Event: "kill", Args: []any{1.0, "F-16C_50", 2.0, 2.0, "Su-27", 1.0, "AIM-120C"}, RealTS: 1},
		// Flanker destroys a ground target with an R-73.
		{Event: "kill", Args: []any{2.0, "Su-27", 1.0, 0.0, "T-72B", 1.0, "R-73"}, RealTS: 2},
		// Viper friendly-fires with a gun.
		{Event: "friendly_fire", Args: []any{1.0, "M61A1", 2.0}, RealTS: 3},
		{Event: "pilot_death", Args: []any{2.0, 100.0}, RealTS: 4},
		{Event: "crash", Args: []any{1.0, 101.0}, RealTS: 5},
	}
	for _, e := range events {
		if err := d.SaveEvent(missionID, e); err != nil {
			t.Fatalf("event: %v", err)
		}
	}
	return missionID
}

func TestPilotsCareer(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	pilots, err := svc.Pilots(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("pilots: %v", err)
	}
	if len(pilots) != 2 {
		t.Fatalf("want 2 pilots, got %d", len(pilots))
	}
	// Viper has the higher score and should be first.
	if pilots[0].Name != "Viper" {
		t.Fatalf("want Viper first, got %q", pilots[0].Name)
	}
	if pilots[0].Kills != 3 || pilots[0].Score != 300 {
		t.Fatalf("unexpected Viper stats: %+v", pilots[0])
	}
	if pilots[0].FriendlyFF != 1 {
		t.Fatalf("Viper friendly-fire should be 1, got %d", pilots[0].FriendlyFF)
	}

	// Flanker died once (pilot_death event resolved via DCS player id).
	var flanker *PilotStats
	for i := range pilots {
		if pilots[i].Name == "Flanker" {
			flanker = &pilots[i]
		}
	}
	if flanker == nil {
		t.Fatal("Flanker not found")
	}
	if flanker.Deaths != 1 {
		t.Fatalf("Flanker deaths should be 1, got %d", flanker.Deaths)
	}
	if flanker.KD != 2 {
		t.Fatalf("Flanker KD should be 2/1=2, got %v", flanker.KD)
	}
}

func TestPilotsMissionScope(t *testing.T) {
	svc, database := setup(t)
	missionID := seed(t, database)

	scoped, err := svc.Pilots(Scope{Mode: "mission", MissionID: missionID})
	if err != nil {
		t.Fatalf("pilots: %v", err)
	}
	if len(scoped) != 2 {
		t.Fatalf("want 2 pilots in mission scope, got %d", len(scoped))
	}

	// A different mission must yield nothing.
	other, err := svc.Pilots(Scope{Mode: "mission", MissionID: 9999})
	if err != nil {
		t.Fatalf("pilots: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("want 0 pilots for an unknown mission, got %d", len(other))
	}
}

func TestWeapons(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	weapons, err := svc.Weapons(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("weapons: %v", err)
	}
	byName := map[string]WeaponStats{}
	for _, w := range weapons {
		byName[w.Weapon] = w
	}

	if byName["AIM-120C"].Kills != 1 {
		t.Fatalf("AIM-120C kills = %d", byName["AIM-120C"].Kills)
	}
	if byName["AIM-120C"].VictimsByType["Su-27"] != 1 {
		t.Fatalf("AIM-120C victims = %v", byName["AIM-120C"].VictimsByType)
	}
	if byName["M61A1"].FriendlyFire != 1 {
		t.Fatalf("M61A1 friendly fire = %d", byName["M61A1"].FriendlyFire)
	}
}

func TestEngines(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	engines, err := svc.Engines(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("engines: %v", err)
	}
	byType := map[string]EngineStats{}
	for _, e := range engines {
		byType[e.TypeID] = e
	}

	if byType["F-16C_50"].Kills != 1 || byType["F-16C_50"].Category != category.Plane {
		t.Fatalf("unexpected F-16C_50: %+v", byType["F-16C_50"])
	}
	if byType["Su-27"].Deaths != 1 {
		t.Fatalf("Su-27 deaths = %d", byType["Su-27"].Deaths)
	}
	if byType["T-72B"].Deaths != 1 || byType["T-72B"].Category != category.Ground {
		t.Fatalf("unexpected T-72B: %+v", byType["T-72B"])
	}
	if byType["F-16C_50"].Sorties != 1 {
		t.Fatalf("F-16C_50 sorties = %d", byType["F-16C_50"].Sorties)
	}
}

func TestCoalitions(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	coal, err := svc.Coalitions(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("coalitions: %v", err)
	}
	if len(coal) != 2 {
		t.Fatalf("want red and blue, got %d", len(coal))
	}
	if coal[0].Coalition != "blue" || coal[0].Score != 300 {
		t.Fatalf("unexpected first coalition: %+v", coal[0])
	}
}

func TestNetwork(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	net, err := svc.Network(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("network: %v", err)
	}
	if len(net) != 2 {
		t.Fatalf("want 2 entries, got %d", len(net))
	}
	// Worst ping first: Flanker at 90 ms.
	if net[0].Name != "Flanker" || net[0].AvgPing != 90 {
		t.Fatalf("unexpected first network entry: %+v", net[0])
	}
}

func TestOverview(t *testing.T) {
	svc, database := setup(t)
	seed(t, database)

	o, err := svc.Overview(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if o.Kills != 2 || o.Deaths != 1 || o.Crashes != 1 || o.FriendlyFF != 1 {
		t.Fatalf("unexpected overview: %+v", o)
	}
	if o.Missions != 1 {
		t.Fatalf("missions = %d", o.Missions)
	}
	if len(o.Coalitions) != 2 {
		t.Fatalf("coalitions = %d", len(o.Coalitions))
	}
}

func TestEnginesUseResolvedUnitTypes(t *testing.T) {
	// Unit types are resolved on the Lua side; engine stats must use them rather
	// than slot ids.
	svc, database := setup(t)
	seed(t, database)

	engines, err := svc.Engines(Scope{Mode: "career"})
	if err != nil {
		t.Fatalf("engines: %v", err)
	}
	byType := map[string]EngineStats{}
	for _, e := range engines {
		byType[e.TypeID] = e
	}
	if byType["Su-27"].Sorties != 1 {
		t.Fatalf("Su-27 sorties = %d, want 1", byType["Su-27"].Sorties)
	}
	if byType["F-16C_50"].Sorties != 1 {
		t.Fatalf("F-16C_50 sorties = %d, want 1", byType["F-16C_50"].Sorties)
	}
}
