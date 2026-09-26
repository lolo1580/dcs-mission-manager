package basemap

import "testing"

func TestAllIncludesBuiltins(t *testing.T) {
	all := All("")
	if len(all) < 4 {
		t.Fatalf("want at least 4 basemaps, got %d", len(all))
	}
	for _, id := range []string{"satellite", "topo", "osm", "dark"} {
		if _, ok := Get(id); !ok {
			t.Errorf("built-in %q should exist", id)
		}
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
