package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestScriptsEndpointEmpty checks a well-formed picture when nothing was read:
// the UI must show "nothing found" rather than break.
func TestScriptsEndpointEmpty(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleScripts(rec, httptest.NewRequest(http.MethodGet, "/api/scripts", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Managed    []any `json:"managed"`
		ThirdParty []any `json:"thirdParty"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Managed) != 0 || len(body.ThirdParty) != 0 {
		t.Fatalf("expected an empty picture, got %+v", body)
	}
}

// TestScriptsEndpointReturnsStatus checks the fields the UI reads are present.
func TestScriptsEndpointReturnsStatus(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetScripts(dcsdata.ScriptStatus{
		SavedGames: `C:\SG`,
		Managed:    []dcsdata.ScriptState{{DestRel: "Scripts/Hooks/dcsmanager.lua", State: "installed"}},
		Exports:    []string{"Tacview"},
		Legacy:     []string{"Config/dcsmm.cfg"},
		ThirdParty: []dcsdata.ThirdPartyHook{{Name: "bhHook.lua", Dir: "Scripts/Hooks", Tool: "BattleHub", SizeBytes: 8711}},
	})

	rec := httptest.NewRecorder()
	s.handleScripts(rec, httptest.NewRequest(http.MethodGet, "/api/scripts", nil))
	var body struct {
		SavedGames string `json:"savedGames"`
		Managed    []struct {
			DestRel string `json:"destRel"`
			State   string `json:"state"`
		} `json:"managed"`
		Exports    []string `json:"exports"`
		Legacy     []string `json:"legacy"`
		ThirdParty []struct {
			Name string `json:"name"`
			Tool string `json:"tool"`
		} `json:"thirdParty"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Managed) != 1 || body.Managed[0].State != "installed" {
		t.Errorf("managed = %+v", body.Managed)
	}
	if len(body.Exports) != 1 || body.Exports[0] != "Tacview" {
		t.Errorf("exports = %v", body.Exports)
	}
	if len(body.Legacy) != 1 || body.Legacy[0] != "Config/dcsmm.cfg" {
		t.Errorf("legacy = %v", body.Legacy)
	}
	if len(body.ThirdParty) != 1 || body.ThirdParty[0].Tool != "BattleHub" {
		t.Errorf("third party = %+v", body.ThirdParty)
	}
}

// TestModsEndpoint checks the mods listing is an array with a count.
func TestModsEndpoint(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetMods([]dcsdata.InstalledMod{
		{Category: "aircraft", Name: "A-4E-C", SizeBytes: 1024, Files: 2, HasEntryLua: true},
	})
	rec := httptest.NewRecorder()
	s.handleMods(rec, httptest.NewRequest(http.MethodGet, "/api/mods", nil))
	var body struct {
		Mods []struct {
			Name        string `json:"name"`
			HasEntryLua bool   `json:"hasEntryLua"`
		} `json:"mods"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Mods) != 1 || !body.Mods[0].HasEntryLua {
		t.Fatalf("mods = %+v", body)
	}
}
