// Package charts indexes the aeronautical charts kept on disk.
//
// The charts are scans (approach plates, ground movement plans, procedure
// charts), not georeferenced rasters: they cannot be overlaid on the map. They
// are documents, shown as documents.
//
// They are never shipped with the binary and never redistributed: the folder is
// local, user-supplied, and each scan remains subject to its own licence. This
// package only reads the file names to build an index, and resolves a name back
// to a file so the UI can display it.
package charts

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Chart is one chart file on disk.
type Chart struct {
	// Name is the bare file name, shown in the UI.
	Name string `json:"name"`
	// Path is the location relative to the charts folder, using forward slashes.
	// It is the identifier used to serve the file, and it is what makes a file in
	// a subfolder reachable.
	Path string `json:"path"`
	// Theatre is the DCS theatre the chart belongs to, when it could be told
	// from the folder it sits in.
	Theatre string `json:"theatre,omitempty"`
	// Kind is the chart family: "approach" (VAD), "ground" (GND), "procedure"
	// (PAR), "instrument" (ILS/RNP/LOC), "general", or "other".
	Kind string `json:"kind"`
	// Runway is the designator found in the file name, when present.
	Runway string `json:"runway,omitempty"`
	// Size is the file size in bytes, to avoid opening a huge scan by accident.
	Size int64 `json:"size,omitempty"`
}

// Catalog is the index of chart files.
type Catalog struct {
	byPath    map[string]Chart
	byTheatre map[string][]Chart
	// unassigned holds charts whose theatre could not be inferred from the folder
	// name. They must still be returned by a listing without a filter.
	unassigned []Chart
	// bySearch maps a normalised keyword to the charts whose file name contains
	// it, which is how charts are matched to an airfield.
	bySearch map[string][]Chart
}

// imageExtensions are the file types treated as charts.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
}

// theatreHints maps a lowercase fragment of a folder name to a DCS theatre id.
// The folder names in a chart library are free-form ("DCS Caucasus Maps",
// "Marianas_High_Detail_Maps"), so the theatre has to be inferred.
//
// The ids must match internal/theatre's own: "MarianaIslandsWWII" and "SinaiMap"
// are DCS's spellings, and using "MarianasWWII" or "Sinai" here made a chart
// unreachable by theatre and advertised an id the theatre selector does not know.
var theatreHints = []struct {
	fragment string
	theatre  string
}{
	{"caucasus", "Caucasus"},
	{"kola", "Kola"},
	{"nevada", "Nevada"},
	{"normandy", "Normandy"},
	{"persiangulf", "PersianGulf"},
	{"syria", "Syria"},
	{"channel", "TheChannel"},
	{"marianaswwii", "MarianaIslandsWWII"},
	{"wwii", "MarianaIslandsWWII"},
	{"marianas", "MarianaIslands"},
	{"mariana", "MarianaIslands"},
	{"falklands", "Falklands"},
	{"sinai", "SinaiMap"},
	{"afghanistan", "Afghanistan"},
	{"iraq", "Iraq"},
	{"germany", "GermanyCW"},
}

// Load indexes every chart under dir. A missing directory yields an empty
// catalogue rather than an error: the feature is optional.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{
		byPath:    make(map[string]Chart),
		byTheatre: make(map[string][]Chart),
		bySearch:  make(map[string][]Chart),
	}
	if dir == "" {
		return c, nil
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return c, nil
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// A single unreadable entry must not abort the whole scan.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !imageExtensions[ext] {
			return nil
		}

		info, statErr := d.Info()
		var size int64
		if statErr == nil {
			size = info.Size()
		}

		rel, _ := filepath.Rel(dir, path)
		relSlash := filepath.ToSlash(rel)
		ch := Chart{
			Name:    d.Name(),
			Path:    relSlash,
			Theatre: inferTheatre(relSlash),
			Kind:    inferKind(d.Name()),
			Runway:  inferRunway(d.Name()),
			Size:    size,
		}

		if _, exists := c.byPath[ch.Path]; exists {
			return nil
		}
		c.byPath[ch.Path] = ch
		if ch.Theatre != "" {
			c.byTheatre[ch.Theatre] = append(c.byTheatre[ch.Theatre], ch)
		} else {
			// No theatre could be inferred. The chart is still indexed and
			// servable, so it must appear in the full listing too; keeping it in
			// its own bucket is what makes that possible.
			c.unassigned = append(c.unassigned, ch)
		}
		for _, key := range searchKeys(d.Name()) {
			c.bySearch[key] = append(c.bySearch[key], ch)
		}
		return nil
	})
	if err != nil {
		return c, err
	}

	for th := range c.byTheatre {
		sortCharts(c.byTheatre[th])
	}
	return c, nil
}

// Get resolves a relative path to its chart. Only indexed files resolve, which
// is what keeps the HTTP handler from serving anything else.
func (c *Catalog) Get(path string) (Chart, bool) {
	ch, ok := c.byPath[path]
	return ch, ok
}

// Count returns the number of indexed charts.
func (c *Catalog) Count() int { return len(c.byPath) }

// ByTheatre returns the charts of a theatre, sorted.
func (c *Catalog) ByTheatre(theatre string) []Chart {
	out := make([]Chart, len(c.byTheatre[theatre]))
	copy(out, c.byTheatre[theatre])
	return out
}

// Theatres returns the theatre ids that have charts.
func (c *Catalog) Theatres() []string {
	out := make([]string, 0, len(c.byTheatre))
	for th := range c.byTheatre {
		out = append(out, th)
	}
	sort.Strings(out)
	return out
}

