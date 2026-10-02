package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestCareerEndpointEmpty checks a well-formed empty answer when no logbook was
// read, so the UI shows "nothing found" rather than breaking.
func TestCareerEndpointEmpty(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleCareer(rec, httptest.NewRequest(http.MethodGet, "/api/career", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Players []any `json:"players"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Players) != 0 {
		t.Fatalf("expected no player, got %+v", body)
	}
}

// TestCareerEndpointReturnsLogbook checks the fields the UI relies on are
// actually serialized (rank and squadron were once tagged rankName/squadronName
// and read as empty by the frontend).
func TestCareerEndpointReturnsLogbook(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetLogbook(dcsdata.Logbook{
		CurrentPlayer: "Laurent Keller",
		Players: []dcsdata.PlayerCareer{{
			Name:     "Laurent Keller",
			Rank:     "Second lieutenant",
			Squadron: "FlSt17",
			Aircraft: []dcsdata.AircraftCareer{{Type: "M-2000C", FlightHours: 4069.9}},
		}},
	})

	rec := httptest.NewRecorder()
	s.handleCareer(rec, httptest.NewRequest(http.MethodGet, "/api/career", nil))

	var body struct {
		CurrentPlayer string `json:"currentPlayer"`
		Players       []struct {
			Name     string `json:"name"`
			Rank     string `json:"rank"`
			Squadron string `json:"squadron"`
			Aircraft []struct {
				Type        string  `json:"type"`
				FlightHours float64 `json:"flightHours"`
			} `json:"aircraft"`
		} `json:"players"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CurrentPlayer != "Laurent Keller" {
		t.Errorf("currentPlayer = %q", body.CurrentPlayer)
	}
	if len(body.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(body.Players))
	}
	p := body.Players[0]
	if p.Rank != "Second lieutenant" || p.Squadron != "FlSt17" {
		t.Errorf("rank/squadron not serialized as the UI reads them: %+v", p)
	}
	if len(p.Aircraft) != 1 || p.Aircraft[0].FlightHours != 4069.9 {
		t.Errorf("aircraft not serialized: %+v", p.Aircraft)
	}
}
