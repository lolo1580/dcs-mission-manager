package db

import (
	"database/sql"
	"errors"
	"testing"

	"dcsmanager/internal/checklist"
	"dcsmanager/internal/flightplan"
)

func planForSync(uid, name, visibility string, updated int64) flightplan.Plan {
	return flightplan.Plan{
		UID:         uid,
		Name:        name,
		Visibility:  visibility,
		Revision:    1,
		UpdatedAt:   updated,
		FlightRules: "VFR",
		Route: []flightplan.Waypoint{
			{Ident: "LFBO", Kind: "airport", Lat: 43.6, Lon: 1.36},
			{Ident: "LFML", Kind: "airport", Lat: 43.4, Lon: 5.2},
		},
	}
}

func checklistForSync(uid, name, visibility string, updated int64) checklist.Checklist {
	return checklist.Checklist{
		UID:        uid,
		Name:       name,
		Phase:      "prestart",
		Visibility: visibility,
		Revision:   1,
		UpdatedAt:  updated,
		Items:      []checklist.Item{{UID: "it_" + uid, Text: "Battery"}},
	}
}

func TestDocumentVisibilityAndOwnership(t *testing.T) {
	d := openTemp(t)
	one, err := d.SaveFlightPlanFor(1, planForSync("fp_one", "Mine", "private", 1000))
	if err != nil {
		t.Fatal(err)
	}
	two, err := d.SaveFlightPlanFor(2, planForSync("fp_two", "Other", "private", 1000))
	if err != nil {
		t.Fatal(err)
	}
	shared, err := d.SaveFlightPlanFor(1, planForSync("fp_shared", "Shared", "squadron", 1000))
	if err != nil {
		t.Fatal(err)
	}
	if one.OwnerID != 1 || two.OwnerID != 2 || shared.Visibility != "squadron" {
		t.Fatalf("owners/visibility = %+v / %+v / %+v", one, two, shared)
	}

	list, err := d.FlightPlansFor(1, false, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("member list = %+v", list)
	}
	if _, err := d.FlightPlanFor(two.ID, 1, false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("private plan of another member: %v", err)
	}
	if _, err := d.FlightPlanFor(shared.ID, 2, false); err != nil {
		t.Fatalf("squadron plan not visible: %v", err)
	}
	all, err := d.FlightPlansFor(1, true, 20)
	if err != nil || len(all) != 3 {
		t.Fatalf("admin list = %+v, %v", all, err)
	}

	// An administrator editing a member's plan keeps the member as owner.
	one.Name = "Renamed by admin"
	updated, err := d.SaveFlightPlanFor(99, one)
	if err != nil {
		t.Fatal(err)
	}
	if updated.OwnerID != 1 {
		t.Fatalf("owner changed by admin: %+v", updated)
	}

	// Checklists follow the same rules.
	c1, err := d.SaveChecklistFor(1, checklistForSync("cl_one", "Mine", "private", 1000))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveChecklistFor(2, checklistForSync("cl_two", "Other", "private", 1000)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveChecklistFor(1, checklistForSync("cl_shared", "Shared", "squadron", 1000)); err != nil {
		t.Fatal(err)
	}
	cl, err := d.ChecklistsFor(1, false, 20)
	if err != nil || len(cl) != 2 {
		t.Fatalf("checklist member list = %+v, %v", cl, err)
	}
	if _, err := d.ChecklistFor("cl_two", 1, false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("private checklist of another member: %v", err)
	}
	if c1.OwnerID != 1 {
		t.Fatalf("checklist owner = %d", c1.OwnerID)
	}
}

func TestFlightPlanSyncLWWAndTombstone(t *testing.T) {
	d := openTemp(t)
	plan := planForSync("fp_sync", "First", "private", 1000)
	saved, applied, err := d.UpsertFlightPlanFromSync(1, false, plan)
	if err != nil || !applied {
		t.Fatalf("insert = %+v, %v, %v", saved, applied, err)
	}
	if saved.OwnerID != 1 || saved.SyncRevision == 0 {
		t.Fatalf("inserted = %+v", saved)
	}

	plan.Name = "Second"
	plan.Revision = 2
	plan.UpdatedAt = 2000
	saved, applied, err = d.UpsertFlightPlanFromSync(1, false, plan)
	if err != nil || !applied || saved.Name != "Second" {
		t.Fatalf("newer = %+v, %v, %v", saved, applied, err)
	}

	older := plan
	older.Name = "Older"
	older.UpdatedAt = 1500
	saved, applied, err = d.UpsertFlightPlanFromSync(1, false, older)
	if err != nil || applied || saved.Name != "Second" {
		t.Fatalf("older = %+v, %v, %v", saved, applied, err)
	}
	conflicts, err := d.Conflicts(1, false, 10)
	if err != nil || len(conflicts) != 1 || conflicts[0].Kind != "flightplan" {
		t.Fatalf("conflicts = %+v, %v", conflicts, err)
	}

	// Pull returns the winning version and respects visibility.
	pull, err := d.SyncFlightPlans(0, 1, false, 100)
	if err != nil || len(pull) != 1 || pull[0].Name != "Second" {
		t.Fatalf("pull = %+v, %v", pull, err)
	}
	if cursor, err := d.SyncCursor(); err != nil || cursor == 0 {
		t.Fatalf("cursor = %d, %v", cursor, err)
	}

	// A deletion is a tombstone: hidden from the UI but present in the pull.
	if err := d.DeleteFlightPlanFor(saved.ID, 1, false); err != nil {
		t.Fatal(err)
	}
	if list, err := d.FlightPlansFor(1, false, 10); err != nil || len(list) != 0 {
		t.Fatalf("after delete = %+v, %v", list, err)
	}
	pull, err = d.SyncFlightPlans(0, 1, false, 100)
	if err != nil || len(pull) != 1 || pull[0].DeletedAt == 0 {
		t.Fatalf("tombstone pull = %+v, %v", pull, err)
	}

	// Another member cannot overwrite the document.
	other := plan
	other.Name = "Hijack"
	other.UpdatedAt = 3000
	other.Revision = 3
	saved, applied, err = d.UpsertFlightPlanFromSync(2, false, other)
	if err != nil || applied || saved.Name != "Second" {
		t.Fatalf("hijack = %+v, %v, %v", saved, applied, err)
	}
}

func TestChecklistSyncLWW(t *testing.T) {
	d := openTemp(t)
	c := checklistForSync("cl_sync", "First", "private", 1000)
	if _, applied, err := d.UpsertChecklistFromSync(1, false, c); err != nil || !applied {
		t.Fatalf("insert = %v, %v", applied, err)
	}
	c.Name = "Second"
	c.Revision = 2
	c.UpdatedAt = 2000
	saved, applied, err := d.UpsertChecklistFromSync(1, false, c)
	if err != nil || !applied || saved.Name != "Second" {
		t.Fatalf("newer = %+v, %v, %v", saved, applied, err)
	}
	if err := d.DeleteChecklistFor(c.UID, 1, false); err != nil {
		t.Fatal(err)
	}
	pull, err := d.SyncChecklists(0, 1, false, 100)
	if err != nil || len(pull) != 1 || pull[0].DeletedAt == 0 {
		t.Fatalf("tombstone = %+v, %v", pull, err)
	}
}

func TestSyncDirtyOutbox(t *testing.T) {
	d := openTemp(t)
	plan, err := d.SaveFlightPlanFor(0, planForSync("fp_dirty", "Local", "private", 0))
	if err != nil {
		t.Fatal(err)
	}
	pending, err := d.ExportFlightPlansForSync(10)
	if err != nil || len(pending) != 1 || pending[0].UID != plan.UID {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	if err := d.MarkFlightPlansSynced(pending); err != nil {
		t.Fatal(err)
	}
	pending, err = d.ExportFlightPlansForSync(10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("after mark = %+v, %v", pending, err)
	}

	// A remote version is stored clean: pulling it must not send it back.
	remote := plan
	remote.Name = "Remote"
	remote.UpdatedAt = plan.UpdatedAt + 1000
	remote.Revision = plan.Revision + 1
	if applied, err := d.ApplySyncedFlightPlan(remote); err != nil || !applied {
		t.Fatalf("apply = %v, %v", applied, err)
	}
	pending, err = d.ExportFlightPlansForSync(10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("after remote apply = %+v, %v", pending, err)
	}

	// A new local save becomes dirty again.
	if _, err := d.SaveFlightPlanFor(0, remote); err != nil {
		t.Fatal(err)
	}
	pending, err = d.ExportFlightPlansForSync(10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("after local save = %+v, %v", pending, err)
	}

	checklistLocal, err := d.SaveChecklistFor(0, checklistForSync("cl_dirty", "Local", "private", 0))
	if err != nil {
		t.Fatal(err)
	}
	pendingChecklists, err := d.ExportChecklistsForSync(10)
	if err != nil || len(pendingChecklists) != 1 {
		t.Fatalf("checklist pending = %+v, %v", pendingChecklists, err)
	}
	if err := d.MarkChecklistsSynced(pendingChecklists); err != nil {
		t.Fatal(err)
	}
	pendingChecklists, err = d.ExportChecklistsForSync(10)
	if err != nil || len(pendingChecklists) != 0 {
		t.Fatalf("checklist after mark = %+v, %v", pendingChecklists, err)
	}
	if checklistLocal.UID == "" {
		t.Fatal("checklist has no uid")
	}
}

func TestApplySyncedDocuments(t *testing.T) {
	d := openTemp(t)
	local, err := d.SaveFlightPlanFor(0, planForSync("fp_local", "Local", "private", 0))
	if err != nil {
		t.Fatal(err)
	}
	remote := local
	remote.Name = "Remote"
	remote.UpdatedAt = local.UpdatedAt + 1000
	remote.Revision = local.Revision + 1
	applied, err := d.ApplySyncedFlightPlan(remote)
	if err != nil || !applied {
		t.Fatalf("apply = %v, %v", applied, err)
	}
	loaded, err := d.FlightPlanFor(local.ID, 0, false)
	if err != nil || loaded.Name != "Remote" {
		t.Fatalf("loaded = %+v, %v", loaded, err)
	}
	old := remote
	old.Name = "Stale"
	old.UpdatedAt = local.UpdatedAt
	applied, err = d.ApplySyncedFlightPlan(old)
	if err != nil || applied {
		t.Fatalf("stale apply = %v, %v", applied, err)
	}

	checklistLocal, err := d.SaveChecklistFor(0, checklistForSync("cl_local", "Local", "private", 0))
	if err != nil {
		t.Fatal(err)
	}
	checklistRemote := checklistLocal
	checklistRemote.Name = "Remote"
	checklistRemote.UpdatedAt = checklistLocal.UpdatedAt + 1000
	checklistRemote.Revision = checklistLocal.Revision + 1
	applied, err = d.ApplySyncedChecklist(checklistRemote)
	if err != nil || !applied {
		t.Fatalf("checklist apply = %v, %v", applied, err)
	}
	loadedChecklist, err := d.ChecklistFor("cl_local", 0, false)
	if err != nil || loadedChecklist.Name != "Remote" {
		t.Fatalf("checklist = %+v, %v", loadedChecklist, err)
	}
}
