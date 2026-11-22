package checklist

import (
	"strings"
	"testing"
)

func TestNormalizeAssignsIdentifiersAndDefaults(t *testing.T) {
	c := Checklist{
		Name:  "  Before start  ",
		Phase: "prestart",
		Items: []Item{
			{Text: "  Battery  "},
			{Text: "APU", UID: "it_keep"},
		},
	}
	if err := Normalize(&c, true); err != nil {
		t.Fatal(err)
	}
	if c.Name != "Before start" {
		t.Fatalf("name = %q", c.Name)
	}
	if c.UID == "" || !strings.HasPrefix(c.UID, "cl_") {
		t.Fatalf("uid = %q", c.UID)
	}
	if c.Source != "user" || c.Visibility != "private" {
		t.Fatalf("defaults = %q/%q", c.Source, c.Visibility)
	}
	if c.Items[0].Text != "Battery" || c.Items[0].UID == "" {
		t.Fatalf("item = %+v", c.Items[0])
	}
	if c.Items[1].UID != "it_keep" {
		t.Fatalf("existing item uid changed: %q", c.Items[1].UID)
	}
	if c.Items[0].UID == c.Items[1].UID {
		t.Fatal("two items share an identifier")
	}
}

func TestNormalizeRejectsInvalidChecklists(t *testing.T) {
	cases := []struct {
		name string
		c    Checklist
	}{
		{"no name", Checklist{Phase: "prestart", Items: []Item{{Text: "x"}}}},
		{"no items", Checklist{Name: "Empty", Phase: "prestart"}},
		{"unknown phase", Checklist{Name: "Phase", Phase: "aerobatics", Items: []Item{{Text: "x"}}}},
		{"blank item", Checklist{Name: "Blank", Phase: "prestart", Items: []Item{{Text: "  "}}}},
		{"bad visibility", Checklist{Name: "Visibility", Phase: "prestart", Visibility: "public", Items: []Item{{Text: "x"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Normalize(&tc.c, true); err == nil {
				t.Fatalf("accepted %+v", tc.c)
			}
		})
	}
}

func TestNormalizeRepairsDuplicateItemUIDs(t *testing.T) {
	c := Checklist{
		Name:  "Duplicates",
		Phase: "taxi",
		Items: []Item{{UID: "it_same", Text: "one"}, {UID: "it_same", Text: "two"}},
	}
	if err := Normalize(&c, true); err != nil {
		t.Fatal(err)
	}
	if c.Items[0].UID == c.Items[1].UID {
		t.Fatal("duplicate identifier kept")
	}
}
