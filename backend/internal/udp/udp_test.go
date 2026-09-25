package udp

import (
	"encoding/json"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	raw := `{"type":"ownship","name":"Player","unitType":"F-16C_50",` +
		`"coalition":"blue","lat":41.5,"lng":41.8,"alt":5000,"heading":123,"modelTime":42.5}`

	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Name != "Player" || m.UnitType != "F-16C_50" || m.Coalition != "blue" {
		t.Fatalf("unexpected message: %+v", m)
	}
	if m.Lat != 41.5 || m.Lng != 41.8 || m.Alt != 5000 || m.Heading != 123 {
		t.Fatalf("unexpected coordinates: %+v", m)
	}
}
