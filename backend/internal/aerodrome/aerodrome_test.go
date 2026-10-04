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
	withPosition := 0
	for _, th := range c.Theatres() {
		for _, a := range c.ByTheatre(th) {
			if a.ID == "" || a.Name == "" {
				t.Errorf("airfield missing id/name: %+v", a)
			}
			if a.Theatre == "" {
				t.Errorf("%s: theatre missing", a.ID)
			}
			// Coordinates are not guaranteed: DCS only records a position for an
			// airfield that carries a navigation aid, so a field may have a
			// tower frequency but no beacon and therefore no position. It is
			// still listed, and the UI says it cannot be placed.
			if a.Lat != 0 || a.Lng != 0 {
				withPosition++
			}
			if a.Tower == 0 {
				t.Errorf("%s: tower frequency missing", a.ID)
			}
		}
	}
	if withPosition == 0 {
		t.Error("no airfield has a position; the dataset looks broken")
	}
}

// TestColdWarGermanyAirfields locks in the embedded Cold War Germany dataset.
// DCS gives every field a radio frequency but only positions those that carry a
// beacon, so the dataset must keep both kinds: the placed fields and the ones
// that are listed without coordinates.
func TestColdWarGermanyAirfields(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	list := c.ByTheatre("GermanyCW")
	if len(list) < 100 {
		t.Fatalf("expected the Cold War Germany dataset (100+ airfields), got %d", len(list))
	}

	var frankfurt, adelsheim *Aerodrome
	withPosition := 0
	for i := range list {
		switch list[i].Name {
		case "FRANKFURT":
			frankfurt = &list[i]
		case "Adelsheim":
			adelsheim = &list[i]
		}
		if list[i].Lat != 0 || list[i].Lng != 0 {
			withPosition++
		}
	}

	if frankfurt == nil {
		t.Fatal("Frankfurt should be in the dataset")
	}
	if frankfurt.Tower != 127.3 || frankfurt.TACAN != "89X FFM" {
		t.Errorf("Frankfurt = tower %v, TACAN %q", frankfurt.Tower, frankfurt.TACAN)
	}
	if frankfurt.Lat == 0 || frankfurt.Lng == 0 {
		t.Error("Frankfurt should have coordinates")
	}

	if adelsheim == nil {
		t.Fatal("Adelsheim should be in the dataset")
	}
	if adelsheim.Tower != 118.0 {
		t.Errorf("Adelsheim tower = %v, want 118.0", adelsheim.Tower)
	}

	if withPosition < 70 {
		t.Errorf("only %d Cold War Germany airfields have a position", withPosition)
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
