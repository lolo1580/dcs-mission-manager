package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPanelsEndpointWithoutService checks the endpoint answers a well-formed
// "not supported" shape when no panel service was wired (a non-Windows build, or
// a test server). The UI reads `supported` to explain itself rather than showing
// an empty list.
func TestPanelsEndpointWithoutService(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handlePanels(rec, httptest.NewRequest(http.MethodGet, "/api/panels", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Supported bool  `json:"supported"`
		Devices   []any `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Supported {
		t.Error("supported should be false without a panel service")
	}
	if body.Devices == nil {
		t.Error("devices should be an empty array, not null")
	}
}

// TestDCSBIOSEndpointWithoutClient checks the DCS-BIOS endpoint answers a
// well-formed "unavailable" shape rather than failing.
func TestDCSBIOSEndpointWithoutClient(t *testing.T) {
	s := &Server{hub: newHub()}
	rec := httptest.NewRecorder()
	s.handleDCSBIOS(rec, httptest.NewRequest(http.MethodGet, "/api/dcsbios", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var body struct {
		Available bool `json:"available"`
		Connected bool `json:"connected"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Available || body.Connected {
		t.Errorf("expected unavailable and disconnected, got %+v", body)
	}
}
