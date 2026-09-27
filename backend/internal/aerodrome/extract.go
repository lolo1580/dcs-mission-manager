package aerodrome

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dcsmm/internal/lua"
)

// Beacon type constants, as written in DCS's beacons.lua. Only the ones we
// surface are listed; anything else is ignored rather than guessed at.
const (
	beaconTACAN   = "BEACON_TYPE_TACAN"
	beaconVOR     = "BEACON_TYPE_VOR"
	beaconVORDME  = "BEACON_TYPE_VOR_DME"
	beaconVORTAC  = "BEACON_TYPE_VORTAC"
	beaconRSBN    = "BEACON_TYPE_RSBN"
	beaconILS     = "BEACON_TYPE_ILS_LOCALIZER"
	beaconPRMG    = "BEACON_TYPE_PRMG_LOCALIZER"
	beaconNDB     = "BEACON_TYPE_AIRPORT_HOMER"
	beaconNDBMark = "BEACON_TYPE_AIRPORT_HOMER_WITH_MARKER"
	// A plain homer is a stand-alone non-directional beacon (an ADF the pilot can
	// tune), and the far/near homers are the outer/inner markers of an ILS. All
	// three carry an LF frequency and are shown as NDBs.
	beaconHomer   = "BEACON_TYPE_HOMER"
	beaconILSFar  = "BEACON_TYPE_ILS_FAR_HOMER"
	beaconILSNear = "BEACON_TYPE_ILS_NEAR_HOMER"
)

// Frequency bands, used to reject a navigation aid whose declared frequency
// cannot belong to its type.
//
// DCS occasionally declares a beacon with a frequency outside the band its type
// uses — Ivalo's ILS is listed at 212 MHz, and Sas Al Nakheel's VOR at
// 128.925 MHz. That is not a parsing mistake: the value really is in the file.
// Showing it would be worse than showing nothing, because no pilot can tune an
// ILS at 212 MHz. Such aids are dropped, and the count is reported at startup so
// the data problem stays visible instead of being silently swallowed.
const (
	ilsMinMHz, ilsMaxMHz = 108.10, 111.95 // ILS localizer / glideslope, PRMG
	vorMinMHz, vorMaxMHz = 108.00, 117.95 // VOR and VOR/DME
	ndbMinKHz, ndbMaxKHz = 190.0, 1750.0  // NDB / marker beacons
)

func inILSband(mhz float64) bool { return mhz >= ilsMinMHz && mhz <= ilsMaxMHz }
func inVORband(mhz float64) bool { return mhz >= vorMinMHz && mhz <= vorMaxMHz }
func inNDBband(khz float64) bool { return khz >= ndbMinKHz && khz <= ndbMaxKHz }

// Terrain holds everything read from one DCS terrain folder.
type Terrain struct {
	Airfields []Aerodrome
	Towns     []Town
	// RadioAirfields and BeaconAirfields record what was found before merging,
	// for diagnostics ("DCS declares 21 airfields, 6 TACAN, 13 ILS").
	RadioAirfields  int
	BeaconAirfields int
	// BeaconTotal counts the beacons tied to an airfield. Beacons with a
	// "world_" id (a VOR in the middle of a desert, for instance) belong to no
	// airfield and are ignored: they must not turn into an aerodrome.
	BeaconTotal int
	// Dropped lists navigation aids whose declared frequency does not belong to
	// their type (see the band constants above). They are reported so a data
	// problem in DCS stays visible rather than silently disappearing.
	Dropped []string
}

