package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServesWeb validates that the embedded dashboard is served without any
// database (the static route never touches the store).
func TestServesWeb(t *testing.T) {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Config{}, nil, web)

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

// TestAuthDisabledByDefault: with no token the static dashboard is open.
func TestAuthDisabledByDefault(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{}, nil, web)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/style.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("style.css without token = %d, want 200", rec.Code)
	}
}

// TestAuthRejectsWithoutToken checks the gate is closed when a token is set.
func TestAuthRejectsWithoutToken(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{AuthToken: "s3cret"}, nil, web)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET / without token = %d, want 401", rec.Code)
	}
}

// TestAuthAcceptsToken forms: header, query (which sets a cookie), cookie.
func TestAuthAcceptsToken(t *testing.T) {
	web, _ := fs.Sub(webFS, "web")
	srv := NewServer(Config{AuthToken: "s3cret"}, nil, web)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Bearer token = %d, want 200", rec.Code)
	}

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

func TestSubtleEqual(t *testing.T) {
	if !subtleEqual("abc", "abc") {
		t.Error("equal strings should match")
	}
	if subtleEqual("abc", "abd") || subtleEqual("abc", "ab") || subtleEqual("", "x") {
		t.Error("different strings must not match")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"F-16C_50":          "plane",
		"F/A-18C":           "plane",
		"Su-27":             "plane",
		"AH-64D_BLK_II":     "heli",
		"Mi-24P":            "heli",
		"USS_Arleigh_Burke": "ship",
		"T-72B":             "ground",
		"SA-10":             "ground",
		"SomethingElse":     "other",
	}
	for typeID, want := range cases {
		if got := classify(typeID); got != want {
			t.Errorf("classify(%q) = %q, want %q", typeID, got, want)
		}
	}
}
