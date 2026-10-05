package api

import (
	"crypto/subtle"
	"net"
	"net/http"
)

// tokenGuard requires DCSMANAGER_API_TOKEN from non-loopback callers when a token
// is configured.
//
// The manager's API has no authentication by design: it listens on loopback, and
// the only way to reach it is to be on the machine. The documented opt-in to
// expose it (a statistics plugin on another host, a tablet) breaks that
// assumption, and the API also carries a destructive purge endpoint. This guard
// restores the protection for exactly that case, without changing local use:
//
//   - loopback callers (the native window, curl on the machine) are exempt, so
//     nothing has to be configured for the normal, local setup;
//   - remote callers must present the token as an Authorization: Bearer header
//     or a ?token= query parameter, which also sets a cookie for the browser's
//     later asset and API requests;
//   - when no token is configured the guard is a pass-through, preserving the
//     current behaviour.
//
// It wraps the whole router (the UI and the API alike), because a remote caller
// who may read the statistics may also load the page that shows them. A browser
// reaches it once through ?token=, which sets the cookie used by every later
// asset and API request.
func (s *Server) tokenGuard(next http.Handler) http.Handler {
	if s.cfg.APIToken == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoopbackRemote(r.RemoteAddr) || s.tokenValid(w, r) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})
}

// tokenValid reports whether the request carries the configured token, and sets
// the convenience cookie when it arrives through ?token=.
func (s *Server) tokenValid(w http.ResponseWriter, r *http.Request) bool {
	want := s.cfg.APIToken

	if c, err := r.Cookie("dcsmanager_token"); err == nil && constantTimeEqual(c.Value, want) {
		return true
	}
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		if constantTimeEqual(h[7:], want) {
			return true
		}
	}
	if q := r.URL.Query().Get("token"); q != "" && constantTimeEqual(q, want) {
		http.SetCookie(w, &http.Cookie{
			Name:     "dcsmanager_token",
			Value:    want,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   30 * 24 * 3600,
		})
		return true
	}
	return false
}

// isLoopbackRemote reports whether the connection originates from the local
// machine. An empty RemoteAddr (a hand-built request in a test) is treated as
// remote, so the guard is exercised rather than silently skipped.
func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// constantTimeEqual compares two secrets without leaking their length or prefix
// through timing on the common path.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
