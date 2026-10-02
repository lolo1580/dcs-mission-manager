package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/mapping"
)

// newMappingStore builds a store with a transport that discards the commands, so
// the endpoints can be exercised without DCS-BIOS.
func newMappingStore(t *testing.T) *mapping.Store {
	t.Helper()
	return mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
}

// TestMappingsEndpointRequiresAircraft checks every write needs an aircraft: a
// profile with no aircraft would overwrite nothing usefully and could confuse the
// store on the next load.
func TestMappingsEndpointRequiresAircraft(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/mappings", strings.NewReader(`{"bindings":[]}`))
		s.handleMappings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s without aircraft: got %d, want 400", method, rec.Code)
		}
	}
}

// TestMappingsRoundTrip checks a profile saved through the API can be read back.
func TestMappingsRoundTrip(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	body := `{"bindings":[{"model":"pz70","control":"AP_BUTTON","command":"AP_BTN_Hdg","interface":"action"}]}`
	rec := httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodPost, "/api/mappings?aircraft=F-16C_50", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodGet, "/api/mappings?aircraft=F-16C_50", nil))
	var got struct {
		Profile mapping.Profile `json:"profile"`
		Enabled bool            `json:"enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Profile.Bindings) != 1 || got.Profile.Bindings[0].Command != "AP_BTN_Hdg" {
		t.Fatalf("round trip lost the binding: %+v", got.Profile)
	}
	if got.Enabled {
		t.Error("a fresh store must report itself disabled")
	}
}

// TestMappingSafetySwitch checks the switch turns sending on and off, and that a
// GET is refused: flipping it must be a deliberate POST.
func TestMappingSafetySwitch(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	rec := httptest.NewRecorder()
	s.handleMappingSafety(rec, httptest.NewRequest(http.MethodGet, "/api/mappings/safety", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleMappingSafety(rec, httptest.NewRequest(http.MethodPost, "/api/mappings/safety", strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("enable: got %d: %s", rec.Code, rec.Body.String())
	}
	if !s.mappings.Enabled() {
		t.Error("the store should be enabled")
	}

	rec = httptest.NewRecorder()
	s.handleMappingSafety(rec, httptest.NewRequest(http.MethodPost, "/api/mappings/safety", strings.NewReader(`{"enabled":false}`)))
	if rec.Code != http.StatusOK || s.mappings.Enabled() {
		t.Errorf("disable failed: code %d, enabled %v", rec.Code, s.mappings.Enabled())
	}
}

// TestMappingsUnavailable checks the endpoints say so clearly when no store was
// wired, rather than panicking on a nil pointer.
func TestMappingsUnavailable(t *testing.T) {
	s := &Server{hub: newHub()}
	for _, path := range []string{"/api/mappings", "/api/mappings/safety"} {
		rec := httptest.NewRecorder()
		s.handleMappings(rec, httptest.NewRequest(http.MethodGet, path, nil))
		s.handleMappingSafety(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d, want 503", path, rec.Code)
		}
	}
}

// TestControlsEndpointNeedsAircraft checks the catalogue needs to know which
// aircraft to describe.
func TestControlsEndpointNeedsAircraft(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleControls(rec, httptest.NewRequest(http.MethodGet, "/api/controls", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

// TestControlsEndpointUnknownAircraft checks an aircraft DCS-BIOS does not
// document answers an empty, well-formed catalogue rather than a 500: it is a
// normal case, not a failure.
func TestControlsEndpointUnknownAircraft(t *testing.T) {
	s := &Server{hub: newHub()}
	s.cfg.SavedGames = t.TempDir() // no DCS-BIOS metadata here

	rec := httptest.NewRecorder()
	s.handleControls(rec, httptest.NewRequest(http.MethodGet, "/api/controls?aircraft=NoSuchAircraft", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Available bool  `json:"available"`
		Controls  []any `json:"controls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Available {
		t.Error("an undocumented aircraft should report unavailable")
	}
	if body.Controls == nil {
		t.Error("controls should be an empty array, not null")
	}
}
