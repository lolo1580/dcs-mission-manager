package basemap

import "testing"

func TestAllIncludesBuiltins(t *testing.T) {
	all := All("")
	if len(all) != 2 {
		t.Fatalf("want exactly 2 built-in basemaps, got %d", len(all))
	}
	for _, id := range []string{"aero", "dark"} {
		if _, ok := Get(id); !ok {
			t.Errorf("built-in %q should exist", id)
		}
	}
	// The real-world basemaps were removed on purpose: they say nothing about
	// the simulator. Keep them from creeping back in.
	for _, id := range []string{"satellite", "topo", "osm"} {
		if _, ok := Get(id); ok {
			t.Errorf("%q should no longer be a built-in basemap", id)
		}
	}
}

// TestDefaultExists checks the configured default is actually offered.
func TestDefaultExists(t *testing.T) {
	if _, ok := Get(DefaultID); !ok {
		t.Fatalf("DefaultID %q is not a built-in basemap", DefaultID)
	}
}

func TestAllAppendsCustom(t *testing.T) {
	all := All("https://example.com/{z}/{x}/{y}.png")
	last := all[len(all)-1]
	if last.ID != "custom" || last.URL == "" {
		t.Fatalf("custom basemap not appended: %+v", last)
	}
}

func TestEveryBasemapHasURLAndAttribution(t *testing.T) {
	for _, b := range All("") {
		if b.URL == "" || b.Name == "" || b.Attribution == "" || b.MaxZoom <= 0 {
			t.Errorf("incomplete basemap: %+v", b)
		}
	}
}
