package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestConfigEndpointEmpty checks a well-formed answer when nothing was read.
func TestConfigEndpointEmpty(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleDCSConfig(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Sections []any `json:"sections"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Sections) != 0 {
		t.Fatalf("expected no section, got %+v", body)
	}
}

// TestConfigEndpointSectionFilter checks ?section= returns one section only, and
// that an unknown name is an empty section rather than an error.
func TestConfigEndpointSectionFilter(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetDCSConfig(dcsdata.DCSConfig{
		Language: "fr",
		Sections: []dcsdata.ConfigSection{
			{Name: "graphics", Settings: []dcsdata.Setting{{Key: "MSAA", Value: "4"}}},
			{Name: "difficulty", Settings: []dcsdata.Setting{{Key: "immortal", Value: "off"}}},
		},
		Plugins: []dcsdata.Plugin{{Name: "Kola", Enabled: false}},
	})

	rec := httptest.NewRecorder()
	s.handleDCSConfig(rec, httptest.NewRequest(http.MethodGet, "/api/config?section=graphics", nil))
	var one struct {
		Name     string            `json:"name"`
		Settings []dcsdata.Setting `json:"settings"`
		Count    int               `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if one.Name != "graphics" || one.Count != 1 || len(one.Settings) != 1 || one.Settings[0].Value != "4" {
		t.Fatalf("section filter wrong: %+v", one)
	}

	rec = httptest.NewRecorder()
	s.handleDCSConfig(rec, httptest.NewRequest(http.MethodGet, "/api/config?section=nope", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown section should answer 200, got %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &one)
	if one.Count != 0 {
		t.Errorf("unknown section should be empty, got %+v", one)
	}

	// Without ?section, the whole config comes back.
	rec = httptest.NewRecorder()
	s.handleDCSConfig(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var all dcsdata.DCSConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Sections) != 2 || all.Language != "fr" || len(all.Plugins) != 1 {
		t.Fatalf("whole config wrong: %+v", all)
	}
}
