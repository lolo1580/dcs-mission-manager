package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"

	"dcsmanager/internal/checklist"
	"dcsmanager/internal/flightplan"
)

// ---- Desktop: export local documents and apply server changes -------------

// ExportFlightPlansForSync returns the local plans that changed since the last
// successful push, including tombstones. The dirty flag is the outbox: remote
// versions applied by a pull are clean, so they are never sent back.
func (d *DB) ExportFlightPlansForSync(limit int) ([]flightplan.Plan, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	return d.queryPlans(
		`SELECT id, COALESCE(uid,''), revision, COALESCE(owner_id,0), visibility,
		        COALESCE(deleted_at,0), COALESCE(sync_revision,0),
		        name, COALESCE(aircraft,''), COALESCE(departure,''), COALESCE(arrival,''),
		        cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
		        wind_kt, COALESCE(flight_rules,'VFR'), COALESCE(notes,''), route,
		        created_at, updated_at
		 FROM flight_plans WHERE dirty = 1 ORDER BY updated_at ASC, id ASC LIMIT ?`, limit)
}

// ExportChecklistsForSync returns the local checklists changed since the last
// successful push, including tombstones.
func (d *DB) ExportChecklistsForSync(limit int) ([]checklist.Checklist, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	return d.queryChecklists(
		`SELECT id, uid, name, COALESCE(aircraft,''), phase, COALESCE(description,''),
		        source, visibility, revision, COALESCE(owner_id,0),
		        COALESCE(deleted_at,0), COALESCE(sync_revision,0),
		        items, created_at, updated_at
		 FROM checklists WHERE dirty = 1 ORDER BY updated_at ASC, id ASC LIMIT ?`, limit)
}

// ApplySyncedFlightPlan stores a server version if it is newer than the local
// one. It is used by the desktop after a pull.
func (d *DB) ApplySyncedFlightPlan(plan flightplan.Plan) (bool, error) {
	if plan.UID == "" {
		return false, errors.New("synced plan has no uid")
	}
	if plan.UpdatedAt == 0 {
		plan.UpdatedAt = time.Now().UnixMilli()
	}
	existing, err := d.planByUID(plan.UID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true, d.insertSyncedPlan(plan, 0)
	case err != nil:
		return false, err
	}
	if plan.UpdatedAt <= existing.UpdatedAt {
		return false, nil
	}
	return true, d.updateSyncedPlan(existing.ID, plan, 0)
}

// ApplySyncedChecklist stores a server checklist if it is newer locally.
func (d *DB) ApplySyncedChecklist(c checklist.Checklist) (bool, error) {
	if c.UID == "" {
		return false, errors.New("synced checklist has no uid")
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = time.Now().UnixMilli()
	}
	existing, err := d.checklistByUID(c.UID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true, d.insertSyncedChecklist(c, 0)
	case err != nil:
		return false, err
	}
	if c.UpdatedAt <= existing.UpdatedAt {
		return false, nil
	}
	return true, d.updateSyncedChecklist(existing.ID, c, 0)
}

// ---- Server: push, pull and last-write-wins -------------------------------

// SyncFlightPlans returns plans changed after the cursor and visible to the
// account. Tombstones are included so deletions propagate.
func (d *DB) SyncFlightPlans(since int64, ownerID int64, admin bool, limit int) ([]flightplan.Plan, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	query := `SELECT id, COALESCE(uid,''), revision, COALESCE(owner_id,0), visibility,
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 name, COALESCE(aircraft,''), COALESCE(departure,''), COALESCE(arrival,''),
	                 cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
	                 wind_kt, COALESCE(flight_rules,'VFR'), COALESCE(notes,''), route,
	                 created_at, updated_at
	          FROM flight_plans WHERE sync_revision > ?`
	args := []any{since}
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron' OR owner_id IS NULL OR owner_id = 0)`
		args = append(args, ownerID)
	}
	query += ` ORDER BY sync_revision ASC, id ASC LIMIT ?`
	args = append(args, limit)
	return d.queryPlans(query, args...)
}

