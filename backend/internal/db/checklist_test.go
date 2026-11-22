package db

import (
	"database/sql"
	"errors"
	"testing"

	"dcsmanager/internal/checklist"
)

func checklistFixture() checklist.Checklist {
	c := checklist.Checklist{
		Name:     "Before start",
		Aircraft: "FA-18C_hornet",
		Phase:    "prestart",
		Items: []checklist.Item{
			{Text: "Battery on", Response: "ON"},
			{Text: "Oxygen", Note: "Check quantity"},
		},
	}
	if err := checklist.Normalize(&c, true); err != nil {
		panic(err)
	}
	return c
}

func TestChecklistCRUD(t *testing.T) {
	d := openTemp(t)
	c := checklistFixture()

	saved, err := d.SaveChecklist(c)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.UID == "" || saved.Revision != 1 || saved.CreatedAt == 0 {
		t.Fatalf("not stamped: %+v", saved)
	}
	if saved.Items[0].Response != "ON" || len(saved.Items) != 2 {
		t.Fatalf("items = %+v", saved.Items)
	}

	loaded, err := d.Checklist(saved.UID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != c.Name || loaded.Aircraft != c.Aircraft || loaded.Phase != c.Phase {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded.Items[1].Note != "Check quantity" {
		t.Fatalf("note lost: %+v", loaded.Items)
	}

	list, err := d.Checklists(10)
	if err != nil || len(list) != 1 || list[0].UID != saved.UID {
		t.Fatalf("list = %+v, %v", list, err)
	}

	loaded.Name = "Renamed"
	updated, err := d.SaveChecklist(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision = %d, want 2", updated.Revision)
	}
	if updated.ID != saved.ID || updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("identity changed: %+v", updated)
	}

	if err := d.DeleteChecklist(saved.UID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Checklist(saved.UID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("after delete: %v", err)
	}
	if err := d.DeleteChecklist(saved.UID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestChecklistImportKeepsExplicitUID(t *testing.T) {
	d := openTemp(t)
	c := checklistFixture()
	c.UID = "cl_imported"
	saved, err := d.SaveChecklist(c)
	if err != nil {
		t.Fatal(err)
	}
	if saved.UID != "cl_imported" {
		t.Fatalf("uid = %q", saved.UID)
	}
}
