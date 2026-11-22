package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("not a hash", "correct horse battery") {
		t.Fatal("malformed hash accepted")
	}
}

func TestPasswordRules(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if _, err := HashPassword("          "); err == nil {
		t.Fatal("whitespace password accepted")
	}
	if !ValidPassword("1234567890") {
		t.Fatal("valid password rejected")
	}
}

func TestTokenIsRandomAndHashed(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) < 40 {
		t.Fatalf("tokens = %q / %q", a, b)
	}
	if HashToken(a) == a || HashToken(a) != HashToken(a) {
		t.Fatal("token hash is not stable")
	}
	if HashToken(a) == HashToken(b) {
		t.Fatal("different tokens hash to the same value")
	}
}

func TestUsernames(t *testing.T) {
	for _, name := range []string{"admin", "Jean-Luc", "pilot_1", "abc"} {
		if !ValidUsername(name) {
			t.Fatalf("rejected %q", name)
		}
	}
	for _, name := range []string{"a", "with space", "é", "toolongtoolongtoolongtoolongtoolong"} {
		if ValidUsername(name) {
			t.Fatalf("accepted %q", name)
		}
	}
}

func TestEmails(t *testing.T) {
	for _, email := range []string{"pilot@example.com", "jean.luc+dcs@example.co.uk"} {
		if !ValidEmail(email) {
			t.Fatalf("rejected %q", email)
		}
	}
	for _, email := range []string{"", "not-an-email", "Name <pilot@example.com>", "a b@example.com"} {
		if ValidEmail(email) {
			t.Fatalf("accepted %q", email)
		}
	}
}
