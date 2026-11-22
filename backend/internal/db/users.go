package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"dcsmanager/internal/auth"
)

// CreateUser inserts an account. The username is unique, case-insensitively.
func (d *DB) CreateUser(u auth.User) (auth.User, error) {
	names, err := json.Marshal(u.DCSNames)
	if err != nil {
		return u, err
	}
	now := time.Now().UnixMilli()
	if u.CreatedAt == 0 {
		u.CreatedAt = now
	}
	result, err := d.sql.Exec(
		`INSERT INTO users(username, display_name, email, password_hash, role, active, dcs_names, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, nullStr(u.DisplayName), nullStr(u.Email), u.PasswordHash, u.Role, boolInt(u.Active), string(names), u.CreatedAt,
	)
	if err != nil {
		return u, err
	}
	u.ID, err = result.LastInsertId()
	return u, err
}

// UserByUsername reads one account by its unique username.
func (d *DB) UserByUsername(username string) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT id, username, COALESCE(display_name,''), COALESCE(email,''), password_hash, role, active,
		        dcs_names, created_at, COALESCE(last_login_at,0)
		 FROM users WHERE username = ? COLLATE NOCASE`, username))
}

// UserByID reads one account by identifier.
func (d *DB) UserByID(id int64) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT id, username, COALESCE(display_name,''), COALESCE(email,''), password_hash, role, active,
		        dcs_names, created_at, COALESCE(last_login_at,0)
		 FROM users WHERE id = ?`, id))
}

// Users lists every account, oldest first.
func (d *DB) Users() ([]auth.User, error) {
	rows, err := d.sql.Query(
		`SELECT id, username, COALESCE(display_name,''), COALESCE(email,''), password_hash, role, active,
		        dcs_names, created_at, COALESCE(last_login_at,0)
		 FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []auth.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers reports how many accounts exist, used for first-run bootstrap.
func (d *DB) CountUsers() (int, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// UpdateUser changes the profile fields of an account. PasswordHash, when not
// empty, replaces the password at the same time.
func (d *DB) UpdateUser(id int64, displayName, role string, active bool, dcsNames []string, passwordHash string) error {
	names, err := json.Marshal(dcsNames)
	if err != nil {
		return err
	}
	if passwordHash == "" {
		_, err = d.sql.Exec(
			`UPDATE users SET display_name = ?, role = ?, active = ?, dcs_names = ? WHERE id = ?`,
			nullStr(displayName), role, boolInt(active), string(names), id,
		)
		return err
	}
	_, err = d.sql.Exec(
		`UPDATE users SET display_name = ?, role = ?, active = ?, dcs_names = ?, password_hash = ? WHERE id = ?`,
		nullStr(displayName), role, boolInt(active), string(names), passwordHash, id,
	)
	return err
}

// UserByEmail reads one active account by email, case-insensitively.
func (d *DB) UserByEmail(email string) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT id, username, COALESCE(display_name,''), COALESCE(email,''), password_hash, role, active,
		        dcs_names, created_at, COALESCE(last_login_at,0)
		 FROM users WHERE email = ? COLLATE NOCASE`, email))
}

// SetEmail updates the recovery email of one account. An empty value clears it.
func (d *DB) SetEmail(id int64, email string) error {
	_, err := d.sql.Exec(`UPDATE users SET email = ? WHERE id = ?`, nullStr(email), id)
	return err
}

// SetPassword replaces the password hash of one account.
func (d *DB) SetPassword(id int64, hash string) error {
	_, err := d.sql.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// DeleteUser removes an account and, through the foreign keys, its sessions and
// API tokens.
func (d *DB) DeleteUser(id int64) error {
	result, err := d.sql.Exec(`DELETE FROM users WHERE id = ?`, id)
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

// TouchLogin records a successful sign-in.
func (d *DB) TouchLogin(id int64) error {
	_, err := d.sql.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, time.Now().UnixMilli(), id)
	return err
}

// CountActiveAdmins reports how many active administrators remain, so the last
// one cannot be deleted or demoted by accident.
func (d *DB) CountActiveAdmins() (int, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ? AND active = 1`, auth.RoleAdmin).Scan(&n)
	return n, err
}

// ---- Sessions -------------------------------------------------------------

// CreateSession stores a hashed session token with its expiry.
func (d *DB) CreateSession(tokenHash string, userID int64, expiresAt int64, userAgent string) error {
	_, err := d.sql.Exec(
		`INSERT INTO sessions(token_hash, user_id, created_at, expires_at, user_agent)
		 VALUES(?, ?, ?, ?, ?)`,
		tokenHash, userID, time.Now().UnixMilli(), expiresAt, userAgent,
	)
	return err
}

