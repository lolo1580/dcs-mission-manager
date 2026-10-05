package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmptyPayloadShapes(t *testing.T) {
	cases := map[string]string{
		"pilots":  `"pilots"`,
		"weapons": `"weapons"`,
		"engines": `"engines"`,
		"network": `"network"`,
	}
	for kind, key := range cases {
		b, err := json.Marshal(emptyPayload(kind))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), key) {
			t.Errorf("emptyPayload(%q) = %s, want key %s", kind, b, key)
		}
	}
}

// TestServesWeb validates that the embedded web assets are served without any
// database or manager (the FileServer route never touches the store).
func TestServesWeb(t *testing.T) {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Config{}, nil, nil, web)

	for _, path := range []string{"/", "/style.css", "/app.js"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET %s returned an empty body", path)
		}
	}
}

// TestAuthDisabledByDefault: with no token, the static dashboard is served
// without authentication (the route never touches the store).
func TestAuthDisabledByDefault(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{}, nil, nil, web)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/style.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("style.css without token = %d, want 200", rec.Code)
	}
}

// TestAuthRejectsWithoutToken checks the gate is closed when a token is set.
func TestAuthRejectsWithoutToken(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{AuthToken: "s3cret"}, nil, nil, web)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET / without token = %d, want 401", rec.Code)
	}
}

// TestAuthAcceptsToken forms: header, query, then cookie.
func TestAuthAcceptsToken(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{AuthToken: "s3cret"}, nil, nil, web)

	// Bearer header.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Bearer token = %d, want 200", rec.Code)
	}

	// ?token= sets a cookie.
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?token=s3cret", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("?token= = %d, want 200", rec.Code)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "plugin_token" {
			found = true
		}
	}
	if !found {
		t.Fatal("?token= should set a plugin_token cookie")
	}
}

// TestSubtleEqual covers the constant-time comparison used for tokens.
func TestSubtleEqual(t *testing.T) {
	if !subtleEqual("abc", "abc") {
		t.Error("equal strings should match")
	}
	if subtleEqual("abc", "abd") || subtleEqual("abc", "ab") || subtleEqual("", "x") {
		t.Error("different strings must not match")
	}
}
