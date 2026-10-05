package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests run the plugin's SQL aggregations against a real PostgreSQL
// database shaped like the manager's. They only run when a DSN is provided, so
// `go test ./...` stays green without one:
//
//	$env:PLUGIN_TEST_DATABASE_URL = "postgres://dcs:dcs@localhost:5432/stats_test?sslmode=disable"
//	go test ./...
//
// Point it at a THROWAWAY database: the suite drops and recreates the manager's
// tables in the public schema.

// testStore prepares a database with the manager's schema and returns both the
// plugin's read-only Store and a writer pool used only to seed data.
func testStore(t *testing.T, includeTest bool) (*Store, *pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("PLUGIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLUGIN_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()

	write, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	t.Cleanup(write.Close)

	if _, err := write.Exec(ctx, seedSchema); err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	store, err := OpenStore(ctx, dsn, includeTest, 15*time.Second)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)
	return store, write, ctx
}

// seedSchema recreates the subset of the manager's schema the plugin reads.
// Every table is dropped first, including the ones only counted by the health
// footer (debriefs, track_positions), so each test starts from a clean slate.
const seedSchema = `
DROP TABLE IF EXISTS player_stats, events, chat, players, missions, debriefs, track_positions CASCADE;
CREATE TABLE missions (
	id bigint PRIMARY KEY, name text NOT NULL, theatre text,
	source text NOT NULL DEFAULT 'live', started_at bigint NOT NULL,
	ended_at bigint, winner text);
CREATE TABLE players (
	id bigint PRIMARY KEY, ucid text, name text NOT NULL,
	first_seen bigint, last_seen bigint, UNIQUE(ucid, name));
CREATE TABLE events (
	id bigint PRIMARY KEY, mission_id bigint, event text NOT NULL,
	args text, detail text, t double precision, real_ts bigint NOT NULL);
CREATE TABLE player_stats (
	id bigint PRIMARY KEY, mission_id bigint, player_id bigint,
	dcs_player_id bigint, side integer, slot text, unit_type text,
	ping integer, crashes integer, kills_car integer, kills_air integer,
	kills_ship integer, score integer, landings integer, ejects integer,
	real_ts bigint NOT NULL);
CREATE TABLE debriefs (
	id bigint PRIMARY KEY, mission_id bigint, mission text, theatre text,
	raw text, parsed text, size integer, created_at bigint);
CREATE TABLE track_positions (
	id bigint PRIMARY KEY, mission_id bigint, real_ts bigint NOT NULL);

-- The manager's read-only interface (v_stats_*). The plugin reads only these, so
-- the internal tables can change without breaking it. Kept in step with the DDL
-- in backend/internal/db/postgres/postgres.go.
CREATE OR REPLACE VIEW v_stats_missions AS
	SELECT id, name, theatre, COALESCE(source,'live') AS source, started_at, ended_at, winner FROM missions;
CREATE OR REPLACE VIEW v_stats_players AS
	SELECT id, COALESCE(ucid,'') AS ucid, name, first_seen, last_seen FROM players;
CREATE OR REPLACE VIEW v_stats_player_stats AS
	SELECT ps.id, ps.mission_id, ps.player_id, ps.dcs_player_id, ps.side, ps.slot,
	       ps.unit_type, ps.ping, ps.crashes, ps.kills_car, ps.kills_air, ps.kills_ship,
	       ps.score, ps.landings, ps.ejects, ps.real_ts, COALESCE(m.source,'live') AS source
	FROM player_stats ps LEFT JOIN missions m ON m.id = ps.mission_id;
CREATE OR REPLACE VIEW v_stats_events AS
	SELECT e.id, e.mission_id, e.event, e.args, e.detail, e.t, e.real_ts, COALESCE(m.source,'live') AS source
	FROM events e LEFT JOIN missions m ON m.id = e.mission_id;
CREATE OR REPLACE VIEW v_stats_debriefs AS
	SELECT d.id, d.mission_id, d.mission, d.theatre, d.size, d.created_at, COALESCE(m.source,'live') AS source
	FROM debriefs d LEFT JOIN missions m ON m.id = d.mission_id;
CREATE OR REPLACE VIEW v_stats_track_positions AS
	SELECT t.id, t.mission_id, t.real_ts, COALESCE(m.source,'live') AS source
	FROM track_positions t LEFT JOIN missions m ON m.id = t.mission_id;
CREATE OR REPLACE VIEW v_stats_config AS
	SELECT (SELECT COUNT(*) FROM missions) AS mission_count,
	       (SELECT COUNT(*) FROM players) AS player_count,
	       (SELECT COUNT(*) FROM events) AS event_count,
	       (SELECT COUNT(*) FROM debriefs) AS debrief_count,
	       (SELECT COUNT(*) FROM track_positions) AS position_count,
	       (SELECT COUNT(*) FROM missions WHERE source = 'test') AS test_mission_count;
`

