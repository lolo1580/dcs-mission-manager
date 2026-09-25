package category

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	c := New("")
	cases := map[string]string{
		"F-16C_50":          Plane,
		"FA-18C_hornet":     Plane,
		"Su-27":             Plane,
		"MiG-29S":           Plane,
		"A-10C":             Plane,
		"Mi-24P":            Heli,
		"Ka-50":             Heli,
		"AH-64D_BLK_II":     Heli,
		"UH-1H":             Heli,
		"T-72B":             Ground,
		"BTR-80":            Ground,
		"SA-10":             Ground,
		"ZU-23":             Ground,
		"Ural-375":          Ground,
		"USS_Arleigh_Burke": Ship,
	}
	for typeID, want := range cases {
		if got := c.Classify(typeID); got != want {
			t.Errorf("Classify(%q) = %q, want %q", typeID, got, want)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	c := New("")
	if got := c.Classify(""); got != Other {
		t.Fatalf("Classify(\"\") = %q, want %q", got, Other)
	}
	if got := c.Classify("Xyzzy-9000"); got != Other {
		t.Fatalf("unknown type should be %q, got %q", Other, got)
	}
}

func TestOverridesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "categories.json")
	if err := os.WriteFile(path, []byte(`{"F-16C_50":"heli"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(path)
	if got := c.Classify("F-16C_50"); got != Heli {
		t.Fatalf("override should win, got %q", got)
	}
	// Non-overridden types keep the heuristic.
	if got := c.Classify("Mi-24P"); got != Heli {
		t.Fatalf("heuristic should still apply, got %q", got)
	}
}

func TestMissingOverrideFileIsIgnored(t *testing.T) {
	c := New("does-not-exist.json")
	if got := c.Classify("F-16C_50"); got != Plane {
		t.Fatalf("missing file should fall back to heuristics, got %q", got)
	}
}
