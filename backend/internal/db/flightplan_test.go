package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"dcsmanager/internal/flightplan"
)

func TestFlightPlanUIDMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-plans.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`
		CREATE TABLE flight_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			aircraft TEXT,
			departure TEXT,
			arrival TEXT,
			cruise_alt_ft INTEGER NOT NULL DEFAULT 0,
			cruise_tas_kt INTEGER NOT NULL DEFAULT 0,
			fuel_burn_pph REAL NOT NULL DEFAULT 0,
			reserve_min INTEGER NOT NULL DEFAULT 0,
			wind_from_deg INTEGER NOT NULL DEFAULT 0,
			wind_kt INTEGER NOT NULL DEFAULT 0,
			flight_rules TEXT NOT NULL DEFAULT 'VFR',
			notes TEXT,
			route TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		INSERT INTO flight_plans(name, route, created_at, updated_at)
		VALUES('Legacy', '[{"ident":"LFBO","kind":"airport","lat":43.6,"lon":1.36},{"ident":"LFML","kind":"airport","lat":43.4,"lon":5.2}]', 1, 1);
	`)
	if err != nil {
		raw.Close()
		t.Fatal(err)
	}
	raw.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	plan, err := d.FlightPlan(1)
	if err != nil {
		t.Fatal(err)
	}
	if plan.UID == "" {
		t.Fatal("legacy plan did not receive a uid")
	}
	if plan.Revision != 1 {
		t.Fatalf("revision = %d, want 1", plan.Revision)
	}

	// The unique index must accept the migrated row and a new plan.
	saved, err := d.SaveFlightPlan(flightplan.Plan{
		Name:  "New",
		Route: plan.Route,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.UID == plan.UID || saved.Revision != 1 {
		t.Fatalf("new plan = %+v", saved)
	}
}

func TestFlightPlanCRUD(t *testing.T) {
	d := openTemp(t)
	plan := flightplan.Plan{
		Name:        "Toulouse to Marseille",
		Aircraft:    "A320neo",
		Departure:   "LFBO",
		Arrival:     "LFML",
		CruiseAltFt: 35000,
		CruiseTASKt: 450,
		FuelBurnPPH: 2500,
		ReserveMin:  30,
		WindFromDeg: 270,
		WindKt:      25,
		FlightRules: "IFR",
		Notes:       "Test plan",
		Route: []flightplan.Waypoint{
			{Ident: "LFBO", ICAO: "LFBO", Name: "Toulouse", Kind: "airport", Lat: 43.6293, Lon: 1.3638, ElevationFt: 499},
			{Ident: "WP1", Kind: "user", Lat: 43.5, Lon: 3.0},
			{Ident: "LFML", ICAO: "LFML", Name: "Marseille", Kind: "airport", Lat: 43.4393, Lon: 5.2214, ElevationFt: 70},
		},
	}
	saved, err := d.SaveFlightPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("not stamped: %+v", saved)
	}

	loaded, err := d.FlightPlan(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != plan.Name || loaded.FuelBurnPPH != plan.FuelBurnPPH || len(loaded.Route) != 3 {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded.Route[1].Lat != 43.5 || loaded.Route[2].ICAO != "LFML" {
		t.Fatalf("route not round-tripped: %+v", loaded.Route)
	}

	list, err := d.FlightPlans(10)
	if err != nil || len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list = %+v, %v", list, err)
	}

	saved.Name = "Renamed"
	saved.Route = saved.Route[:2]
	updated, err := d.SaveFlightPlan(saved)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != saved.ID || updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("update changed identity: %+v", updated)
	}
	loaded, _ = d.FlightPlan(saved.ID)
	if loaded.Name != "Renamed" || len(loaded.Route) != 2 {
		t.Fatalf("update not stored: %+v", loaded)
	}

	if err := d.DeleteFlightPlan(saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FlightPlan(saved.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted plan still readable: %v", err)
	}
}

func TestFlightPlanEmptyRoute(t *testing.T) {
	d := openTemp(t)
	saved, err := d.SaveFlightPlan(flightplan.Plan{Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := d.FlightPlan(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Route == nil || len(loaded.Route) != 0 {
		t.Fatalf("empty route = %#v", loaded.Route)
	}
}

func TestFlightPlanUnknownIDIsNoRows(t *testing.T) {
	d := openTemp(t)
	if _, err := d.SaveFlightPlan(flightplan.Plan{ID: 999, Name: "Ghost"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("update unknown id = %v, want sql.ErrNoRows", err)
	}
	if err := d.DeleteFlightPlan(999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("delete unknown id = %v, want sql.ErrNoRows", err)
	}
}
