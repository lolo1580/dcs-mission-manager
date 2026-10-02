package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestMissionsEndpointEmpty checks a well-formed empty answer when no .miz was
// found. The frontend renders a single mission with seven distinct fields, so
// an empty list must still be an array (not null).
func TestMissionsEndpointEmpty(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleMissions(rec, httptest.NewRequest(http.MethodGet, "/api/missions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Missions []any `json:"missions"`
		Total    int   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Missions) != 0 || body.Total != 0 {
		t.Fatalf("expected an empty list, got %+v", body)
	}
}

// TestMissionsEndpointTheatreFilter checks ?theatre= keeps only that theatre.
func TestMissionsEndpointTheatreFilter(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetMissions([]dcsdata.MissionFile{
		{Name: "Caucasus run", Theatre: "Caucasus", Date: "1988-08-26", SizeBytes: 18570},
		{Name: "Syria run", Theatre: "Syria", Date: "2011-03-01"},
	})

	rec := httptest.NewRecorder()
	s.handleMissions(rec, httptest.NewRequest(http.MethodGet, "/api/missions?theatre=Syria", nil))
	var body struct {
		Missions []dcsdata.MissionFile `json:"missions"`
		Total    int                   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Missions) != 1 || body.Missions[0].Name != "Syria run" {
		t.Fatalf("theatre filter wrong: %+v", body.Missions)
	}
	if body.Total != 2 {
		t.Errorf("Total = %d, want 2 (the whole library)", body.Total)
	}
}
