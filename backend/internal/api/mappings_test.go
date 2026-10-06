package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/debuglog"
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

// TestMappingTestSwitch checks the live-mapping-test switch toggles and that a GET
// is refused, mirroring the sending switch.
func TestMappingTestSwitch(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	rec := httptest.NewRecorder()
	s.handleMappingTest(rec, httptest.NewRequest(http.MethodGet, "/api/mappings/test", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleMappingTest(rec, httptest.NewRequest(http.MethodPost, "/api/mappings/test", strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK || !s.TestMode() {
		t.Fatalf("enable: code %d, testMode %v", rec.Code, s.TestMode())
	}
	if s.mappings.Enabled() {
		t.Error("the test switch must not arm command sending")
	}

	rec = httptest.NewRecorder()
	s.handleMappingTest(rec, httptest.NewRequest(http.MethodPost, "/api/mappings/test", strings.NewReader(`{"enabled":false}`)))
	if rec.Code != http.StatusOK || s.TestMode() {
		t.Errorf("disable failed: code %d, testMode %v", rec.Code, s.TestMode())
	}
}

// TestDisplayPreviewNoValue checks the preview reports "value not delivered"
// rather than a wrong number when DCS-BIOS has not sent the address yet.
func TestDisplayPreviewNoValue(t *testing.T) {
	s := &Server{hub: newHub()}
	s.cfg.SavedGames = t.TempDir() // no metadata
	rec := httptest.NewRecorder()
	body := `{"aircraft":"X","command":"ALT_SEL","export":0,"scale":1,"line":"upper"}`
	s.handleDisplayPreview(rec, httptest.NewRequest(http.MethodPost, "/api/display/preview", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var out struct {
		Available bool `json:"available"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Available {
		t.Error("with no metadata/value the preview should be unavailable")
	}
}

// TestDisplayTestNoPanels checks the hardware test writes nothing (and does not
// panic) when no PZ70 is connected.
func TestDisplayTestNoPanels(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleDisplayTest(rec, httptest.NewRequest(http.MethodPost, "/api/display/test", strings.NewReader(`{"upper":12345,"lower":-1234}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var out struct {
		Written int `json:"written"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Written != 0 {
		t.Errorf("wrote %d with no panel", out.Written)
	}
}

// TestOutputsSwitchEndpoint checks the panel-outputs switch is separate from the
// command-sending switch, and that a GET is refused.
func TestOutputsSwitchEndpoint(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	rec := httptest.NewRecorder()
	s.handleOutputsSafety(rec, httptest.NewRequest(http.MethodGet, "/api/mappings/outputs", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d, want 405", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleOutputsSafety(rec, httptest.NewRequest(http.MethodPost, "/api/mappings/outputs", strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK || !s.mappings.OutputsEnabled() {
		t.Fatalf("enable: code %d, outputs %v", rec.Code, s.mappings.OutputsEnabled())
	}
	if s.mappings.Enabled() {
		t.Error("turning outputs on must not arm command sending")
	}
}

// TestDebugEndpoint checks the switch toggles and the log is served, and that an
// unavailable logger is reported rather than panicking.
func TestDebugEndpoint(t *testing.T) {
	// No logger: report unavailable, empty log.
	bare := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	bare.handleDebug(rec, httptest.NewRequest(http.MethodGet, "/api/debug", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("unavailable debug: got %d", rec.Code)
	}

	logger := debuglog.New(false, nil)
	s := &Server{hub: newHub(), debug: logger}

	rec = httptest.NewRecorder()
	s.handleDebug(rec, httptest.NewRequest(http.MethodPost, "/api/debug", strings.NewReader(`{"enabled":true}`)))
	if rec.Code != http.StatusOK || !s.DebugEnabled() {
		t.Fatalf("enable: code %d, enabled %v", rec.Code, s.DebugEnabled())
	}

	logger.Infof("app", "hello")
	rec = httptest.NewRecorder()
	s.handleLog(rec, httptest.NewRequest(http.MethodGet, "/api/log", nil))
	var body struct {
		Lines []debuglog.Line `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range body.Lines {
		if l.Message == "hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the recorded line should be served: %+v", body.Lines)
	}

	// A GET must not flip the switch.
	rec = httptest.NewRecorder()
	s.handleDebug(rec, httptest.NewRequest(http.MethodGet, "/api/debug", nil))
	if !s.DebugEnabled() {
		t.Error("a GET must not change the switch")
	}
}

// TestAircraftEndpoint checks the editor can list what to edit even with no DCS
// running, which is the whole point of choosing the aircraft by hand.
func TestAircraftEndpoint(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	rec := httptest.NewRecorder()
	s.handleAircraft(rec, httptest.NewRequest(http.MethodGet, "/api/aircraft", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Aircraft []string `json:"aircraft"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// The starter profiles are seeded on a fresh file, so F-16C_50 is offered even
	// without a mission: the mapping can be edited and tested offline.
	found := false
	for _, a := range body.Aircraft {
		if a == "F-16C_50" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the seeded aircraft should be listed: %v", body.Aircraft)
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

// TestMappingsOutputsRoundTrip checks output (LED) bindings survive a profile save
// and reload, and that a client that does not send them leaves the existing ones
// alone rather than wiping them.
func TestMappingsOutputsRoundTrip(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}

	body := `{"bindings":[],"outputs":[{"model":"pz55","target":"LIGHT_GEAR_LEFT","command":"LIGHT_GEAR_L","color":"red"}]}`
	rec := httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodPost, "/api/mappings?aircraft=F-16C_50", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodGet, "/api/mappings?aircraft=F-16C_50", nil))
	var got struct {
		Profile mapping.Profile `json:"profile"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Profile.Outputs) != 1 || got.Profile.Outputs[0].Target != "LIGHT_GEAR_LEFT" {
		t.Fatalf("round trip lost the output: %+v", got.Profile.Outputs)
	}

	// A client that only knows bindings (no outputs field) must not clear them.
	rec = httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodPost, "/api/mappings?aircraft=F-16C_50", strings.NewReader(`{"bindings":[]}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("binding-only save: got %d: %s", rec.Code, rec.Body.String())
	}
	if n := len(s.mappings.Profile("F-16C_50").Outputs); n != 1 {
		t.Fatalf("a binding-only save wiped the outputs (%d left)", n)
	}
}

// TestMappingsEndpointRejectsABadOutput checks a malformed output is refused
// rather than persisted: the target must name a real indicator.
func TestMappingsEndpointRejectsABadOutput(t *testing.T) {
	s := &Server{hub: newHub(), mappings: newMappingStore(t)}
	body := `{"bindings":[],"outputs":[{"model":"pz55","target":"NOT_AN_INDICATOR","command":"X"}]}`
	rec := httptest.NewRecorder()
	s.handleMappings(rec, httptest.NewRequest(http.MethodPost, "/api/mappings?aircraft=F-16C_50", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", rec.Code, rec.Body.String())
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
