package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeCommander stands in for the TCP listener: it records what was sent and
// reports a fixed number of connected hooks.
type fakeCommander struct {
	connected int
	sent      []any
}

func (f *fakeCommander) SendCommand(v any) int {
	if f.connected == 0 {
		return 0
	}
	f.sent = append(f.sent, v)
	return f.connected
}

func (f *fakeCommander) Connected() int { return f.connected }

// TestChatPostWithoutDCS checks that sending with no hook connected answers 503
// (a normal "DCS not running" state) rather than pretending to succeed.
func TestChatPostWithoutDCS(t *testing.T) {
	s := &Server{commander: &fakeCommander{connected: 0}, hub: newHub()}
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hi"}`))
	rec := httptest.NewRecorder()
	s.handleChat(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
}

// TestChatPostReachesDCS checks the happy path: the message is handed to the
// commander, which reports it delivered.
func TestChatPostReachesDCS(t *testing.T) {
	fc := &fakeCommander{connected: 1}
	s := &Server{commander: fc, hub: newHub()}
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hi there"}`))
	rec := httptest.NewRecorder()
	s.handleChat(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if len(fc.sent) != 1 {
		t.Fatalf("commander received %d command(s), want 1", len(fc.sent))
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad response body: %v", err)
	}
	if body["sent"] != float64(1) {
		t.Fatalf("sent = %v, want 1", body["sent"])
	}
}

// TestChatPostEmptyMessage checks the message is required.
func TestChatPostEmptyMessage(t *testing.T) {
	s := &Server{commander: &fakeCommander{connected: 1}, hub: newHub()}
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":""}`))
	rec := httptest.NewRecorder()
	s.handleChat(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}