// Town is a named settlement with real coordinates, from map/towns.lua.
type Town struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// LoadTerrain reads a terrain folder. Missing files are not an error: most
// terrains have radio.lua and beacons.lua, but a few have no towns.
func LoadTerrain(dir, theatre string) (Terrain, error) {
	var t Terrain

	radio, err := loadRadio(filepath.Join(dir, "radio.lua"))
	if err != nil && !os.IsNotExist(err) {
		return t, fmt.Errorf("radio.lua: %w", err)
	}

	beacons, err := loadBeacons(filepath.Join(dir, "beacons.lua"))
	if err != nil && !os.IsNotExist(err) {
		return t, fmt.Errorf("beacons.lua: %w", err)
	}

	towns, err := loadTowns(filepath.Join(dir, "map", "towns.lua"))
	if err != nil && !os.IsNotExist(err) {
		return t, fmt.Errorf("towns.lua: %w", err)
	}
	t.Towns = towns

	t.RadioAirfields = len(radio)
	t.BeaconTotal = len(beacons)

	// Merge: the two files describe the same airfields and share an identifier
	// (airfield22_0 = Batumi in both). The radio file carries the name and the
	// ATC frequency, the beacon file the navigation aids and the true position.
	merged, dropped := mergeAirfields(radio, beacons, theatre)
	t.Airfields = merged
	t.Dropped = dropped

	for _, a := range merged {
		if a.Lat != 0 || a.Lng != 0 {
			t.BeaconAirfields++
		}
	}
	return t, nil
}

// radioEntry is one airfield as described by radio.lua.
type radioEntry struct {
	callsign  string
	icao      string
	towerHz   float64
	hasTower  bool
	airfieldN int // from airfield12_0 -> 12
}

// tableEntries returns the entries of a parsed Lua table regardless of the shape
// the parser produced: a map (keyed "1".."n" for positional entries) or a slice
// (explicit [1]..[n] keys collapse to a slice). Without this, a data file written
// in the other style is silently treated as empty.
func tableEntries(v any) []any {
	switch t := v.(type) {
	case map[string]any:
		// Sort by numeric key so the result does not depend on map iteration
		// order, which would otherwise make the chosen airfield position and the
		// order of its navigation aids vary between runs.
		keys := make([]int, 0, len(t))
		for k := range t {
			if n, ok := numericKey(k); ok {
				keys = append(keys, n)
			}
		}
		sort.Ints(keys)
		out := make([]any, 0, len(keys))
		for _, n := range keys {
			out = append(out, t[strconv.Itoa(n)])
		}
		return out
	case []any:
		return t
	default:
		return nil
	}
}

