package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dcsmm/internal/state"
)

func sample() []state.Unit {
	return []state.Unit{
		{ID: "1", Type: "F-16C_50", Category: "plane", Coalition: "blue", Ownship: true, Label: "Player"},
		{ID: "2", Type: "T-72B", Category: "ground", Coalition: "red"},
		{ID: "3", Type: "USS_Arleigh_Burke", Category: "ship", Coalition: "blue", Country: "USA"},
	}
}

func TestFilterNoParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state", nil)
	if got := filter(sample(), r); len(got) != 3 {
		t.Fatalf("want 3 units, got %d", len(got))
	}
}

func TestFilterByCategory(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?category=ground", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestFilterByCoalition(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?coalition=blue", nil)
	if got := filter(sample(), r); len(got) != 2 {
		t.Fatalf("want 2 blue units, got %d", len(got))
	}
}

func TestFilterOwnship(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?ownship=true", nil)
	got := filter(sample(), r)
	if len(got) != 1 || !got[0].Ownship {
		t.Fatalf("want only ownship, got %+v", got)
	}
}

func TestFilterSearch(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?q=burke", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("search should match type, got %+v", got)
	}
}

func TestFilterSearchCaseInsensitive(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/state?q=PLAYER", nil)
	got := filter(sample(), r)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("search should be case-insensitive and match label, got %+v", got)
	}
}

func TestSummarise(t *testing.T) {
	s := summarise(sample())
	if s.ByCategory["plane"] != 1 || s.ByCoalition["blue"] != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	// Missing category/coalition default to other/neutral.
	s2 := summarise([]state.Unit{{ID: "x"}})
	if s2.ByCategory["other"] != 1 || s2.ByCoalition["neutral"] != 1 {
		t.Fatalf("unexpected default summary: %+v", s2)
	}
}

func TestValidTilePart(t *testing.T) {
	for _, ok := range []string{"Caucasus", "3", "12", "17", "abc-1_2"} {
		if !validTilePart(ok) {
			t.Errorf("validTilePart(%q) should be true", ok)
		}
	}
	for _, bad := range []string{"", "../", "a/b", "a.b", "a b", "a\\b"} {
		if validTilePart(bad) {
			t.Errorf("validTilePart(%q) should be false", bad)
		}
	}
}

// TestHandleTilesExtension covers the regression that made every tile request
// fail: Leaflet sends the extension in the URL (".../11.png"), and the handler
// appended ".png" unconditionally, producing "11.png.png". Nothing caught it
// until a tile actually existed on disk.
func TestHandleTilesExtension(t *testing.T) {
	dir := t.TempDir()
	tileDir := filepath.Join(dir, "Caucasus", "5", "19")
	if err := os.MkdirAll(tileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "not really a png, but a file"
	tile := filepath.Join(tileDir, "11.png")
	if err := os.WriteFile(tile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{tilesDir: dir}
	handler := http.HandlerFunc(s.handleTiles)

	cases := []struct {
		path string
		want int
		why  string
	}{
		{"/api/tiles/Caucasus/5/19/11.png", http.StatusOK, "the form Leaflet requests"},
		{"/api/tiles/Caucasus/5/19/11", http.StatusOK, "without extension, also accepted"},
		{"/api/tiles/Caucasus/5/19/12.png", http.StatusNotFound, "no such tile"},
		{"/api/tiles/Caucasus/5/19/11.png.png", http.StatusNotFound, "double extension"},
		{"/api/tiles/Caucasus/5/19/..%2f..%2fgo.mod.png", http.StatusNotFound, "traversal"},
		{"/api/tiles/../5/19/11.png", http.StatusNotFound, "traversal in theatre"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s (%s): got %d, want %d", tc.path, tc.why, rec.Code, tc.want)
		}
	}
}