// jsonArgs renders the []any the manager stores in events.args.
func jsonArgs(vals ...any) string {
	b, _ := json.Marshal(vals)
	return string(b)
}

// seed inserts one live mission with two pilots and a small event set, using the
// exact column layout and args order the manager produces.
func seed(t *testing.T, w *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	now := time.Now().UnixMilli()

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := w.Exec(ctx, q, args...); err != nil {
			t.Fatalf("seed exec: %v (%s)", err, q)
		}
	}

	exec(`INSERT INTO missions(id,name,theatre,source,started_at) VALUES(1,'Free Flight','Caucasus','live',$1)`, now)
	exec(`INSERT INTO missions(id,name,theatre,source,started_at) VALUES(2,'Sim Test','Caucasus','test',$1)`, now)
	exec(`INSERT INTO players(id,ucid,name,first_seen,last_seen) VALUES(1,'u-viper','Viper',$1,$1)`, now)
	exec(`INSERT INTO players(id,ucid,name,first_seen,last_seen) VALUES(2,'u-ghost','Ghost',$1,$1)`, now)

	// Viper: two snapshots in the same mission. Only the latest must count, so
	// the score is 120, not 170.
	exec(`INSERT INTO player_stats(id,mission_id,player_id,dcs_player_id,side,unit_type,ping,kills_air,kills_car,kills_ship,score,crashes,ejects,landings,real_ts)
	      VALUES(10,1,1,7,2,'F-16C_50',40,2,0,0,50,0,0,1,$1)`, now-1000)
	exec(`INSERT INTO player_stats(id,mission_id,player_id,dcs_player_id,side,unit_type,ping,kills_air,kills_car,kills_ship,score,crashes,ejects,landings,real_ts)
	      VALUES(11,1,1,7,2,'F-16C_50',44,5,1,0,120,1,0,3,$1)`, now)
	// Ghost, blue side, one snapshot.
	exec(`INSERT INTO player_stats(id,mission_id,player_id,dcs_player_id,side,unit_type,ping,kills_air,kills_car,kills_ship,score,crashes,ejects,landings,real_ts)
	      VALUES(12,1,2,9,2,'F/A-18C',55,3,0,1,80,0,1,2,$1)`, now)
	// A test-mission snapshot, to be excluded by default.
	exec(`INSERT INTO player_stats(id,mission_id,player_id,dcs_player_id,side,unit_type,ping,kills_air,kills_car,kills_ship,score,crashes,ejects,landings,real_ts)
	      VALUES(13,2,2,9,2,'F/A-18C',55,99,0,0,9999,0,0,0,$1)`, now)

	// Events. kill args: killerID, killerType, killerSide, victimID, victimType, victimSide, weapon.
	exec(`INSERT INTO events(id,mission_id,event,args,real_ts) VALUES(20,1,'kill',$1,$2)`,
		jsonArgs(7.0, "F-16C_50", 2.0, 8.0, "Su-27", 1.0, "AIM-120C"), now)
	exec(`INSERT INTO events(id,mission_id,event,args,real_ts) VALUES(21,1,'kill',$1,$2)`,
		jsonArgs(9.0, "F/A-18C", 2.0, 8.0, "Su-27", 1.0, "AIM-9X"), now)
	// pilot_death args: playerID, unit_missionID. DCS id 7 is Viper in mission 1.
	exec(`INSERT INTO events(id,mission_id,event,args,real_ts) VALUES(22,1,'pilot_death',$1,$2)`,
		jsonArgs(7.0, 1.0), now)
	// A test-mission kill, excluded by default.
	exec(`INSERT INTO events(id,mission_id,event,args,real_ts) VALUES(23,2,'kill',$1,$2)`,
		jsonArgs(9.0, "F/A-18C", 2.0, 8.0, "Su-27", 1.0, "TEST"), now)
}

func TestIntegrationOverview(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	o, err := store.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Missions != 1 {
		t.Errorf("Missions = %d, want 1 (test excluded)", o.Missions)
	}
	if o.Kills != 2 || o.Deaths != 1 {
		t.Errorf("Kills=%d Deaths=%d, want 2 and 1", o.Kills, o.Deaths)
	}
	if o.Players != 2 {
		t.Errorf("Players = %d, want 2", o.Players)
	}
}

