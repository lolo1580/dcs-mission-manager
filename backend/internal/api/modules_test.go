package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestModulesEndpointEmpty checks the endpoint answers a well-formed empty list
// when the inventory could not be read (no DCS installed), so the UI shows
// "nothing found" rather than breaking.
func TestModulesEndpointEmpty(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleModules(rec, httptest.NewRequest(http.MethodGet, "/api/modules", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Modules []any `json:"modules"`
		Total   int   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Modules) != 0 || body.Total != 0 {
		t.Fatalf("expected empty inventory, got %+v", body)
	}
}

// TestModulesEndpointOwnedFilter checks ?owned=1 keeps only owned modules.
func TestModulesEndpointOwnedFilter(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetModules(dcsdata.ModuleInventory{
		Modules: []dcsdata.Module{
			{Category: "terrains", Title: "DCS: Caucasus", Owned: true},
			{Category: "terrains", Title: "DCS: Iraq", Owned: false},
		},
		Owned: 1,
		Total: 2,
	})

	rec := httptest.NewRecorder()
	s.handleModules(rec, httptest.NewRequest(http.MethodGet, "/api/modules?owned=1", nil))
	var body struct {
		Modules []dcsdata.Module `json:"modules"`
		Total   int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Modules) != 1 || !body.Modules[0].Owned {
		t.Fatalf("owned filter wrong: %+v", body.Modules)
	}
	if body.Total != 2 {
		t.Errorf("Total = %d, want 2 (the whole inventory)", body.Total)
	}
}
