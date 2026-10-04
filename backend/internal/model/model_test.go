package model

import (
	"encoding/json"
	"testing"
)

// TestPlayersEmptyObjectIsAccepted locks the fix for the roster the Lua
// serializer sends as {} when no player is connected. The strict decode used to
// reject the whole line, which dropped the message.
func TestPlayersEmptyObjectIsAccepted(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"players","players":{}}`), &m)
	if err != nil {
		t.Fatalf("empty roster object should decode, got %v", err)
	}
	if m.Type != "players" {
		t.Errorf("type = %q", m.Type)
	}
	if len(m.Players) != 0 {
		t.Errorf("players = %v, want empty", m.Players)
	}
}

// TestPlayersArrayIsAccepted is the normal shape, which must keep working.
func TestPlayersArrayIsAccepted(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"players","players":[{"id":7,"name":"Cellar","side":2}]}`), &m)
	if err != nil {
		t.Fatalf("array roster should decode, got %v", err)
	}
	if len(m.Players) != 1 || m.Players[0].Name != "Cellar" || m.Players[0].ID != 7 {
		t.Fatalf("players = %+v", m.Players)
	}
}

// TestPlayersObjectKeyedByID covers the other object shape: a roster keyed by
// player id, which an older hook produced with pairs().
func TestPlayersObjectKeyedByID(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"players","players":{"7":{"id":7,"name":"A"},"9":{"id":9,"name":"B"}}}`), &m)
	if err != nil {
		t.Fatalf("keyed roster should decode, got %v", err)
	}
	if len(m.Players) != 2 {
		t.Fatalf("players = %+v", m.Players)
	}
}

// TestOtherFieldsSurvive is the real regression: the roster used to take the
// whole message down with it. A malformed players field must not stop the event
// it was sent with.
func TestOtherFieldsSurvive(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"event","event":"kill","args":[1,"F-16C"],"players":{}}`), &m)
	if err != nil {
		t.Fatalf("message should decode despite the roster shape, got %v", err)
	}
	if m.Event != "kill" || len(m.Args) != 2 {
		t.Fatalf("event fields lost: %+v", m)
	}
}

// TestTrulyMalformedPlayersStillFails: a shape that is neither an array nor an
// object must still be reported, so a real bug is not hidden.
func TestTrulyMalformedPlayersStillFails(t *testing.T) {
	var m Message
	if err := json.Unmarshal([]byte(`{"type":"players","players":42}`), &m); err == nil {
		t.Fatal("a number roster should fail to decode")
	}
}

// TestEmptyOptionsArrayIsAccepted: the Lua serializer emits [] for an empty
// table, and mission options are a map, so an empty list means "no options".
func TestEmptyOptionsArrayIsAccepted(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"mission","phase":"start","name":"m","options":[]}`), &m)
	if err != nil {
		t.Fatalf("empty options array should decode, got %v", err)
	}
	if m.Phase != "start" || m.Name != "m" {
		t.Fatalf("mission fields lost: %+v", m)
	}
	if len(m.Options) != 0 {
		t.Errorf("options = %v, want empty", m.Options)
	}
}

// TestOptionsObjectIsAccepted is the normal shape for a mission with options.
func TestOptionsObjectIsAccepted(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"type":"mission","phase":"start","options":{"optionsView":"all"}}`), &m)
	if err != nil {
		t.Fatalf("options object should decode, got %v", err)
	}
	if m.Options["optionsView"] != "all" {
		t.Fatalf("options = %v", m.Options)
	}
}
