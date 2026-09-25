package udp

import (
	"encoding/json"
	"testing"
	"time"

	"dcsmm/internal/category"
	"dcsmm/internal/state"
)

func TestOwnshipMessageRoundTrip(t *testing.T) {
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

func TestHandleOwnship(t *testing.T) {
	store := state.New(time.Minute, 0)
	l := NewListener(store, category.New(""))

	l.handle(&Message{
		Type:      "ownship",
		Name:      "Player",
		UnitType:  "F-16C_50",
		Coalition: "blue",
		Lat:       41.5,
		Lng:       41.8,
		Alt:       5000,
	})

	units := store.Snapshot()
	if len(units) != 1 {
		t.Fatalf("want 1 unit, got %d", len(units))
	}
	u := units[0]
	if u.ID != "ownship" || !u.Ownship || u.Category != category.Plane {
		t.Fatalf("unexpected ownship: %+v", u)
	}
	if u.Label != "Player" {
		t.Fatalf("want label Player, got %q", u.Label)
	}
}

func TestHandleWorld(t *testing.T) {
	store := state.New(time.Minute, 0)
	l := NewListener(store, category.New(""))

	l.handle(&Message{
		Type: "world",
		Units: []World{
			{ID: "42", Type: "T-72B", Coalition: "red", Lat: 42.1, Lng: 41.2},
			{ID: "43", Type: "Su-27", Coalition: "red", Lat: 42.2, Lng: 41.3},
			{ID: "", Type: "Ignored"},
		},
	})

	units := store.Snapshot()
	if len(units) != 2 {
		t.Fatalf("want 2 units (empty id skipped), got %d", len(units))
	}
	byID := map[string]state.Unit{}
	for _, u := range units {
		byID[u.ID] = u
	}
	if byID["42"].Category != category.Ground {
		t.Fatalf("T-72B should be ground, got %q", byID["42"].Category)
	}
	if byID["43"].Category != category.Plane {
		t.Fatalf("Su-27 should be plane, got %q", byID["43"].Category)
	}
}

func TestHandleUnknownType(t *testing.T) {
	store := state.New(time.Minute, 0)
	l := NewListener(store, category.New(""))
	l.handle(&Message{Type: "whatever"})
	l.handle(&Message{Type: ""})
	if n := store.Count(); n != 0 {
		t.Fatalf("unknown messages should not create units, got %d", n)
	}
}
