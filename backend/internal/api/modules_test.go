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

// TestModulesEndpointInstalledFilter checks ?installed=1 keeps only the modules
// actually present on disk, and that the summary counters are the whole
// inventory's, not the filtered page's.
func TestModulesEndpointInstalledFilter(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetModules(dcsdata.ModuleInventory{
		Modules: []dcsdata.Module{
			{Category: "terrains", Title: "DCS: Caucasus", Owned: true, InstallKnown: true, Installed: true},
			{Category: "terrains", Title: "DCS: Kola", Owned: true, InstallKnown: true, Installed: false},
			{Category: "campaigns", Title: "Some campaign", Owned: false},
		},
		Owned:     2,
		Installed: 1,
		Total:     3,
	})

	rec := httptest.NewRecorder()
	s.handleModules(rec, httptest.NewRequest(http.MethodGet, "/api/modules?installed=1", nil))
	var body struct {
		Modules   []dcsdata.Module `json:"modules"`
		Owned     int              `json:"owned"`
		Installed int              `json:"installed"`
		Total     int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Modules) != 1 || body.Modules[0].Title != "DCS: Caucasus" {
		t.Fatalf("installed filter wrong: %+v", body.Modules)
	}
	if body.Owned != 2 || body.Installed != 1 || body.Total != 3 {
		t.Errorf("summary should describe the whole inventory: %+v", body)
	}
}
