package theatre

import "testing"

func TestGet(t *testing.T) {
	th, ok := Get("Caucasus")
	if !ok {
		t.Fatal("Caucasus theatre should exist")
	}
	if th.Name == "" || th.Bounds.MaxLat <= th.Bounds.MinLat {
		t.Fatalf("invalid theatre: %+v", th)
	}
	if _, ok := Get("Nowhere"); ok {
		t.Fatal("unknown theatre should not be found")
	}
}

// TestDCSIdentifiers locks in the theatre identifiers DCS itself uses. Getting
// one wrong is not cosmetic: airfields are indexed by theatre, so a mismatch
// makes them unreachable even though they were read successfully. "Marianas" and
// "Sinai" were both wrong, which silently hid 5 airfields of the Marianas.
func TestDCSIdentifiers(t *testing.T) {
	// These are the ids DCS declares in each terrain's entry.lua.
	required := []string{
		"Caucasus", "Syria", "Nevada", "PersianGulf",
		"MarianaIslands", "MarianaIslandsWWII", "SinaiMap", "Kola",
		"Afghanistan", "Iraq", "Falklands", "Normandy", "TheChannel",
		"GermanyCW", "SouthEastAsia",
	}
	for _, id := range required {
		if _, ok := Get(id); !ok {
			t.Errorf("theatre %q must exist: it is the id DCS uses", id)
		}
	}

	// The wrong spellings must no longer be theatre ids of their own.
	for _, wrong := range []string{"Marianas", "Sinai"} {
		for _, th := range All() {
			if th.ID == wrong {
				t.Errorf("%q must not be a theatre id; DCS uses the other spelling", wrong)
			}
		}
	}

	// They still resolve, so a preference saved by an earlier release works.
	if _, ok := Get("Marianas"); !ok {
		t.Error("Get(\"Marianas\") should follow the alias")
	}
	if got := Resolve("Sinai"); got != "SinaiMap" {
		t.Errorf("Resolve(\"Sinai\") = %q, want SinaiMap", got)
	}
	if got := Resolve("Caucasus"); got != "Caucasus" {
		t.Errorf("Resolve should leave a real id alone, got %q", got)
	}
	if got := Resolve("Nowhere"); got != "Nowhere" {
		t.Errorf("Resolve should leave an unknown id alone, got %q", got)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	a := All()
	if len(a) == 0 {
		t.Fatal("expected built-in theatres")
	}
	a[0].Name = "mutated"
	if All()[0].Name == "mutated" {
		t.Fatal("All() must return a copy")
	}
}

// TestEveryTheatreHasBounds checks the map framing data is coherent.
func TestEveryTheatreHasBounds(t *testing.T) {
	for _, th := range All() {
		if th.Name == "" {
			t.Errorf("%s: missing name", th.ID)
		}
		b := th.Bounds
		if b.MaxLat <= b.MinLat || b.MaxLng <= b.MinLng {
			t.Errorf("%s: incoherent bounds %+v", th.ID, b)
		}
		if b.MinLat < -90 || b.MaxLat > 90 || b.MinLng < -180 || b.MaxLng > 180 {
			t.Errorf("%s: bounds outside the world %+v", th.ID, b)
		}
	}
}