// SyncChecklists is the checklist counterpart of SyncFlightPlans.
func (d *DB) SyncChecklists(since int64, ownerID int64, admin bool, limit int) ([]checklist.Checklist, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	query := `SELECT id, uid, name, COALESCE(aircraft,''), phase, COALESCE(description,''),
	                 source, visibility, revision, COALESCE(owner_id,0),
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 items, created_at, updated_at
	          FROM checklists WHERE sync_revision > ?`
	args := []any{since}
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron' OR owner_id IS NULL OR owner_id = 0)`
		args = append(args, ownerID)
	}
	query += ` ORDER BY sync_revision ASC, id ASC LIMIT ?`
	args = append(args, limit)
	return d.queryChecklists(query, args...)
}

// UpsertFlightPlanFromSync applies one pushed plan with last-write-wins. It
// returns the winning server document and whether the incoming version won.
// The losing version is stored as a conflict copy.
func (d *DB) UpsertFlightPlanFromSync(ownerID int64, admin bool, plan flightplan.Plan) (flightplan.Plan, bool, error) {
	if plan.UID == "" {
		return plan, false, errors.New("plan has no uid")
	}
	if plan.Name == "" {
		return plan, false, errors.New("plan has no name")
	}
	if plan.Visibility == "" {
		plan.Visibility = "private"
	}
	if plan.UpdatedAt == 0 {
		plan.UpdatedAt = time.Now().UnixMilli()
	}
	existing, err := d.planByUID(plan.UID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		revision, err := d.nextSyncRevision()
		if err != nil {
			return plan, false, err
		}
		plan.Revision = maxInt64(plan.Revision, 1)
		plan.OwnerID = ownerID
		plan.SyncRevision = revision
		if plan.CreatedAt == 0 {
			plan.CreatedAt = plan.UpdatedAt
		}
		if err := d.insertSyncedPlan(plan, 0); err != nil {
			return plan, false, err
		}
		saved, err := d.planByUID(plan.UID)
		return saved, true, err
	case err != nil:
		return plan, false, err
	}

	if existing.OwnerID != 0 && existing.OwnerID != ownerID && !admin {
		_ = d.RecordConflict("flightplan", plan.UID, existing.OwnerID, plan, "owned by another member")
		return existing, false, nil
	}
	if plan.UpdatedAt < existing.UpdatedAt {
		_ = d.RecordConflict("flightplan", plan.UID, existing.OwnerID, plan, "older version")
		return existing, false, nil
	}
	if plan.UpdatedAt == existing.UpdatedAt && plan.Revision <= existing.Revision {
		return existing, false, nil
	}
	revision, err := d.nextSyncRevision()
	if err != nil {
		return plan, false, err
	}
	plan.ID = existing.ID
	plan.OwnerID = existing.OwnerID
	if plan.OwnerID == 0 {
		plan.OwnerID = ownerID
	}
	if plan.Visibility == "" {
		plan.Visibility = existing.Visibility
	}
	plan.Revision = maxInt64(existing.Revision+1, plan.Revision)
	plan.SyncRevision = revision
	if plan.CreatedAt == 0 {
		plan.CreatedAt = existing.CreatedAt
	}
	if err := d.updateSyncedPlan(existing.ID, plan, 0); err != nil {
		return plan, false, err
	}
	saved, err := d.planByUID(plan.UID)
	return saved, true, err
}

