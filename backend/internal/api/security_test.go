package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/config"
)

// TestOriginGuard covers the protection of an API that has a destructive
// endpoint and no authentication. A page on another site can POST to
// http://127.0.0.1:8080 without a preflight, so the Origin must be checked; and
// because Origin and Host agree under DNS rebinding, the Host must be local too.
func TestOriginGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name      string
		origin    string
		host      string
		localOnly bool
		want      int
	}{
		{"same origin is allowed", "http://127.0.0.1:8080", "127.0.0.1:8080", true, http.StatusOK},
		{"localhost is allowed", "http://localhost:8080", "localhost:8080", true, http.StatusOK},
		{"another site is refused", "https://evil.example", "127.0.0.1:8080", true, http.StatusForbidden},
		{"a lookalike port is refused", "http://127.0.0.1:9999", "127.0.0.1:8080", true, http.StatusForbidden},
		{"no origin is allowed (curl, CLI)", "", "127.0.0.1:8080", true, http.StatusOK},
		{"a foreign Host is refused (DNS rebinding)", "http://evil.example", "evil.example", true, http.StatusForbidden},
		{"IPv6 loopback is allowed", "", "[::1]:8080", true, http.StatusOK},
		// When the operator deliberately binds to the network, the Host check
		// must stand down: they asked for remote access.
		{"foreign Host allowed when not localOnly", "", "192.168.1.20:8080", false, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{localOnly: tc.localOnly}
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/maintenance/purge", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			s.originGuard(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// TestIsLoopbackAddr checks the decision that drives localOnly.
func TestIsLoopbackAddr(t *testing.T) {
	local := []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "127.0.0.1:0"}
	for _, a := range local {
		if !isLoopbackAddr(a) {
			t.Errorf("isLoopbackAddr(%q) should be true", a)
		}
	}
	exposed := []string{"", "0.0.0.0:8080", ":8080", "192.168.1.10:8080", "[::]:8080"}
	for _, a := range exposed {
		if isLoopbackAddr(a) {
			t.Errorf("isLoopbackAddr(%q) should be false: it accepts remote traffic", a)
		}
	}
}

// TestTokenGuard covers the opt-in protection for exposing the manager beyond
// loopback. The point is that local callers stay exempt (nothing to configure)
// while remote callers must present the token.
func TestTokenGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name     string
		token    string // DCSMANAGER_API_TOKEN
		remote   string // RemoteAddr
		header   string
		query    string
		cookie   string
		want     int
		wantCook bool
	}{
		{name: "no token configured: open", token: "", remote: "203.0.113.9:1234", want: http.StatusOK},
		{name: "loopback is exempt", token: "s3cret", remote: "127.0.0.1:5000", want: http.StatusOK},
		{name: "IPv6 loopback is exempt", token: "s3cret", remote: "[::1]:5000", want: http.StatusOK},
		{name: "remote without token is refused", token: "s3cret", remote: "203.0.113.9:1234", want: http.StatusUnauthorized},
		{name: "remote with wrong token is refused", token: "s3cret", remote: "203.0.113.9:1234", header: "Bearer nope", want: http.StatusUnauthorized},
		{name: "remote with bearer is allowed", token: "s3cret", remote: "203.0.113.9:1234", header: "Bearer s3cret", want: http.StatusOK},
		{name: "remote with query token is allowed and sets a cookie", token: "s3cret", remote: "203.0.113.9:1234", query: "?token=s3cret", want: http.StatusOK, wantCook: true},
		{name: "remote with cookie is allowed", token: "s3cret", remote: "203.0.113.9:1234", cookie: "s3cret", want: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{cfg: config.Config{APIToken: tc.token}}
			req := httptest.NewRequest(http.MethodGet, "http://192.168.1.20:8080/api/stats/overview"+tc.query, nil)
			req.RemoteAddr = tc.remote
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "dcsmanager_token", Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			s.tokenGuard(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
			if tc.wantCook {
				found := false
				for _, c := range rec.Result().Cookies() {
					if c.Name == "dcsmanager_token" {
						found = true
					}
				}
				if !found {
					t.Error("?token= should set a dcsmanager_token cookie")
				}
			}
		})
	}
}
