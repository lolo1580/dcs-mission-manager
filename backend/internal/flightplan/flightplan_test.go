package flightplan

import (
	"math"
	"strings"
	"testing"
)

func TestDistanceAndBearing(t *testing.T) {
	// One degree of longitude at the equator is about 60.04 NM.
	if got := DistanceNM(0, 0, 0, 1); math.Abs(got-60.04) > 0.1 {
		t.Fatalf("equatorial degree = %.2f NM", got)
	}
	if got := DistanceNM(0, 0, 0, 180); math.IsNaN(got) || math.Abs(got-10807.28) > 1 {
		t.Fatalf("half circumference = %.2f NM", got)
	}
	if got := BearingTrue(0, 0, 0, 1); math.Abs(got-90) > 0.01 {
		t.Fatalf("east bearing = %.2f", got)
	}
	if got := BearingTrue(0, 0, 1, 0); math.Abs(got) > 0.01 {
		t.Fatalf("north bearing = %.2f", got)
	}
}

func TestWindTriangle(t *testing.T) {
	heading, gs := WindTriangle(0, 100, 360, 20)
	if math.Abs(heading) > 0.01 || math.Abs(gs-80) > 0.01 {
		t.Fatalf("pure headwind: heading=%.2f gs=%.2f", heading, gs)
	}
	heading, gs = WindTriangle(0, 100, 90, 20)
	if math.Abs(heading-11.54) > 0.05 {
		t.Fatalf("crosswind heading = %.2f", heading)
	}
	if math.Abs(gs-97.98) > 0.05 {
		t.Fatalf("crosswind gs = %.2f", gs)
	}
	// A crosswind stronger than the airspeed has no solution.
	heading, gs = WindTriangle(0, 10, 90, 40)
	if heading != 0 || gs != 0 {
		t.Fatalf("impossible wind triangle: heading=%.2f gs=%.2f", heading, gs)
	}
}

func TestCalculateTotalsAndReserve(t *testing.T) {
	plan := Plan{
		CruiseTASKt: 100,
		FuelBurnPPH: 60,
		ReserveMin:  30,
		Route: []Waypoint{
			{Ident: "A", Lat: 0, Lon: 0},
			{Ident: "B", Lat: 0, Lon: 1},
			{Ident: "C", Lat: 0, Lon: 2},
		},
	}
	nav := Calculate(plan)
	if len(nav.Legs) != 2 {
		t.Fatalf("legs = %d", len(nav.Legs))
	}
	if math.Abs(nav.TotalDistanceNM-120.08) > 0.2 {
		t.Fatalf("distance = %.2f", nav.TotalDistanceNM)
	}
	if math.Abs(nav.TotalTimeMin-72.05) > 0.2 {
		t.Fatalf("time = %.2f", nav.TotalTimeMin)
	}
	if math.Abs(nav.TotalFuel-72.05) > 0.3 {
		t.Fatalf("fuel = %.2f", nav.TotalFuel)
	}
	if math.Abs(nav.ReserveFuel-30) > 0.01 {
		t.Fatalf("reserve = %.2f", nav.ReserveFuel)
	}
	if math.Abs(nav.RequiredFuel-102.05) > 0.3 {
		t.Fatalf("required = %.2f", nav.RequiredFuel)
	}
}

func TestBriefingHTML(t *testing.T) {
	plan := Plan{
		Name: "Test", Departure: "LFBO", Arrival: "LFML", CruiseAltFt: 35000,
		CruiseTASKt: 450, FuelBurnPPH: 2500, ReserveMin: 30, WindFromDeg: 270, WindKt: 30,
		Route: []Waypoint{
			{Ident: "LFBO", Kind: "airport", Lat: 43.6293, Lon: 1.3638},
			{Ident: "LFML", Kind: "airport", Lat: 43.4393, Lon: 5.2214},
		},
	}
	body, err := BriefingHTML(plan, Calculate(plan), "fr")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, needle := range []string{"Briefing de vol", "LFBO", "LFML"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("briefing missing %q", needle)
		}
	}
	if !strings.Contains(text, "caps sont vrais") {
		t.Fatal("true-heading notice missing")
	}
}