// UpsertChecklistFromSync is the checklist counterpart.
func (d *DB) UpsertChecklistFromSync(ownerID int64, admin bool, c checklist.Checklist) (checklist.Checklist, bool, error) {
	if c.UID == "" {
		return c, false, errors.New("checklist has no uid")
	}
	if c.Visibility == "" {
		c.Visibility = "private"
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = time.Now().UnixMilli()
	}
	existing, err := d.checklistByUID(c.UID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		revision, err := d.nextSyncRevision()
		if err != nil {
			return c, false, err
		}
		c.Revision = maxInt64(c.Revision, 1)
		c.OwnerID = ownerID
		c.SyncRevision = revision
		if c.CreatedAt == 0 {
			c.CreatedAt = c.UpdatedAt
		}
		if err := d.insertSyncedChecklist(c, 0); err != nil {
			return c, false, err
		}
		saved, err := d.checklistByUID(c.UID)
		return saved, true, err
	case err != nil:
		return c, false, err
	}

	if existing.OwnerID != 0 && existing.OwnerID != ownerID && !admin {
		_ = d.RecordConflict("checklist", c.UID, existing.OwnerID, c, "owned by another member")
		return existing, false, nil
	}
	if c.UpdatedAt < existing.UpdatedAt {
		_ = d.RecordConflict("checklist", c.UID, existing.OwnerID, c, "older version")
		return existing, false, nil
	}
	if c.UpdatedAt == existing.UpdatedAt && c.Revision <= existing.Revision {
		return existing, false, nil
	}
	revision, err := d.nextSyncRevision()
	if err != nil {
		return c, false, err
	}
	c.ID = existing.ID
	c.OwnerID = existing.OwnerID
	if c.OwnerID == 0 {
		c.OwnerID = ownerID
	}
	if c.Visibility == "" {
		c.Visibility = existing.Visibility
	}
	c.Revision = maxInt64(existing.Revision+1, c.Revision)
	c.SyncRevision = revision
	if c.CreatedAt == 0 {
		c.CreatedAt = existing.CreatedAt
	}
	if err := d.updateSyncedChecklist(existing.ID, c, 0); err != nil {
		return c, false, err
	}
	saved, err := d.checklistByUID(c.UID)
	return saved, true, err
}

// ---- Query helpers --------------------------------------------------------

