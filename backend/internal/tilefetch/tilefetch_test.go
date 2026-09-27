package tilefetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestFetchFlipsTMSRows is the load-bearing test: the source addresses rows from
// the bottom, the files are stored top-down. The server answers ONLY for the
// flipped row, so a wrong flip fetches nothing.
func TestFetchFlipsTMSRows(t *testing.T) {
	// Work out the single z1 tile these bounds cover, then the row the server
	// should be asked for (1 - XYZ row, since z1 has two rows).
	x0, y0 := deg2num(1, 0, 1)
	wantX, wantXYZY := int(x0), int(y0)
	wantTMSRow := 1 - wantXYZY

	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, fmt.Sprintf("/1/%d/%d.png", wantX, wantTMSRow)) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nrest of the file"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := Fetch(Options{
		URLTemplate: srv.URL + "/{z}/{x}/{y}.png",
		TMS:         true,
		Bounds:      Bounds{MinLat: 0, MinLng: 0, MaxLat: 1, MaxLng: 1},
		MinZoom:     1, MaxZoom: 1,
		OutDir: dir, Theatre: "Testland",
		Concurrency: 2, Delay: 0,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if st.Fetched == 0 {
		t.Fatalf("nothing fetched; asked for %v (wanted the TMS row %d)", asked, wantTMSRow)
	}
	// The tile must be stored under the XYZ row, not the source row.
	dest := filepath.Join(dir, "Testland", "1", fmt.Sprint(wantX), fmt.Sprint(wantXYZY)+".png")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected the tile at XYZ row %d: %v", wantXYZY, err)
	}
}

// TestFetchWithoutTMSSkipsExisting covers the non-TMS path and resuming.
func TestFetchWithoutTMSSkipsExisting(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nrest of the file"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	opt := Options{
		URLTemplate: srv.URL + "/{z}/{x}/{y}.png",
		Bounds:      Bounds{MinLat: 0, MinLng: 0, MaxLat: 1, MaxLng: 1},
		MinZoom:     1, MaxZoom: 1,
		OutDir: dir, Theatre: "Testland",
		Concurrency: 2, Delay: 0,
	}
	if _, err := Fetch(opt); err != nil {
		t.Fatal(err)
	}
	first := atomic.LoadInt32(&hits)
	if first == 0 {
		t.Fatal("nothing downloaded")
	}
	// The second run must not touch the server again.
	if _, err := Fetch(opt); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&hits) != first {
		t.Errorf("resume requested %d more tiles, want 0", atomic.LoadInt32(&hits)-first)
	}
}

// TestFetchRejectsBadOptions documents the guard rails.
func TestFetchRejectsBadOptions(t *testing.T) {
	cases := []Options{
		{Theatre: "X", Bounds: Bounds{0, 0, 1, 1}, MinZoom: 0, MaxZoom: 1},                              // no URL
		{URLTemplate: "http://x/{z}", Bounds: Bounds{0, 0, 1, 1}, MinZoom: 0, MaxZoom: 1},               // no theatre
		{URLTemplate: "http://x/{z}", Theatre: "X", Bounds: Bounds{5, 5, 1, 1}, MinZoom: 0, MaxZoom: 1}, // bad bounds
		{URLTemplate: "http://x/{z}", Theatre: "X", Bounds: Bounds{0, 0, 1, 1}, MinZoom: 5, MaxZoom: 2}, // bad zoom
	}
	for i, c := range cases {
		if _, err := Fetch(c); err == nil {
			t.Errorf("case %d should have been rejected", i)
		}
	}
}

// TestParseBounds covers the CLI input.
func TestParseBounds(t *testing.T) {
	b, err := ParseBounds("40.8,36.5,45.8,45.5")
	if err != nil {
		t.Fatal(err)
	}
	if b.MinLat != 40.8 || b.MaxLng != 45.5 {
		t.Errorf("bounds = %+v", b)
	}
	if _, err := ParseBounds("nonsense"); err == nil {
		t.Error("nonsense should be rejected")
	}
}

// TestFetchHandlesMissingTiles checks that a 404 (normal at the edges of a set)
// counts as a failure but does not abort the run. The bounds span several z1
// tiles, and only one of them exists upstream.
func TestFetchHandlesMissingTiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Answer for the western column only; the rest 404.
		if strings.Contains(r.URL.Path, "/1/0/") {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nok"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := Fetch(Options{
		URLTemplate: srv.URL + "/{z}/{x}/{y}.png",
		Bounds:      Bounds{MinLat: -10, MinLng: -180, MaxLat: 10, MaxLng: 180},
		MinZoom:     1, MaxZoom: 1,
		OutDir: dir, Theatre: "Testland",
		Concurrency: 1, Delay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("a missing tile must not abort the run: %v", err)
	}
	if st.Failed == 0 {
		t.Errorf("stats = %+v, expected some failures", st)
	}
	if st.Fetched == 0 {
		t.Errorf("stats = %+v, expected the present tile to be fetched", st)
	}
}
