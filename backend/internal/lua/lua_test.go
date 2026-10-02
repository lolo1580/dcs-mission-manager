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

	// --- DCS data-file constructs -------------------------------------------
	// These appear in Mods/terrains/<map>/radio.lua and beacons.lua. Without
	// support for them, reading DCS's own terrain data is impossible.
	extra := `
radio = {
	{
		radioId = 'airfield12_0';
		callsign = {{["common"] = {_("Anapa"), "Anapa"}}};
		frequency = {[VHF_HI] = {MODULATIONTYPE_AM, 121000000.000000}};
	};
}
beacons = {
	{
		beaconId = 'airfield12_0';
		type = BEACON_TYPE_ILS_FAR_HOMER;
		callsign = 'AP';
		positionGeo = { latitude = 45.039907, longitude = 37.396435 };
	};
}
`
	got2, err := Parse([]byte(extra))
	if err != nil {
		t.Fatalf("parse DCS constructs: %v", err)
	}

	radio, ok := got2["radio"].(map[string]any)
	if !ok {
		t.Fatalf("radio should be a map, got %T", got2["radio"])
	}
	entry, ok := radio["1"].(map[string]any)
	if !ok {
		t.Fatalf("radio[1] should be a map, got %T", radio["1"])
	}
	if entry["radioId"] != "airfield12_0" {
		t.Errorf("radioId = %v", entry["radioId"])
	}

	// _("...") must unwrap to the string, and the enum constant must survive as
	// its own name so beacon types can be told apart.
	cs, ok := entry["callsign"].(map[string]any)
	if !ok {
		t.Fatalf("callsign should be a map, got %T", entry["callsign"])
	}
	first, _ := cs["1"].(map[string]any)
	lang, _ := first["common"].(map[string]any)
	if lang["1"] != "Anapa" {
		t.Errorf("gettext call not unwrapped: %v", lang["1"])
	}

	freq, _ := entry["frequency"].(map[string]any)
	band, _ := freq["VHF_HI"].(map[string]any)
	if band["1"] != "MODULATIONTYPE_AM" {
		t.Errorf("enum constant lost: %v", band["1"])
	}
	if band["2"] != float64(121000000) {
		t.Errorf("frequency = %v", band["2"])
	}

	beacons, _ := got2["beacons"].(map[string]any)
	b, _ := beacons["1"].(map[string]any)
	if b["type"] != "BEACON_TYPE_ILS_FAR_HOMER" {
		t.Errorf("beacon type = %v", b["type"])
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

// TestTruncatedGettextDoesNotPanic covers a truncated file ending inside the
// gettext wrapper: _(" is the one value form that reaches parseString without a
// guaranteed quote, so it used to index past the end of the input.
func TestTruncatedGettextDoesNotPanic(t *testing.T) {
	for _, doc := range []string{"a = _(", "a = _(\n", "a = _(\"unterminated"} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("Parse(%q) should report an error, not succeed", doc)
		}
	}
}

// TestDecimalEscapeRange: Lua rejects \256, and byte(n) would silently wrap it
// to 0.
func TestDecimalEscapeRange(t *testing.T) {
	if _, err := Parse([]byte(`s = "\256"`)); err == nil {
		t.Error("a decimal escape above 255 should be reported")
	}
	got, err := Parse([]byte(`s = "\065"`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["s"] != "A" {
		t.Errorf(`"\065" = %q, want "A"`, got["s"])
	}
}

// TestTableShapes documents the two shapes the parser produces, and why.
//
// This distinction is load-bearing: DCS's radio.lua and beacons.lua are written
// as positional entries ({ {…}, {…} }), and the extractor reads them as a map.
// A table keyed [1]..[n] instead collapses to a slice, which is the form the
// debrief uses. Getting this backwards silently empties the airfield dataset.
func TestTableShapes(t *testing.T) {
	// Positional entries -> map keyed "1".."n".
	pos, err := Parse([]byte(`t = {"a", "b", "c"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m, ok := pos["t"].(map[string]any); !ok || m["1"] != "a" || m["3"] != "c" {
		t.Fatalf("positional entries should be a map keyed 1..n, got %#v", pos["t"])
	}

	// Explicit [1]..[n] -> slice.
	exp, err := Parse([]byte(`t = {[1] = "a", [2] = "b"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if arr, ok := exp["t"].([]any); !ok || len(arr) != 2 {
		t.Fatalf("explicit 1..n keys should collapse to a slice, got %#v", exp["t"])
	}
}

// TestMixedBareAndExplicitKeys covers a table using both forms. The explicit
// keys must survive: the parser used to renumber the positional values from 1,
// silently overwriting them.
func TestMixedBareAndExplicitKeys(t *testing.T) {
	got, err := Parse([]byte(`t = {[10] = "x", "a", [20] = "y"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, ok := got["t"].(map[string]any)
	if !ok {
		t.Fatalf("a mixed table should stay a map, got %T", got["t"])
	}
	if m["10"] != "x" || m["20"] != "y" {
		t.Errorf("explicit keys lost: %#v", m)
	}
	found := false
	for _, v := range m {
		if v == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("the positional value was lost: %#v", m)
	}
}

// TestExplicitKeyWinsItsSlot documents Lua's own behaviour: an explicit [1]
// overwrites a positional value that landed in slot 1.
func TestExplicitKeyWinsItsSlot(t *testing.T) {
	got, err := Parse([]byte(`t = {"a", [1] = "b"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, ok := got["t"].(map[string]any)
	if !ok {
		t.Fatalf("expected a map, got %T", got["t"])
	}
	if m["1"] != "b" {
		t.Errorf(`t[1] = %v, want the explicit "b" (Lua's rule)`, m["1"])
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
	path := os.Getenv("DCSMANAGER_TEST_DEBRIEF")
	if path == "" {
		t.Skip("set DCSMANAGER_TEST_DEBRIEF to a debrief.log to run this test")
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
