package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"dcsmanager/internal/auth"
)

func testUser(t *testing.T) auth.User {
	t.Helper()
	hash, err := auth.HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	return auth.User{
		Username:     "admin",
		DisplayName:  "Administrator",
		Email:        "admin@example.com",
		Role:         auth.RoleAdmin,
		Active:       true,
		DCSNames:     []string{"Viper"},
		PasswordHash: hash,
	}
}

func TestUserCRUD(t *testing.T) {
	d := openTemp(t)
	u := testUser(t)
	saved, err := d.CreateUser(u)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.CreatedAt == 0 {
		t.Fatalf("not stamped: %+v", saved)
	}

	byName, err := d.UserByUsername("ADMIN")
	if err != nil || byName.ID != saved.ID {
		t.Fatalf("by name = %+v, %v", byName, err)
	}
	if len(byName.DCSNames) != 1 || byName.DCSNames[0] != "Viper" {
		t.Fatalf("dcs names = %+v", byName.DCSNames)
	}
	if byName.Email != "admin@example.com" {
		t.Fatalf("email = %q", byName.Email)
	}
	if byEmail, err := d.UserByEmail("ADMIN@example.com"); err != nil || byEmail.ID != saved.ID {
		t.Fatalf("by email = %+v, %v", byEmail, err)
	}
	if !auth.VerifyPassword(byName.PasswordHash, "correct horse battery") {
		t.Fatal("hash not stored or not verifiable")
	}

	if err := d.UpdateUser(saved.ID, "Admin", auth.RoleMember, true, []string{"Hornet"}, ""); err != nil {
		t.Fatal(err)
	}
	updated, err := d.UserByID(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != "Admin" || updated.Role != auth.RoleMember || updated.DCSNames[0] != "Hornet" {
		t.Fatalf("updated = %+v", updated)
	}

	count, err := d.CountUsers()
	if err != nil || count != 1 {
		t.Fatalf("count = %d, %v", count, err)
	}
	if err := d.DeleteUser(saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UserByID(saved.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestSessions(t *testing.T) {
	d := openTemp(t)
	u, err := d.CreateUser(testUser(t))
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("session-token")
	now := time.Now().UnixMilli()
	if err := d.CreateSession(hash, u.ID, now+60_000, "test"); err != nil {
		t.Fatal(err)
	}
	got, err := d.SessionUser(hash, now)
	if err != nil || got.ID != u.ID {
		t.Fatalf("session user = %+v, %v", got, err)
	}
	if _, err := d.SessionUser(hash, now+120_000); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired session: %v", err)
	}
	if err := d.DeleteSession(hash); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SessionUser(hash, now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted session: %v", err)
	}
}

func TestAPITokens(t *testing.T) {
	d := openTemp(t)
	u, err := d.CreateUser(testUser(t))
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("api-token")
	token, err := d.CreateAPIToken(u.ID, "Desktop", hash)
	if err != nil {
		t.Fatal(err)
	}
	if token.ID == 0 {
		t.Fatalf("token = %+v", token)
	}
	got, err := d.APITokenUser(hash, time.Now().UnixMilli())
	if err != nil || got.ID != u.ID {
		t.Fatalf("token user = %+v, %v", got, err)
	}
	list, err := d.APITokens(u.ID)
	if err != nil || len(list) != 1 || list[0].Name != "Desktop" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if err := d.DeleteAPIToken(u.ID, token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.APITokenUser(hash, time.Now().UnixMilli()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted token: %v", err)
	}
}

func TestPasswordReset(t *testing.T) {
	d := openTemp(t)
	u, err := d.CreateUser(testUser(t))
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("reset-token")
	now := time.Now().UnixMilli()
	if err := d.CreatePasswordReset(u.ID, hash, now+60_000); err != nil {
		t.Fatal(err)
	}
	got, err := d.PasswordResetUser(hash, now)
	if err != nil || got.ID != u.ID {
		t.Fatalf("reset user = %+v, %v", got, err)
	}
	if _, err := d.PasswordResetUser(hash, now+120_000); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired reset: %v", err)
	}
	if err := d.UsePasswordReset(hash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PasswordResetUser(hash, now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("used reset: %v", err)
	}
}

func TestAudit(t *testing.T) {
	d := openTemp(t)
	u, err := d.CreateUser(testUser(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Audit(u.ID, "login", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	entries, err := d.AuditEntries(10)
	if err != nil || len(entries) != 1 || entries[0].Username != "admin" {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
}
