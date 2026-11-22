package db

import (
	"database/sql"
	"time"

	"dcsmanager/internal/auth"
)

// CreatePasswordReset stores a single-use reset token hash.
func (d *DB) CreatePasswordReset(userID int64, tokenHash string, expiresAt int64) error {
	_, err := d.sql.Exec(
		`INSERT INTO password_resets(token_hash, user_id, created_at, expires_at)
		 VALUES(?, ?, ?, ?)`,
		tokenHash, userID, time.Now().UnixMilli(), expiresAt,
	)
	return err
}

// PasswordResetUser resolves an unused, unexpired reset token to its account.
func (d *DB) PasswordResetUser(tokenHash string, now int64) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.email,''),
		        u.password_hash, u.role, u.active, u.dcs_names, u.created_at,
		        COALESCE(u.last_login_at,0)
		 FROM password_resets r JOIN users u ON u.id = r.user_id
		 WHERE r.token_hash = ? AND r.used_at IS NULL AND r.expires_at > ?
		   AND u.active = 1`, tokenHash, now))
}

// UsePasswordReset marks a reset token as consumed.
func (d *DB) UsePasswordReset(tokenHash string, now int64) error {
	result, err := d.sql.Exec(
		`UPDATE password_resets SET used_at = ? WHERE token_hash = ? AND used_at IS NULL`,
		now, tokenHash,
	)
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

// PrunePasswordResets removes expired or consumed tokens.
func (d *DB) PrunePasswordResets(now int64) error {
	_, err := d.sql.Exec(
		`DELETE FROM password_resets WHERE expires_at <= ? OR used_at IS NOT NULL`,
		now,
	)
	return err
}
