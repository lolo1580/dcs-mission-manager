package app

import (
	"path/filepath"
	"testing"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
)

func TestBootstrapAdmin(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := bootstrapAdmin(database, config.Config{}); err == nil {
		t.Fatal("empty credentials were accepted")
	}
	if err := bootstrapAdmin(database, config.Config{AdminUser: "admin", AdminPassword: "short"}); err == nil {
		t.Fatal("short password was accepted")
	}
	if err := bootstrapAdmin(database, config.Config{AdminUser: "admin", AdminPassword: "verysecret123"}); err != nil {
		t.Fatal(err)
	}
	user, err := database.UserByUsername("admin")
	if err != nil || user.Role != auth.RoleAdmin || !user.Active {
		t.Fatalf("admin = %+v, %v", user, err)
	}
	// A second start must not try to create the account again.
	if err := bootstrapAdmin(database, config.Config{AdminUser: "other", AdminPassword: "anothersecret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UserByUsername("other"); err == nil {
		t.Fatal("bootstrap created a second account")
	}
}
