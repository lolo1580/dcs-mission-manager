package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
