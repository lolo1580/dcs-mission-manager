package db

import (
	"encoding/json"
	"time"

	"dcsmanager/internal/checklist"
	"dcsmanager/internal/idgen"
)

// SaveChecklist inserts a checklist or updates the one with the same uid.
func (d *DB) SaveChecklist(c checklist.Checklist) (checklist.Checklist, error) {
	return d.SaveChecklistFor(0, c)
}

// SaveChecklistFor inserts or updates a checklist owned by an account. ownerID
// is 0 on the desktop; an existing server owner is preserved.
func (d *DB) SaveChecklistFor(ownerID int64, c checklist.Checklist) (checklist.Checklist, error) {
	items, err := json.Marshal(c.Items)
	if err != nil {
		return c, err
	}
	if c.Items == nil {
		items = []byte("[]")
	}
	if c.UID == "" {
		c.UID = idgen.New("cl_")
	}
	if c.Visibility == "" {
		c.Visibility = "private"
	}
	now := time.Now().UnixMilli()
	revision, err := d.nextSyncRevision()
	if err != nil {
		return c, err
	}

	result, err := d.sql.Exec(
		`UPDATE checklists
		 SET name = ?, aircraft = ?, phase = ?, description = ?, source = ?,
		     owner_id = CASE WHEN owner_id IS NULL OR owner_id = 0 THEN ? ELSE owner_id END,
		     visibility = ?, deleted_at = NULL, sync_revision = ?, dirty = 1,
		     items = ?, revision = revision + 1, updated_at = ?
		 WHERE uid = ?`,
		c.Name, nullStr(c.Aircraft), c.Phase, nullStr(c.Description), c.Source,
		ownerID,
		c.Visibility, revision, string(items), now, c.UID,
	)
	if err != nil {
		return c, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return c, err
	}
	if affected == 0 {
		_, err = d.sql.Exec(
			`INSERT INTO checklists(uid, name, aircraft, phase, description, source,
			   visibility, revision, owner_id, deleted_at, sync_revision, dirty, items, created_at, updated_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?, 1, ?, NULL, ?, 1, ?, ?, ?)`,
			c.UID, c.Name, nullStr(c.Aircraft), c.Phase, nullStr(c.Description), c.Source,
			c.Visibility, nullID(ownerID), revision, string(items), now, now,
		)
		if err != nil {
			return c, err
		}
	}
	if err := d.sql.QueryRow(
		`SELECT id, revision, COALESCE(owner_id,0), visibility,
		        COALESCE(sync_revision,0), created_at
		 FROM checklists WHERE uid = ?`, c.UID,
	).Scan(&c.ID, &c.Revision, &c.OwnerID, &c.Visibility, &c.SyncRevision, &c.CreatedAt); err != nil {
		return c, err
	}
	c.DeletedAt = 0
	c.UpdatedAt = now
	return c, nil
}

// Checklists lists checklists visible to the local player (all of them).
func (d *DB) Checklists(limit int) ([]checklist.Checklist, error) {
	return d.ChecklistsFor(0, false, limit)
}

// ChecklistsFor lists checklists visible to an account.
func (d *DB) ChecklistsFor(ownerID int64, admin bool, limit int) ([]checklist.Checklist, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	query := `SELECT id, uid, name, COALESCE(aircraft,''), phase, COALESCE(description,''),
	                 source, visibility, revision, COALESCE(owner_id,0),
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 items, created_at, updated_at
	          FROM checklists WHERE deleted_at IS NULL`
	var args []any
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron')`
		args = append(args, ownerID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

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

// Checklist reads one checklist by uid, without an ownership filter.
func (d *DB) Checklist(uid string) (checklist.Checklist, error) {
	return d.ChecklistFor(uid, 0, false)
}

// ChecklistFor reads a checklist the account may see.
func (d *DB) ChecklistFor(uid string, ownerID int64, admin bool) (checklist.Checklist, error) {
	query := `SELECT id, uid, name, COALESCE(aircraft,''), phase, COALESCE(description,''),
	                 source, visibility, revision, COALESCE(owner_id,0),
	                 COALESCE(deleted_at,0), COALESCE(sync_revision,0),
	                 items, created_at, updated_at
	          FROM checklists WHERE uid = ? AND deleted_at IS NULL`
	args := []any{uid}
	if ownerID != 0 && !admin {
		query += ` AND (owner_id = ? OR visibility = 'squadron')`
		args = append(args, ownerID)
	}
	return scanChecklist(d.sql.QueryRow(query, args...))
}

// DeleteChecklist soft-deletes a checklist.
func (d *DB) DeleteChecklist(uid string) error {
	return d.DeleteChecklistFor(uid, 0, false)
}

// DeleteChecklistFor soft-deletes a checklist visible to the account.
func (d *DB) DeleteChecklistFor(uid string, ownerID int64, admin bool) error {
	if _, err := d.ChecklistFor(uid, ownerID, admin); err != nil {
		return err
	}
	revision, err := d.nextSyncRevision()
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err = d.sql.Exec(
		`UPDATE checklists SET deleted_at = ?, updated_at = ?,
		   revision = revision + 1, sync_revision = ?, dirty = 1 WHERE uid = ?`,
		now, now, revision, uid,
	)
	return err
}

func scanChecklist(row interface{ Scan(...any) error }) (checklist.Checklist, error) {
	var (
		c     checklist.Checklist
		items string
	)
	if err := row.Scan(&c.ID, &c.UID, &c.Name, &c.Aircraft, &c.Phase, &c.Description,
		&c.Source, &c.Visibility, &c.Revision, &c.OwnerID, &c.DeletedAt,
		&c.SyncRevision, &items, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	if items == "" {
		items = "[]"
	}
	if err := json.Unmarshal([]byte(items), &c.Items); err != nil {
		return c, err
	}
	if c.Items == nil {
		c.Items = []checklist.Item{}
	}
	return c, nil
}