func TestIntegrationPilotsLatestSnapshot(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	pilots, err := store.Pilots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]PilotStats{}
	for _, p := range pilots {
		byName[p.Name] = p
	}

	viper, ok := byName["Viper"]
	if !ok {
		t.Fatalf("Viper missing from %+v", pilots)
	}
	// Only the latest snapshot (score 120), not the sum (170).
	if viper.Score != 120 {
		t.Errorf("Viper score = %d, want 120 (latest snapshot only)", viper.Score)
	}
	if viper.KillsAir != 5 || viper.KillsCar != 1 || viper.Kills != 6 {
		t.Errorf("Viper kills = air %d car %d total %d, want 5/1/6", viper.KillsAir, viper.KillsCar, viper.Kills)
	}
	if viper.Crashes != 1 || viper.Landings != 3 {
		t.Errorf("Viper crashes=%d landings=%d, want 1/3", viper.Crashes, viper.Landings)
	}
	// pilot_death with DCS id 7 resolves to Viper in mission 1.
	if viper.Deaths != 1 {
		t.Errorf("Viper deaths = %d, want 1", viper.Deaths)
	}
	if viper.KD != 6.0 {
		t.Errorf("Viper K/D = %v, want 6", viper.KD)
	}

	// The test mission must not leak into Ghost's career numbers.
	ghost := byName["Ghost"]
	if ghost.Score != 80 {
		t.Errorf("Ghost score = %d, want 80 (test excluded)", ghost.Score)
	}
}

func TestIntegrationWeapons(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	weapons, err := store.Weapons(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byW := map[string]WeaponStats{}
	for _, x := range weapons {
		byW[x.Weapon] = x
	}
	if byW["AIM-120C"].Kills != 1 || byW["AIM-9X"].Kills != 1 {
		t.Fatalf("weapons = %+v", weapons)
	}
	if _, leaked := byW["TEST"]; leaked {
		t.Error("the test mission's weapon leaked into weapons")
	}
	if byW["AIM-120C"].VictimsByType["Su-27"] != 1 {
		t.Errorf("AIM-120C victims = %+v", byW["AIM-120C"].VictimsByType)
	}
	if byW["AIM-120C"].KillersByType["F-16C_50"] != 1 {
		t.Errorf("AIM-120C killers = %+v", byW["AIM-120C"].KillersByType)
	}
}

func TestIntegrationEngines(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	engines, err := store.Engines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]EngineStats{}
	for _, e := range engines {
		byType[e.TypeID] = e
	}
	// F-16C_50: kill by it (1) and it was flown by Viper (sortie 1).
	f16 := byType["F-16C_50"]
	if f16.Kills != 1 {
		t.Errorf("F-16C_50 kills = %d, want 1", f16.Kills)
	}
	if f16.Category != "plane" {
		t.Errorf("F-16C_50 category = %q, want plane", f16.Category)
	}
	// Su-27 was the victim twice (deaths).
	if byType["Su-27"].Deaths != 2 {
		t.Errorf("Su-27 deaths = %d, want 2", byType["Su-27"].Deaths)
	}
}

func TestIntegrationEventSeries(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	points, err := store.EventSeries(ctx, "kill", 30)
	if err != nil {
		t.Fatal(err)
	}
	// Both kills are today, so one bucket of 2.
	var total float64
	for _, p := range points {
		total += p.Value
	}
	if total != 2 {
		t.Fatalf("kill series total = %v, want 2 (%+v)", total, points)
	}
}

func TestIntegrationMissions(t *testing.T) {
	store, w, ctx := testStore(t, false)
	seed(t, w, ctx)

	missions, err := store.Missions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 || missions[0].Name != "Free Flight" {
		t.Fatalf("missions = %+v, want only Free Flight", missions)
	}
}

// TestIntegrationIncludeTest flips the flag and checks the test mission reappears.
func TestIntegrationIncludeTest(t *testing.T) {
	store, w, ctx := testStore(t, true)
	seed(t, w, ctx)

	o, err := store.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Missions != 2 {
		t.Errorf("Missions with includeTest = %d, want 2", o.Missions)
	}
	if o.Kills != 3 {
		t.Errorf("Kills with includeTest = %d, want 3", o.Kills)
	}
}

// TestIntegrationReadOnly proves the plugin cannot write: the session is opened
// with default_transaction_read_only=on.
func TestIntegrationReadOnly(t *testing.T) {
	store, _, ctx := testStore(t, false)
	if _, err := store.pool.Exec(ctx, `INSERT INTO missions(id,name,started_at) VALUES(999,'x',1)`); err == nil {
		t.Fatal("the plugin's session must reject writes")
	} else if !isReadOnlyError(err) {
		t.Fatalf("expected a read-only error, got %v", err)
	}
}

func isReadOnlyError(err error) bool {
	s := fmt.Sprint(err)
	return contains(s, "read-only") || contains(s, "read only") || contains(s, "25006")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
