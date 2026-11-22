package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// nextSyncRevision increments and returns the server-wide document revision.
// The mutex makes the two statements atomic within this process; SQLite's write
// lock does the same across processes.
func (d *DB) nextSyncRevision() (int64, error) {
	d.syncRevisionMu.Lock()
	defer d.syncRevisionMu.Unlock()
	if _, err := d.sql.Exec(
		`INSERT INTO server_meta(key, value) VALUES('revision', 1)
		 ON CONFLICT(key) DO UPDATE SET value = value + 1`,
	); err != nil {
		return 0, err
	}
	var revision int64
	if err := d.sql.QueryRow(`SELECT value FROM server_meta WHERE key = 'revision'`).Scan(&revision); err != nil {
		return 0, err
	}
	return revision, nil
}

// SyncCursor returns the current revision without incrementing it.
func (d *DB) SyncCursor() (int64, error) {
	var revision int64
	err := d.sql.QueryRow(`SELECT value FROM server_meta WHERE key = 'revision'`).Scan(&revision)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return revision, err
}

// ConflictCopy is a version rejected by last-write-wins.
type ConflictCopy struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	UID       string `json:"uid"`
	OwnerID   int64  `json:"ownerId,omitempty"`
	Revision  int64  `json:"revision,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	Payload   string `json:"payload"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

// RecordConflict stores the losing version of a document.
func (d *DB) RecordConflict(kind, uid string, ownerID int64, payload any, reason string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(
		`INSERT INTO conflict_copies(kind, uid, owner_id, payload, reason, created_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		kind, uid, nullID(ownerID), string(data), nullStr(reason), time.Now().UnixMilli(),
	)
	return err
}

// Conflicts lists conflict copies. Administrators see every account; a member
// sees the copies recorded against their own documents.
func (d *DB) Conflicts(ownerID int64, admin bool, limit int) ([]ConflictCopy, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, kind, uid, COALESCE(owner_id,0), COALESCE(revision,0),
	                 COALESCE(updated_at,0), payload, COALESCE(reason,''), created_at
	          FROM conflict_copies`
	var args []any
	if !admin {
		query += ` WHERE owner_id = ?`
		args = append(args, ownerID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConflictCopy
	for rows.Next() {
		var c ConflictCopy
		if err := rows.Scan(&c.ID, &c.Kind, &c.UID, &c.OwnerID, &c.Revision,
			&c.UpdatedAt, &c.Payload, &c.Reason, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Conflict reads one conflict copy.
func (d *DB) Conflict(id int64) (ConflictCopy, error) {
	var c ConflictCopy
	err := d.sql.QueryRow(
		`SELECT id, kind, uid, COALESCE(owner_id,0), COALESCE(revision,0),
		        COALESCE(updated_at,0), payload, COALESCE(reason,''), created_at
		 FROM conflict_copies WHERE id = ?`, id,
	).Scan(&c.ID, &c.Kind, &c.UID, &c.OwnerID, &c.Revision, &c.UpdatedAt,
		&c.Payload, &c.Reason, &c.CreatedAt)
	return c, err
}

// DeleteConflict removes one conflict copy.
func (d *DB) DeleteConflict(id int64) error {
	result, err := d.sql.Exec(`DELETE FROM conflict_copies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