func (d *DB) queryPlans(query string, args ...any) ([]flightplan.Plan, error) {
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flightplan.Plan
	for rows.Next() {
		plan, err := scanFlightPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, rows.Err()
}

func (d *DB) queryChecklists(query string, args ...any) ([]checklist.Checklist, error) {
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []checklist.Checklist
	for rows.Next() {
		c, err := scanChecklist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) planByUID(uid string) (flightplan.Plan, error) {
	return scanFlightPlan(d.sql.QueryRow(
		`SELECT id, COALESCE(uid,''), revision, COALESCE(owner_id,0), visibility,
		        COALESCE(deleted_at,0), COALESCE(sync_revision,0),
		        name, COALESCE(aircraft,''), COALESCE(departure,''), COALESCE(arrival,''),
		        cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
		        wind_kt, COALESCE(flight_rules,'VFR'), COALESCE(notes,''), route,
		        created_at, updated_at
		 FROM flight_plans WHERE uid = ?`, uid))
}

// FlightPlanByUID reads a plan including its tombstone, for conflict handling.
func (d *DB) FlightPlanByUID(uid string) (flightplan.Plan, error) { return d.planByUID(uid) }

// ChecklistByUID reads a checklist including its tombstone.
func (d *DB) ChecklistByUID(uid string) (checklist.Checklist, error) {
	return d.checklistByUID(uid)
}

func (d *DB) checklistByUID(uid string) (checklist.Checklist, error) {
	return scanChecklist(d.sql.QueryRow(
		`SELECT id, uid, name, COALESCE(aircraft,''), phase, COALESCE(description,''),
		        source, visibility, revision, COALESCE(owner_id,0),
		        COALESCE(deleted_at,0), COALESCE(sync_revision,0),
		        items, created_at, updated_at
		 FROM checklists WHERE uid = ?`, uid))
}

func (d *DB) insertSyncedPlan(plan flightplan.Plan, dirty int) error {
	route, err := json.Marshal(plan.Route)
	if err != nil {
		return err
	}
	if plan.Route == nil {
		route = []byte("[]")
	}
	result, err := d.sql.Exec(
		`INSERT INTO flight_plans(uid, revision, owner_id, visibility, deleted_at,
		   sync_revision, dirty, name, aircraft, departure, arrival,
		   cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
		   wind_kt, flight_rules, notes, route, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		plan.UID, plan.Revision, nullID(plan.OwnerID), plan.Visibility, nullInt(plan.DeletedAt),
		plan.SyncRevision, dirty, plan.Name, nullStr(plan.Aircraft), nullStr(plan.Departure), nullStr(plan.Arrival),
		plan.CruiseAltFt, plan.CruiseTASKt, plan.FuelBurnPPH, plan.ReserveMin,
		plan.WindFromDeg, plan.WindKt, plan.FlightRules, nullStr(plan.Notes),
		string(route), plan.CreatedAt, plan.UpdatedAt,
	)
	if err != nil {
		return err
	}
	plan.ID, err = result.LastInsertId()
	return err
}

func (d *DB) updateSyncedPlan(id int64, plan flightplan.Plan, dirty int) error {
	route, err := json.Marshal(plan.Route)
	if err != nil {
		return err
	}
	if plan.Route == nil {
		route = []byte("[]")
	}
	_, err = d.sql.Exec(
		`UPDATE flight_plans SET uid = ?, revision = ?, owner_id = ?, visibility = ?,
		   deleted_at = ?, sync_revision = ?, dirty = ?, name = ?, aircraft = ?, departure = ?,
		   arrival = ?, cruise_alt_ft = ?, cruise_tas_kt = ?, fuel_burn_pph = ?,
		   reserve_min = ?, wind_from_deg = ?, wind_kt = ?, flight_rules = ?,
		   notes = ?, route = ?, updated_at = ?
		 WHERE id = ?`,
		plan.UID, plan.Revision, nullID(plan.OwnerID), plan.Visibility, nullInt(plan.DeletedAt),
		plan.SyncRevision, dirty, plan.Name, nullStr(plan.Aircraft), nullStr(plan.Departure),
		nullStr(plan.Arrival), plan.CruiseAltFt, plan.CruiseTASKt, plan.FuelBurnPPH,
		plan.ReserveMin, plan.WindFromDeg, plan.WindKt, plan.FlightRules,
		nullStr(plan.Notes), string(route), plan.UpdatedAt, id,
	)
	return err
}

func (d *DB) insertSyncedChecklist(c checklist.Checklist, dirty int) error {
	items, err := json.Marshal(c.Items)
	if err != nil {
		return err
	}
	if c.Items == nil {
		items = []byte("[]")
	}
	result, err := d.sql.Exec(
		`INSERT INTO checklists(uid, name, aircraft, phase, description, source,
		   visibility, revision, owner_id, deleted_at, sync_revision, dirty, items, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.UID, c.Name, nullStr(c.Aircraft), c.Phase, nullStr(c.Description), c.Source,
		c.Visibility, c.Revision, nullID(c.OwnerID), nullInt(c.DeletedAt), c.SyncRevision,
		dirty, string(items), c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return err
	}
	c.ID, err = result.LastInsertId()
	return err
}

func (d *DB) updateSyncedChecklist(id int64, c checklist.Checklist, dirty int) error {
	items, err := json.Marshal(c.Items)
	if err != nil {
		return err
	}
	if c.Items == nil {
		items = []byte("[]")
	}
	_, err = d.sql.Exec(
		`UPDATE checklists SET uid = ?, name = ?, aircraft = ?, phase = ?, description = ?,
		   source = ?, visibility = ?, revision = ?, owner_id = ?, deleted_at = ?,
		   sync_revision = ?, dirty = ?, items = ?, updated_at = ?
		 WHERE id = ?`,
		c.UID, c.Name, nullStr(c.Aircraft), c.Phase, nullStr(c.Description), c.Source,
		c.Visibility, c.Revision, nullID(c.OwnerID), nullInt(c.DeletedAt), c.SyncRevision,
		dirty, string(items), c.UpdatedAt, id,
	)
	return err
}

// MarkFlightPlansSynced clears the outbox flag after a successful push. The
// update is conditional on the timestamp that was sent, so a local edit made
// while the request was in flight keeps its dirty flag.
func (d *DB) MarkFlightPlansSynced(plans []flightplan.Plan) error {
	for _, plan := range plans {
		if _, err := d.sql.Exec(
			`UPDATE flight_plans SET dirty = 0 WHERE uid = ? AND updated_at = ?`,
			plan.UID, plan.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// MarkChecklistsSynced clears the outbox flag after a successful push.
func (d *DB) MarkChecklistsSynced(checklists []checklist.Checklist) error {
	for _, c := range checklists {
		if _, err := d.sql.Exec(
			`UPDATE checklists SET dirty = 0 WHERE uid = ? AND updated_at = ?`,
			c.UID, c.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// BackupTo writes a consistent snapshot of the database to path using SQLite's
// VACUUM INTO. The destination must not exist.
func (d *DB) BackupTo(path string) error {
	_ = os.Remove(path)
	_, err := d.sql.Exec(`VACUUM INTO ?`, path)
	return err
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