// numericKey reports whether a table key is a positive integer, and returns it.
func numericKey(k string) (int, bool) {
	n, err := strconv.Atoi(k)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// loadRadio parses radio.lua. Entries are identified by their radioId.
func loadRadio(path string) (map[string]radioEntry, error) {
	root, err := parseLuaFile(path)
	if err != nil {
		return nil, err
	}
	entries := tableEntries(root["radio"])
	if entries == nil {
		return nil, nil
	}

	out := make(map[string]radioEntry)
	for _, v := range entries {
		e, ok := v.(map[string]any)
		if !ok {
			continue
		}
		id, _ := e["radioId"].(string)
		if !strings.HasPrefix(id, "airfield") {
			continue
		}

		entry := radioEntry{airfieldN: airfieldNumber(id)}

		// callsign = {{["common"] = {_("OMAA"), "OMAA"}}}
		// The first value is the human name, the second the code (often an ICAO).
		if name, code := firstCallsign(e["callsign"]); name != "" {
			entry.callsign = name
			entry.icao = code
		}

		// frequency = {[VHF_HI] = {MODULATIONTYPE_AM, 121000000}} — Hz.
		if hz, ok := towerFrequency(e["frequency"]); ok {
			entry.towerHz = hz
			entry.hasTower = true
		}

		out[id] = entry
	}
	return out, nil
}

// firstCallsign digs the display name and code out of the nested callsign table.
// The shape is callsign -> [1] -> language -> {[1] = name, [2] = code}, and the
// language key varies (common, nato, ussr), so we descend until a name pair is
// found rather than assuming a fixed layout.
func firstCallsign(v any) (name, code string) {
	return findNamePair(v, 0)
}

func findNamePair(v any, depth int) (string, string) {
	if depth > 4 {
		return "", ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return "", ""
	}
	// A leaf: {"1": "Anapa", "2": "Anapa"}.
	if n, ok := m["1"].(string); ok {
		c, _ := m["2"].(string)
		return n, c
	}
	for _, k := range sortedKeys(m) {
		if n, c := findNamePair(m[k], depth+1); n != "" {
			return n, c
		}
	}
	return "", ""
}

// towerFrequency returns the ATC frequency in Hz, preferring VHF_HI (which is
// what DCS uses for tower/approach), then UHF, then any band available.
func towerFrequency(v any) (float64, bool) {
	freq, ok := v.(map[string]any)
	if !ok {
		return 0, false
	}
	for _, band := range []string{"VHF_HI", "UHF", "VHF_LOW", "HF"} {
		pair, ok := freq[band].(map[string]any)
		if !ok {
			continue
		}
		if hz, ok := pair["2"].(float64); ok && hz > 0 {
			return hz, true
		}
	}
	return 0, false
}

// beaconEntry is one navigation aid, from beacons.lua.
type beaconEntry struct {
	id       string
	name     string
	kind     string
	callsign string
	channel  int
	hz       float64
	lat, lng float64
	// alt is the beacon's altitude in metres, from position[2] (DCS uses a
	// Y-up Vec3, so the second component is the height).
	alt float64
}

// loadBeacons parses beacons.lua.
func loadBeacons(path string) ([]beaconEntry, error) {
	root, err := parseLuaFile(path)
	if err != nil {
		return nil, err
	}
	entries := tableEntries(root["beacons"])
	if entries == nil {
		return nil, nil
	}

	var out []beaconEntry
	for _, v := range entries {
		e, ok := v.(map[string]any)
		if !ok {
			continue
		}
		id, _ := e["beaconId"].(string)
		if !strings.HasPrefix(id, "airfield") {
			continue
		}
		b := beaconEntry{id: id}
		b.name, _ = e["display_name"].(string)
		b.kind, _ = e["type"].(string)
		b.callsign, _ = e["callsign"].(string)
		b.hz, _ = e["frequency"].(float64) // 0 when the beacon has no frequency
		if ch, ok := e["channel"].(float64); ok {
			b.channel = int(ch)
		}
		if geo, ok := e["positionGeo"].(map[string]any); ok {
			b.lat, _ = geo["latitude"].(float64)
			b.lng, _ = geo["longitude"].(float64)
		}
		if pos, ok := e["position"].(map[string]any); ok {
			b.alt, _ = pos["2"].(float64)
		}
		out = append(out, b)
	}
	return out, nil
}

// loadTowns parses map/towns.lua: a map of settlements with real coordinates.
func loadTowns(path string) ([]Town, error) {
	root, err := parseLuaFile(path)
	if err != nil {
		return nil, err
	}
	table, _ := root["towns"].(map[string]any)
	if table == nil {
		return nil, nil
	}

	out := make([]Town, 0, len(table))
	// Sort the keys so the result is deterministic (map order is random).
	for _, key := range sortedKeys(table) {
		v := table[key]
		e, ok := v.(map[string]any)
		if !ok {
			continue
		}
		lat, okLat := e["latitude"].(float64)
		lng, okLng := e["longitude"].(float64)
		if !okLat || !okLng {
			continue
		}
		name, _ := e["display_name"].(string)
		if name == "" {
			name = key
		}
		out = append(out, Town{Name: name, Lat: lat, Lng: lng})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// mergeAirfields joins the radio and beacon data on the airfield number.
func mergeAirfields(radio map[string]radioEntry, beacons []beaconEntry, theatre string) ([]Aerodrome, []string) {
	// Group beacons by airfield number, so both files can be matched even when
	// their identifiers differ in suffix.
	byAirfield := map[int][]beaconEntry{}
	for _, b := range beacons {
		n := airfieldNumber(b.id)
		if n < 0 {
			continue
		}
		byAirfield[n] = append(byAirfield[n], b)
	}

	// The union of airfield numbers seen in either file.
	numbers := map[int]bool{}
	for _, r := range radio {
		if r.airfieldN >= 0 {
			numbers[r.airfieldN] = true
		}
	}
	for n := range byAirfield {
		numbers[n] = true
	}

	ordered := make([]int, 0, len(numbers))
	for n := range numbers {
		ordered = append(ordered, n)
	}
	sort.Ints(ordered)

	// dropped collects the aids rejected for an out-of-band frequency.
	var dropped []string
	reject := func(b beaconEntry, kind string, mhz float64) {
		dropped = append(dropped, fmt.Sprintf("%s/%s %s %.3f outside the %s band",
			theatre, b.name, b.callsign, mhz, kind))
	}

	out := make([]Aerodrome, 0, len(ordered))
	for _, n := range ordered {
		a := Aerodrome{
			Theatre: theatre,
			Source:  SourceDCS,
		}

		// Name and ATC frequency from radio.lua.
		if r, ok := radioByNumber(radio, n); ok {
			a.Name = r.callsign
			a.ICAOCode = r.icao
			if r.hasTower {
				a.Tower = roundMHz(r.towerHz)
			}
		}

		// Aids and position from beacons.lua.
		list := byAirfield[n]
		a.ID = airfieldID(list, a.ICAOCode, theatre, n)
		a.Runway = runwayFromBeacons(list)

		// The airfield centre is the best position available: an ILS localizer
		// sits at the runway threshold and a TACAN near the field, while some
		// beacons (outer markers) are kilometres away.
		if lat, lng, alt, ok := airfieldPosition(list); ok {
			a.Lat, a.Lng = lat, lng
			a.ElevationM = alt
		}

		for _, b := range list {
			switch b.kind {
			case beaconTACAN:
				a.TACAN = formatTACAN(b.callsign, b.channel)
			case beaconVOR, beaconVORDME, beaconVORTAC:
				mhz := roundMHz(b.hz)
				if !hasFrequency(b.hz) || inVORband(mhz) {
					a.VOR = formatTACAN(b.callsign, b.channel)
					if a.VORMHz == 0 {
						a.VORMHz = mhz
					}
				} else {
					reject(b, "VOR", mhz)
				}
				// A VORTAC also carries a TACAN, so the pilot gets both.
				if b.kind == beaconVORTAC && b.channel > 0 && a.TACAN == "" {
					a.TACAN = formatTACAN(b.callsign, b.channel)
				}
			case beaconRSBN:
				a.RSBN = b.callsign
			case beaconILS:
				// The localizer frequency is the ILS frequency.
				if mhz := roundMHz(b.hz); inILSband(mhz) {
					a.ILS = append(a.ILS, ILS{Runway: a.Runway, MHz: mhz})
				} else {
					reject(b, "ILS", mhz)
				}
			case beaconPRMG:
				if mhz := roundMHz(b.hz); inILSband(mhz) {
					a.PRMG = append(a.PRMG, ILS{Runway: a.Runway, MHz: mhz})
				} else {
					reject(b, "PRMG", mhz)
				}
			case beaconNDB, beaconNDBMark, beaconHomer, beaconILSFar, beaconILSNear:
				khz := roundKHz(b.hz)
				if hasFrequency(b.hz) && inNDBband(khz) {
					a.NDB = append(a.NDB, NDB{Name: b.callsign, KHz: khz})
				} else {
					reject(b, "NDB", khz/1000)
				}
			}
		}

		// An airfield with neither name nor position is not usable.
		if a.Name == "" && a.Lat == 0 && a.Lng == 0 {
			continue
		}
		if a.Name == "" {
			a.Name = fmt.Sprintf("Airfield %d", n)
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, dropped
}

// airfieldPosition picks the best representative position for an airfield, with
// its altitude in metres.
func airfieldPosition(beacons []beaconEntry) (lat, lng, alt float64, ok bool) {
	// Priority: a localizer (runway), then TACAN, then VOR, then anything.
	for _, want := range []string{beaconILS, beaconPRMG, beaconTACAN, beaconVORDME, beaconVOR, beaconRSBN} {
		for _, b := range beacons {
			if b.kind == want && (b.lat != 0 || b.lng != 0) {
				return b.lat, b.lng, b.alt, true
			}
		}
	}
	for _, b := range beacons {
		if b.lat != 0 || b.lng != 0 {
			return b.lat, b.lng, b.alt, true
		}
	}
	return 0, 0, 0, false
}

// runwayFromBeacons derives a runway label when DCS provides one. ILS localizer
// entries carry the magnetic direction in their callsign only sometimes, so this
// stays conservative: it only fills the field when a designator is recognisable.
func runwayFromBeacons(beacons []beaconEntry) string {
	for _, b := range beacons {
		name := strings.ToUpper(strings.TrimSpace(b.callsign))
		// Two or three characters that are digits (e.g. "ILU" is not a runway).
		if len(name) >= 2 && len(name) <= 3 && allDigits(name) {
			return name
		}
	}
	return ""
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// airfieldID builds a stable identifier. DCS beacons use airfield numbers
// (12 = Anapa), which are stable per terrain; the ICAO code is preferred when
// the radio file provides one.
func airfieldID(beacons []beaconEntry, icao, theatre string, n int) string {
	if icao != "" {
		return strings.ToUpper(icao)
	}
	_ = beacons
	_ = theatre
	return fmt.Sprintf("A%d", n)
}

// airfieldNumber extracts the N from "airfield12_0". Returns -1 when absent.
func airfieldNumber(id string) int {
	rest, ok := strings.CutPrefix(id, "airfield")
	if !ok {
		return -1
	}
	// Stop at the first non-digit.
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return -1
	}
	n := 0
	for _, c := range rest[:end] {
		n = n*10 + int(c-'0')
	}
	return n
}

func radioByNumber(radio map[string]radioEntry, n int) (radioEntry, bool) {
	for _, r := range radio {
		if r.airfieldN == n {
			return r, true
		}
	}
	return radioEntry{}, false
}

// formatTACAN renders a beacon as DCS does in the F10 view: "16X BTM".
func formatTACAN(callsign string, channel int) string {
	if callsign == "" {
		return ""
	}
	if channel <= 0 {
		return callsign
	}
	// TACAN channels above 126 use the Y band in DCS.
	band := "X"
	if channel > 126 {
		band = "Y"
	}
	return fmt.Sprintf("%d%s %s", channel, band, callsign)
}

func roundMHz(hz float64) float64 {
	if !hasFrequency(hz) {
		return 0
	}
	// Frequencies are stored in Hz; MHz to three decimals, as DCS displays them.
	return float64(int(hz/1000+0.5)) / 1000
}

// hasFrequency reports whether a beacon declares a usable frequency. Some
// entries have none (a TACAN is tuned by channel, not frequency), and some
// declare 0, which must not be read as a real value.
func hasFrequency(hz float64) bool { return hz > 0 }

func roundKHz(hz float64) float64 {
	if !hasFrequency(hz) {
		return 0
	}
	return float64(int(hz/100+0.5)) / 10
}

// parseLuaFile reads a DCS data file and parses it, skipping the statements that
// precede the data table (dofile, local, comments), which are not data.
func parseLuaFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return lua.Parse(stripLuaPreamble(raw))
}

func stripLuaPreamble(raw []byte) []byte {
	var out []byte
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "dofile("),
			strings.HasPrefix(trimmed, "local "),
			strings.HasPrefix(trimmed, "require("),
			strings.HasPrefix(trimmed, "--"):
			continue
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
