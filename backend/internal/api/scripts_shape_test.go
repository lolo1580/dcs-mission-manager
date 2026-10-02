package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dcsmanager/internal/dcsdata"
)

// TestScriptsEndpointShape locks the JSON keys the frontend reads, and guards
// against re-introducing a field that no longer exists (a "legacy" list was
// removed once the project became fresh-install only).
func TestScriptsEndpointShape(t *testing.T) {
	s := &Server{hub: newHub()}
	s.SetScripts(dcsdata.ScriptStatus{
		SavedGames: `C:\SG`,
		Managed:    []dcsdata.ScriptState{{DestRel: "Scripts/Hooks/dcsmanager.lua", State: "installed"}},
		Exports:    []string{"Tacview"},
		ThirdParty: []dcsdata.ThirdPartyHook{{Name: "bhHook.lua", Dir: "Scripts/Hooks", Tool: "BattleHub"}},
	})
	rec := httptest.NewRecorder()
	s.handleScripts(rec, httptest.NewRequest(http.MethodGet, "/api/scripts", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, key := range []string{`"managed"`, `"exports"`, `"thirdParty"`, `"savedGames"`} {
		if !strings.Contains(body, key) {
			t.Errorf("response should contain %s: %s", key, body)
		}
	}
	if strings.Contains(body, `"legacy"`) {
		t.Errorf("response should not carry a legacy list any more: %s", body)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 4 {
		t.Errorf("expected exactly 4 fields, got %d: %v", len(parsed), parsed)
	}
}
