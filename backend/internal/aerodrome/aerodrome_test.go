package aerodrome

import "testing"

func TestLoadCatalog(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Count() == 0 {
		t.Fatal("expected at least one airfield")
	}
}

func TestKnownAirfield(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	batumi, ok := c.ByID("UGSB")
	if !ok {
		t.Fatal("Batumi (UGSB) should be present")
	}
	if batumi.Name != "Batumi" || batumi.Theatre != "Caucasus" {
		t.Fatalf("unexpected airfield: %+v", batumi)
	}
	if batumi.Tower != 131.0 {
		t.Fatalf("Batumi tower = %v, want 131.0", batumi.Tower)
	}
	if batumi.TACAN != "16X BTM" {
		t.Fatalf("Batumi TACAN = %q", batumi.TACAN)
	}
	if len(batumi.ILS) != 1 || batumi.ILS[0].MHz != 110.3 {
		t.Fatalf("Batumi ILS = %+v", batumi.ILS)
	}
	if batumi.Lat == 0 || batumi.Lng == 0 {
		t.Fatal("coordinates should be set")
	}
}

func TestEveryAirfieldHasMinimumData(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, th := range c.Theatres() {
		for _, a := range c.ByTheatre(th) {
			if a.ID == "" || a.Name == "" {
				t.Errorf("airfield missing id/name: %+v", a)
			}
			if a.Theatre == "" {
				t.Errorf("%s: theatre missing", a.ID)
			}
			if a.Lat == 0 || a.Lng == 0 {
				t.Errorf("%s: coordinates missing", a.ID)
			}
			if a.Tower == 0 {
				t.Errorf("%s: tower frequency missing", a.ID)
			}
		}
	}
}

func TestByTheatreSortedAndFiltered(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	caucasus := c.ByTheatre("Caucasus")
	if len(caucasus) < 15 {
		t.Fatalf("expected the Caucasus dataset, got %d", len(caucasus))
	}
	for i := 1; i < len(caucasus); i++ {
		if caucasus[i-1].Name > caucasus[i].Name {
			t.Fatalf("airfields not sorted by name at %d", i)
		}
	}
	if len(c.ByTheatre("Nowhere")) != 0 {
		t.Fatal("an unknown theatre should yield nothing")
	}
}

func TestTheatres(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, th := range c.Theatres() {
		if th == "Caucasus" {
			found = true
		}
	}
	if !found {
		t.Fatal("Caucasus should be listed among theatres")
	}
}
