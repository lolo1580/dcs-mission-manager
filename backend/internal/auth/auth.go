// Package auth contains the account model's pure pieces: password hashing,
// random session and API tokens, and the validation rules shared by the API.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Roles are deliberately few: an administrator manages accounts and sees every
// record; a member sees the squadron's shared data and owns their own documents.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// MinPasswordLength is the smallest accepted password. The server never stores
// the password itself, only its Argon2id hash.
const MinPasswordLength = 10

// UsernamePattern is the accepted account name. It is intentionally narrow so
// that a name is also safe in a URL and in a log line.
var UsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

// User is an account. PasswordHash is never serialised.
type User struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName,omitempty"`
	Email       string   `json:"email,omitempty"`
	Role        string   `json:"role"`
	Active      bool     `json:"active"`
	DCSNames    []string `json:"dcsNames,omitempty"`
	CreatedAt   int64    `json:"createdAt"`
	LastLoginAt int64    `json:"lastLoginAt,omitempty"`

	PasswordHash string `json:"-"`
}

// APIToken is a personal token used by a desktop to upload data. Only its hash
// is stored; the clear value is shown once at creation.
type APIToken struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
}

// ValidRole reports whether role is known.
func ValidRole(role string) bool { return role == RoleAdmin || role == RoleMember }

// ValidUsername reports whether name is an acceptable account name.
func ValidUsername(name string) bool { return UsernamePattern.MatchString(name) }

// ValidEmail reports whether value is a plain email address (without a display
// name), which is what the password-reset flow needs.
func ValidEmail(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

// ValidPassword enforces the minimum length and rejects a password made only
// of whitespace. Length is counted in bytes, which is a lower bound for UTF-8
// passwords and therefore never more permissive than the displayed rule.
func ValidPassword(password string) bool {
	return len(password) >= MinPasswordLength && strings.TrimSpace(password) != ""
}

// HashPassword derives an Argon2id hash with the parameters stored alongside it.
// The encoded form is self-describing, so the parameters can be raised later
// without invalidating existing passwords.
func HashPassword(password string) (string, error) {
	if !ValidPassword(password) {
		return "", fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	const (
		timeCost   = 2
		memoryKiB  = 64 * 1024
		threads    = 4
		keyLength  = 32
		saltLength = 16
	)
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, timeCost, memoryKiB, threads, keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memoryKiB, timeCost, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash. It runs in
// constant time with respect to the derived key.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false
	}
	if memory == 0 || timeCost == 0 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken returns 32 random bytes encoded for a cookie or an Authorization
// header.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken is what the database stores for a session or API token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ErrLastAdmin is returned when an operation would remove the last active
// administrator.
var ErrLastAdmin = errors.New("at least one active administrator is required")
