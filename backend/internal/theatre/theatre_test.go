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