// Unassigned returns the charts whose theatre could not be inferred, so a full
// listing can include them.
func (c *Catalog) Unassigned() []Chart {
	out := make([]Chart, len(c.unassigned))
	copy(out, c.unassigned)
	return out
}

// ForAerodrome returns the charts belonging to one airfield, matched by name:
// the scans are named after the field ("01_VAD_UG5X_Kobulei.png",
// "NORWAY_LAKSELV-ILS-RWY34.jpg"), so a name match is what identifies them.
//
// icao and name are both tried; either may be empty.
func (c *Catalog) ForAerodrome(icao, name string) []Chart {
	seen := map[string]bool{}
	var out []Chart

	add := func(list []Chart) {
		for _, ch := range list {
			if !seen[ch.Path] {
				seen[ch.Path] = true
				out = append(out, ch)
			}
		}
	}
	if icao != "" {
		add(c.bySearch[normalise(icao)])
	}
	if name != "" {
		add(c.bySearch[normalise(name)])
		// Long names are often shortened in file names ("Maykop Khanskaya" ->
		// "Maykop"), so also try the longest word of the name.
		if word := longestWord(name); word != "" {
			add(c.bySearch[normalise(word)])
		}
	}

	// Exact lists already curated for the Caucasus take precedence in ordering,
	// so keep results sorted by kind then name.
	sortCharts(out)
	return out
}

// ResolveForAerodrome keeps only the charts that actually exist, from a list of
// names the dataset records. It lets the UI stop advertising charts that are not
// on disk.
func (c *Catalog) ResolveForAerodrome(names []string) []Chart {
	out := make([]Chart, 0, len(names))
	for _, n := range names {
		// The curated dataset stores bare names; match them against the index.
		for _, ch := range c.byPath {
			if ch.Name == n {
				out = append(out, ch)
				break
			}
		}
	}
	return out
}

// inferTheatre finds the theatre from the relative path of a chart.
func inferTheatre(rel string) string {
	lower := strings.ToLower(rel)
	for _, hint := range theatreHints {
		if strings.Contains(lower, hint.fragment) {
			return hint.theatre
		}
	}
	return ""
}

// inferKind classifies a chart from its file name.
func inferKind(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.Contains(upper, "_VAD_"):
		return "approach"
	case strings.Contains(upper, "_GND_"):
		return "ground"
	case strings.Contains(upper, "_PAR_"):
		return "procedure"
	case strings.Contains(upper, "ILS"), strings.Contains(upper, "RNP"),
		strings.Contains(upper, "LOC-"), strings.Contains(upper, "LOC_"):
		return "instrument"
	case strings.HasPrefix(upper, "00_"), strings.Contains(upper, "LEGEND"),
		strings.Contains(upper, "GENERAL"):
		return "general"
	default:
		return "other"
	}
}

// inferRunway extracts a runway designator such as "18" or "16" from a file
// name ("..._Kobuleti_18.png", "...RWY16.jpg").
//
// The scans also start with a sequence number ("01_GND_UG5X_Kobuleti_18"), so
// only the *last* numeric segment counts: taking the first would return "01",
// which is a filing number, not a runway.
func inferRunway(name string) string {
	upper := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))

	// "...RWY16" / "...RWY-16"
	if i := strings.Index(upper, "RWY"); i >= 0 {
		rest := strings.TrimLeft(upper[i+3:], "-_ ")
		if n := leadingDigits(rest); n != "" {
			return n
		}
	}

	parts := strings.Split(upper, "_")
	// Drop trailing page markers ("_P1", "_P2"), which follow the runway.
	for len(parts) > 1 && isPageMarker(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if len(last) >= 2 && len(last) <= 3 && allDigits(last) {
		return last
	}
	return ""
}

// isPageMarker reports whether a segment is a page suffix such as "P1" or "P2".
func isPageMarker(s string) bool {
	if len(s) < 2 || len(s) > 3 || s[0] != 'P' {
		return false
	}
	return allDigits(s[1:])
}

// searchKeys returns the normalised keywords a file name can be matched by:
// every alphanumeric word of at least four characters, plus a few codes.
func searchKeys(name string) []string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	var keys []string
	seen := map[string]bool{}

	// A two-character airfield code is real ("H4", "H3"), so the threshold is
	// two rather than three; four is the common case (ICAO).
	addKey := func(k string) {
		if len(k) < 2 || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}

	// Split on every non-alphanumeric character.
	fields := strings.FieldsFunc(base, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	for _, f := range fields {
		n := normalise(f)
		// Numeric-only fields are runway or sequence numbers, never airfield
		// codes, so they are not match keys.
		if len(n) >= 2 && !allDigits(n) {
			addKey(n)
		}
	}
	return keys
}

func sortCharts(list []Chart) {
	kindRank := map[string]int{
		"approach": 0, "instrument": 1, "procedure": 2,
		"ground": 3, "general": 4, "other": 5,
	}
	sort.Slice(list, func(i, j int) bool {
		ri, rj := kindRank[list[i].Kind], kindRank[list[j].Kind]
		if ri != rj {
			return ri < rj
		}
		return list[i].Name < list[j].Name
	})
}

// normalise lowercases and drops non-alphanumeric characters, so "Anapa" and
// "ANAPA-Vityazevo" can be compared.
func normalise(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// longestWord returns the longest alphanumeric word of a name.
func longestWord(s string) string {
	best := ""
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if len(f) > len(best) {
			best = f
		}
	}
	return best
}

func leadingDigits(s string) string {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return s[:end]
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
