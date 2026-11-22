package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"dcsmanager/internal/flightplan"
	"dcsmanager/internal/idgen"
)

// SaveFlightPlan inserts or updates a plan owned by the local player. It is the
// desktop entry point; the server uses SaveFlightPlanFor with an account id.
func (d *DB) SaveFlightPlan(plan flightplan.Plan) (flightplan.Plan, error) {
	return d.SaveFlightPlanFor(0, plan)
}

// SaveFlightPlanFor inserts or updates a plan. ownerID is 0 on the desktop and
// the account id on the server; an existing owner is never cleared by a local
// save.
func (d *DB) SaveFlightPlanFor(ownerID int64, plan flightplan.Plan) (flightplan.Plan, error) {
	route, err := json.Marshal(plan.Route)
	if err != nil {
		return plan, err
	}
	if plan.Route == nil {
		route = []byte("[]")
	}
	if plan.Visibility == "" {
		plan.Visibility = "private"
	}
	now := time.Now().UnixMilli()
	revision, err := d.nextSyncRevision()
	if err != nil {
		return plan, err
	}
	if plan.ID == 0 {
		if plan.UID == "" {
			plan.UID = idgen.New("fp_")
		}
		plan.Revision = 1
		result, err := d.sql.Exec(
			`INSERT INTO flight_plans(uid, revision, owner_id, visibility, deleted_at,
			   sync_revision, dirty, name, aircraft, departure, arrival,
			   cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
			   wind_kt, flight_rules, notes, route, created_at, updated_at)
			 VALUES(?, 1, ?, ?, NULL, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			plan.UID, nullID(ownerID), plan.Visibility, revision,
			plan.Name, nullStr(plan.Aircraft), nullStr(plan.Departure), nullStr(plan.Arrival),
			plan.CruiseAltFt, plan.CruiseTASKt, plan.FuelBurnPPH, plan.ReserveMin,
			plan.WindFromDeg, plan.WindKt, plan.FlightRules, nullStr(plan.Notes),
			string(route), now, now,
		)
		if err != nil {
			return plan, err
		}
		plan.ID, err = result.LastInsertId()
		if err != nil {
			return plan, err
		}
		plan.OwnerID, plan.DeletedAt, plan.SyncRevision = ownerID, 0, revision
		plan.CreatedAt, plan.UpdatedAt = now, now
		return plan, nil
	}

	result, err := d.sql.Exec(
		`UPDATE flight_plans
		 SET uid = CASE WHEN ? = '' THEN uid ELSE ? END,
		     owner_id = CASE WHEN owner_id IS NULL OR owner_id = 0 THEN ? ELSE owner_id END,
		     visibility = CASE WHEN ? = '' THEN visibility ELSE ? END,
		     deleted_at = NULL,
		     sync_revision = ?,
		     dirty = 1,
		     name = ?, aircraft = ?, departure = ?, arrival = ?, cruise_alt_ft = ?,
		     cruise_tas_kt = ?, fuel_burn_pph = ?, reserve_min = ?, wind_from_deg = ?,
		     wind_kt = ?, flight_rules = ?, notes = ?, route = ?,
		     revision = revision + 1, updated_at = ?
		 WHERE id = ?`,
		plan.UID, plan.UID, ownerID, plan.Visibility, plan.Visibility,
		revision,
		plan.Name, nullStr(plan.Aircraft), nullStr(plan.Departure), nullStr(plan.Arrival),
		plan.CruiseAltFt, plan.CruiseTASKt, plan.FuelBurnPPH, plan.ReserveMin,
		plan.WindFromDeg, plan.WindKt, plan.FlightRules, nullStr(plan.Notes),
		string(route), now, plan.ID,
	)
	if err != nil {
		return plan, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return plan, err
	}
	if affected == 0 {
		return plan, sql.ErrNoRows
	}
	if err := d.sql.QueryRow(
		`SELECT uid, revision, COALESCE(owner_id,0), visibility, COALESCE(sync_revision,0)
		 FROM flight_plans WHERE id = ?`, plan.ID,
	).Scan(&plan.UID, &plan.Revision, &plan.OwnerID, &plan.Visibility, &plan.SyncRevision); err != nil {
		return plan, err
	}
	plan.DeletedAt = 0
	plan.UpdatedAt = now
	return plan, nil
}

// FlightPlans lists saved plans visible to the local player (all of them).
func (d *DB) FlightPlans(limit int) ([]flightplan.Plan, error) {
	return d.FlightPlansFor(0, false, limit)
}

// FlightPlansFor lists plans visible to an account. ownerID 0 means the
// desktop and returns every plan; an administrator sees every server plan; a
// member sees their own plans plus the squadron ones.
func (d *DB) FlightPlansFor(ownerID int64, admin bool, limit int) ([]flightplan.Plan, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, COALESCE(uid,''), revision, COALESCE(owner_id,0), visibility,
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 name, COALESCE(aircraft,''), COALESCE(departure,''), COALESCE(arrival,''),
	                 cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
	                 wind_kt, COALESCE(flight_rules,'VFR'), COALESCE(notes,''), route,
	                 created_at, updated_at
	          FROM flight_plans WHERE deleted_at IS NULL`
	var args []any
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron' OR owner_id IS NULL OR owner_id = 0)`
		args = append(args, ownerID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

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

// FlightPlan reads one plan by id, without an ownership filter.
func (d *DB) FlightPlan(id int64) (flightplan.Plan, error) {
	return d.FlightPlanFor(id, 0, false)
}

// FlightPlanFor reads a plan the account may see. A private plan of another
// member reports sql.ErrNoRows, which the API turns into 404.
func (d *DB) FlightPlanFor(id, ownerID int64, admin bool) (flightplan.Plan, error) {
	query := `SELECT id, COALESCE(uid,''), revision, COALESCE(owner_id,0), visibility,
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 name, COALESCE(aircraft,''), COALESCE(departure,''), COALESCE(arrival,''),
	                 cruise_alt_ft, cruise_tas_kt, fuel_burn_pph, reserve_min, wind_from_deg,
	                 wind_kt, COALESCE(flight_rules,'VFR'), COALESCE(notes,''), route,
	                 created_at, updated_at
	          FROM flight_plans WHERE id = ? AND deleted_at IS NULL`
	args := []any{id}
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron' OR owner_id IS NULL OR owner_id = 0)`
		args = append(args, ownerID)
	}
	return scanFlightPlan(d.sql.QueryRow(query, args...))
}

// DeleteFlightPlan soft-deletes a plan. The row is kept as a tombstone so the
// deletion can reach the other side of a synchronisation.
func (d *DB) DeleteFlightPlan(id int64) error {
	return d.DeleteFlightPlanFor(id, 0, false)
}

// DeleteFlightPlanFor soft-deletes a plan visible to the account.
func (d *DB) DeleteFlightPlanFor(id, ownerID int64, admin bool) error {
	if _, err := d.FlightPlanFor(id, ownerID, admin); err != nil {
		return err
	}
	revision, err := d.nextSyncRevision()
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err = d.sql.Exec(
		`UPDATE flight_plans SET deleted_at = ?, updated_at = ?,
		   revision = revision + 1, sync_revision = ?, dirty = 1 WHERE id = ?`,
		now, now, revision, id,
	)
	return err
}

func scanFlightPlan(row interface{ Scan(...any) error }) (flightplan.Plan, error) {
	var (
		plan  flightplan.Plan
		route string
	)
	if err := row.Scan(&plan.ID, &plan.UID, &plan.Revision, &plan.OwnerID, &plan.Visibility,
		&plan.DeletedAt, &plan.SyncRevision, &plan.Name, &plan.Aircraft, &plan.Departure, &plan.Arrival,
		&plan.CruiseAltFt, &plan.CruiseTASKt, &plan.FuelBurnPPH, &plan.ReserveMin,
		&plan.WindFromDeg, &plan.WindKt, &plan.FlightRules, &plan.Notes, &route,
		&plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return plan, err
	}
	if route == "" {
		route = "[]"
	}
	if err := json.Unmarshal([]byte(route), &plan.Route); err != nil {
		return plan, err
	}
	return plan, nil
}
