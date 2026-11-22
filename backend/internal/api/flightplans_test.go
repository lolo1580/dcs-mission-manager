package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dcsmanager/internal/aerodrome"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/flightplan"
)

func flightPlanFixture() flightplan.Plan {
	return flightplan.Plan{
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
		Route: []flightplan.Waypoint{
			{Ident: "LFBO", ICAO: "LFBO", Name: "Toulouse", Kind: "airport", Lat: 43.6293, Lon: 1.3638, ElevationFt: 499},
			{Ident: "WP1", Kind: "user", Lat: 43.5, Lon: 3.0},
			{Ident: "LFML", ICAO: "LFML", Name: "Marseille", Kind: "airport", Lat: 43.4393, Lon: 5.2214, ElevationFt: 70},
		},
	}
}

func TestAirportSearchMergesDCSAirfields(t *testing.T) {
	catalog, err := aerodrome.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: config.Config{Theatre: "Caucasus"}, aerodromes: catalog}
	rec := httptest.NewRecorder()
	s.handleAirports(rec, httptest.NewRequest(http.MethodGet, "/api/airports?q=Batumi&theatre=Caucasus", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Airports []struct {
			Ident  string  `json:"ident"`
			Name   string  `json:"name"`
			Source string  `json:"source"`
			Tower  float64 `json:"tower"`
		} `json:"airports"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Airports) == 0 {
		t.Fatal("no airport found")
	}
	foundDCS := false
	for _, item := range body.Airports {
		if item.Source == "dcs" && strings.Contains(strings.ToUpper(item.Name), "BATUMI") {
			foundDCS = true
			if item.Tower == 0 {
				t.Fatalf("DCS tower missing: %+v", item)
			}
		}
	}
	if !foundDCS {
		t.Fatalf("DCS Batumi not merged: %+v", body.Airports)
	}

	// The worldwide dataset is searched even without DCS airfields.
	s = &Server{}
	rec = httptest.NewRecorder()
	s.handleAirports(rec, httptest.NewRequest(http.MethodGet, "/api/airports?q=LFBO", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "LFBO") {
		t.Fatalf("dataset search failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFlightPlanAPICRUDAndExports(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "plans.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := &Server{db: database, cfg: config.Config{Theatre: "Caucasus"}}

	body, _ := json.Marshal(flightPlanFixture())
	rec := httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodPost, "/api/flightplans", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var saved flightplan.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.Name == "" {
		t.Fatalf("saved = %+v", saved)
	}

	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodGet, "/api/flightplans", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Toulouse to Marseille") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodGet, "/api/flightplans/"+itoa(saved.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}

	// Calculate works on an unsaved draft as well.
	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodPost, "/api/flightplans/calculate", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("calculate: %d %s", rec.Code, rec.Body.String())
	}
	var calc struct {
		Navlog flightplan.Navlog `json:"navlog"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &calc); err != nil {
		t.Fatal(err)
	}
	if len(calc.Navlog.Legs) != 2 || calc.Navlog.TotalDistanceNM <= 0 {
		t.Fatalf("navlog = %+v", calc.Navlog)
	}

	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodGet, "/api/flightplans/"+itoa(saved.ID)+"/briefing?lang=fr", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Briefing de vol") {
		t.Fatalf("briefing: %d %s", rec.Code, rec.Body.String())
	}

	// Validation: one waypoint is not a plan.
	invalid := flightPlanFixture()
	invalid.Route = invalid.Route[:1]
	invalidBody, _ := json.Marshal(invalid)
	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodPost, "/api/flightplans", strings.NewReader(string(invalidBody))))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid plan status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodDelete, "/api/flightplans/"+itoa(saved.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodGet, "/api/flightplans/"+itoa(saved.ID), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted plan status = %d", rec.Code)
	}
}

func TestFlightPlanUnknownIDReturns404(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "unknown.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := &Server{db: database}
	body, _ := json.Marshal(flightPlanFixture())

	rec := httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodPut, "/api/flightplans/999", strings.NewReader(string(body))))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update unknown id = %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleFlightPlans(rec, httptest.NewRequest(http.MethodDelete, "/api/flightplans/999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown id = %d: %s", rec.Code, rec.Body.String())
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
