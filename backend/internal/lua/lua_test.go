package lua

import (
	"os"
	"testing"
)

func TestParseScalars(t *testing.T) {
	doc := `
mission_file_path = ".\\Mods\\x.miz"
callsign = "Cellar"
result = 0
mission_time = 1913.748
enabled = true
missing = nil
neg = -42
`
	got, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["callsign"] != "Cellar" {
		t.Errorf("callsign = %v", got["callsign"])
	}
	if got["result"] != float64(0) {
		t.Errorf("result = %v", got["result"])
	}
	if got["mission_time"] != 1913.748 {
		t.Errorf("mission_time = %v", got["mission_time"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %v", got["enabled"])
	}
	if got["neg"] != float64(-42) {
		t.Errorf("neg = %v", got["neg"])
	}
	if v, ok := got["missing"]; !ok || v != nil {
		t.Errorf("missing = %v (ok=%v)", v, ok)
	}
}

func TestParseArray(t *testing.T) {
	doc := `
events =
{
	[1] =
	{
		type = "mission start",
		t = 0,
	},
	[2] =
	{
		type = "land",
		t = 1470.82,
	},
}
`
	got, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	arr, ok := got["events"].([]any)
	if !ok {
		t.Fatalf("events should be an array, got %T", got["events"])
	}
	if len(arr) != 2 {
		t.Fatalf("want 2 events, got %d", len(arr))
	}
	first, ok := arr[0].(map[string]any)
	if !ok || first["type"] != "mission start" {
		t.Fatalf("unexpected first event: %#v", arr[0])
	}
	if first["t"] != float64(0) {
		t.Fatalf("t = %v", first["t"])
	}
}

func TestParseNestedObject(t *testing.T) {
	doc := `
world_state =
{
	[1] =
	{
		alt = 321.5,
		type = "M-2000C",
		dead = false,
		unitId = 3,
	},
}
`
	got, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ws, ok := got["world_state"].([]any)
	if !ok || len(ws) != 1 {
		t.Fatalf("world_state = %#v", got["world_state"])
	}
	unit := ws[0].(map[string]any)
	if unit["type"] != "M-2000C" || unit["unitId"] != float64(3) || unit["dead"] != false {
		t.Fatalf("unexpected unit: %#v", unit)
	}
}

func TestParseComments(t *testing.T) {
	doc := `
a = 1, -- trailing comment
-- full line comment
b = 2,
`
	got, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["a"] != float64(1) || got["b"] != float64(2) {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestParseStringEscapes(t *testing.T) {
	doc := `s = "line\nbreak\t\"quoted\""`
	got, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["s"] != "line\nbreak\t\"quoted\"" {
		t.Fatalf("s = %q", got["s"])
	}
}

func TestParseEmptyTable(t *testing.T) {
	got, err := Parse([]byte(`graveyard = {}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	arr, ok := got["graveyard"].([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("graveyard = %#v", got["graveyard"])
	}
}

// TestParseRealDebrief runs against a real DCS debrief.log when one is present.
// It is skipped otherwise, so the suite stays portable.
func TestParseRealDebrief(t *testing.T) {
	path := os.Getenv("DCSMM_TEST_DEBRIEF")
	if path == "" {
		t.Skip("set DCSMM_TEST_DEBRIEF to a debrief.log to run this test")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := got["events"]; !ok {
		t.Fatal("expected an 'events' key")
	}
	if _, ok := got["world_state"]; !ok {
		t.Fatal("expected a 'world_state' key")
	}
}