// SessionUser resolves a session token to an active account. Expired or unknown
// tokens report sql.ErrNoRows.
func (d *DB) SessionUser(tokenHash string, now int64) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.email,''), u.password_hash, u.role,
		        u.active, u.dcs_names, u.created_at, COALESCE(u.last_login_at,0)
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ? AND u.active = 1`, tokenHash, now))
}

// DeleteSession removes one session by token hash.
func (d *DB) DeleteSession(tokenHash string) error {
	_, err := d.sql.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions removes every session of an account.
func (d *DB) DeleteUserSessions(userID int64) error {
	_, err := d.sql.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// PruneSessions removes expired sessions.
func (d *DB) PruneSessions(now int64) error {
	_, err := d.sql.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now)
	return err
}

// ---- API tokens -----------------------------------------------------------

// CreateAPIToken stores a hashed personal token and returns its metadata.
func (d *DB) CreateAPIToken(userID int64, name, tokenHash string) (auth.APIToken, error) {
	now := time.Now().UnixMilli()
	result, err := d.sql.Exec(
		`INSERT INTO api_tokens(user_id, name, token_hash, created_at) VALUES(?, ?, ?, ?)`,
		userID, name, tokenHash, now,
	)
	if err != nil {
		return auth.APIToken{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return auth.APIToken{}, err
	}
	return auth.APIToken{ID: id, Name: name, CreatedAt: now}, nil
}

// APITokenUser resolves a personal token to an active account.
func (d *DB) APITokenUser(tokenHash string, now int64) (auth.User, error) {
	return scanUser(d.sql.QueryRow(
		`SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.email,''), u.password_hash, u.role,
		        u.active, u.dcs_names, u.created_at, COALESCE(u.last_login_at,0)
		 FROM api_tokens t JOIN users u ON u.id = t.user_id
		 WHERE t.token_hash = ? AND u.active = 1`, tokenHash))
}

// TouchAPIToken records the last use of a personal token.
func (d *DB) TouchAPIToken(tokenHash string) error {
	_, err := d.sql.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE token_hash = ?`, time.Now().UnixMilli(), tokenHash)
	return err
}

// APITokens lists the tokens of one account. The hash is never returned.
func (d *DB) APITokens(userID int64) ([]auth.APIToken, error) {
	rows, err := d.sql.Query(
		`SELECT id, name, created_at, COALESCE(last_used_at,0)
		 FROM api_tokens WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []auth.APIToken
	for rows.Next() {
		var t auth.APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteAPIToken removes one token belonging to an account.
func (d *DB) DeleteAPIToken(userID, id int64) error {
	result, err := d.sql.Exec(`DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
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

// ---- Audit ----------------------------------------------------------------

// Audit appends one entry to the server audit log.
func (d *DB) Audit(userID int64, action, detail string) error {
	_, err := d.sql.Exec(
		`INSERT INTO audit_log(user_id, action, detail, real_ts) VALUES(?, ?, ?, ?)`,
		nullID(userID), action, nullStr(detail), time.Now().UnixMilli(),
	)
	return err
}

// AuditEntries returns the most recent audit lines.
func (d *DB) AuditEntries(limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT a.id, COALESCE(a.user_id,0), COALESCE(u.username,''), a.action,
		        COALESCE(a.detail,''), a.real_ts
		 FROM audit_log a LEFT JOIN users u ON u.id = a.user_id
		 ORDER BY a.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &e.Detail, &e.RealTS); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AuditEntry is one line of the server audit log.
type AuditEntry struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"userId,omitempty"`
	Username string `json:"username,omitempty"`
	Action   string `json:"action"`
	Detail   string `json:"detail,omitempty"`
	RealTS   int64  `json:"realTs"`
}

func scanUser(row interface{ Scan(...any) error }) (auth.User, error) {
	var (
		u     auth.User
		names string
	)
	if err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash, &u.Role,
		&u.Active, &names, &u.CreatedAt, &u.LastLoginAt); err != nil {
		return u, err
	}
	if names != "" {
		if err := json.Unmarshal([]byte(names), &u.DCSNames); err != nil {
			return u, err
		}
	}
	if u.DCSNames == nil {
		u.DCSNames = []string{}
	}
	return u, nil
}

func nullID(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
