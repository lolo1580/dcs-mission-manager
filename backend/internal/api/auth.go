package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/db"
	"dcsmanager/internal/mailer"
)

const (
	sessionCookie   = "dcsm_session"
	sessionDuration = 7 * 24 * time.Hour
)

type authContextKey struct{}

// ---- Middleware -----------------------------------------------------------

// authMiddleware protects the server profile. Static files and the handful of
// endpoints needed to render the login page stay public; everything else needs
// a session, except data upload which uses a personal API token.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "accounts are not available"})
			return
		}
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		switch path {
		case "/api/health", "/api/auth/login", "/api/auth/logout", "/api/auth/session",
			"/api/auth/forgot", "/api/auth/reset":
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/sync/conflicts") {
			user, ok := s.sessionUser(r)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, user)))
			return
		}
		if strings.HasPrefix(path, "/api/sync/") {
			user, ok := s.tokenUser(w, r)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, user)))
			return
		}
		user, ok := s.sessionUser(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, user)))
	})
}

// desktopGuard hides the endpoints that only make sense on the machine running
// DCS. The server profile must never expose the local purge, backup, script
// installer or live telemetry APIs, even to an authenticated member.
func (s *Server) desktopGuard(next http.Handler) http.Handler {
	if s.Mode() != "server" {
		return next
	}
	blocked := map[string]bool{
		"/api/state":             true,
		"/api/events":            true,
		"/api/game-events":       true,
		"/api/chat":              true,
		"/api/players":           true,
		"/api/mission":           true,
		"/api/modules":           true,
		"/api/mods":              true,
		"/api/career":            true,
		"/api/backup":            true,
		"/api/backup/categories": true,
		"/api/scripts":           true,
		"/api/debug":             true,
		"/api/log":               true,
		"/api/maintenance":       true,
	}
	blockedPrefixes := []string{
		"/api/units/",
		"/api/backup/",
		"/api/scripts/",
		"/api/maintenance/",
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if blocked[path] {
			http.NotFound(w, r)
			return
		}
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(path, prefix) {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sessionUser(r *http.Request) (auth.User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return auth.User{}, false
	}
	user, err := s.auth.SessionUser(auth.HashToken(cookie.Value), time.Now().UnixMilli())
	if err != nil {
		return auth.User{}, false
	}
	return user, true
}

func (s *Server) tokenUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "a personal API token is required"})
		return auth.User{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "empty API token"})
		return auth.User{}, false
	}
	hash := auth.HashToken(token)
	user, err := s.auth.APITokenUser(hash, time.Now().UnixMilli())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API token"})
		return auth.User{}, false
	}
	_ = s.auth.TouchAPIToken(hash)
	return user, true
}

func currentUser(r *http.Request) (auth.User, bool) {
	user, ok := r.Context().Value(authContextKey{}).(auth.User)
	return user, ok
}

// canEditDocument reports whether an account may modify a plan or checklist.
// Administrators and the owner may; an unowned legacy document stays editable
// by the squadron until it is claimed. The desktop is administrator by
// construction (ownerID 0, admin true).
func canEditDocument(ownerID int64, admin bool, documentOwner int64) bool {
	if admin || documentOwner == 0 {
		return true
	}
	return ownerID != 0 && documentOwner == ownerID
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := currentUser(r)
	if !ok || user.Role != auth.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "administrator access required"})
		return auth.User{}, false
	}
	return user, true
}

// ---- Login and sessions ---------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	key := loginKey(r, req.Username)
	if !s.login.allow(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again in a few minutes"})
		return
	}

	user, err := s.auth.UserByUsername(req.Username)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!user.Active || !auth.VerifyPassword(user.PasswordHash, req.Password))) {
		s.login.fail(key)
		_ = s.auth.Audit(0, "login_failed", req.Username)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.login.reset(key)

	if err := s.startSession(w, r, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.auth.TouchLogin(user.ID)
	_ = s.auth.Audit(user.ID, "login", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user auth.User) error {
	token, err := auth.NewToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionDuration).UnixMilli()
	if err := s.auth.CreateSession(auth.HashToken(token), user.ID, expires, r.UserAgent()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})
	return nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" {
		http.NotFound(w, r)
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" && s.auth != nil {
		hash := auth.HashToken(cookie.Value)
		if user, err := s.auth.SessionUser(hash, time.Now().UnixMilli()); err == nil {
			_ = s.auth.Audit(user.ID, "logout", r.RemoteAddr)
		}
		_ = s.auth.DeleteSession(hash)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Mode() != "server" {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "mode": "desktop", "authenticated": true})
		return
	}
	user, ok := s.sessionUser(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "mode": "server", "authenticated": false, "user": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "mode": "server", "authenticated": true, "user": user})
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if !auth.VerifyPassword(user.PasswordHash, req.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.SetPassword(user.ID, hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Changing a password invalidates every existing session, including this
	// one; a fresh session is issued right away so the browser stays signed in.
	if err := s.auth.DeleteUserSessions(user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.startSession(w, r, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.auth.Audit(user.ID, "password_changed", "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleForgotPassword starts the email reset flow. It always answers 200 when
// email is configured, so an attacker cannot enumerate accounts.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if !s.cfg.EmailRecovery() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "email recovery is not configured"})
		return
	}
	var req struct {
		Identifier string `json:"identifier"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	key := "reset|" + requestHost(r)
	if !s.login.allow(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests, try again later"})
		return
	}
	s.login.fail(key)

	identifier := strings.TrimSpace(req.Identifier)
	user, err := s.auth.UserByUsername(identifier)
	if errors.Is(err, sql.ErrNoRows) {
		user, err = s.auth.UserByEmail(identifier)
	}
	if err != nil || !user.Active || user.Email == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ttl := s.cfg.PasswordResetTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	now := time.Now()
	if err := s.auth.CreatePasswordReset(user.ID, auth.HashToken(token), now.Add(ttl).UnixMilli()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	base := strings.TrimRight(s.cfg.PublicURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	link := base + "/reset?token=" + url.QueryEscape(token)
	sender := mailer.Sender{
		Host: s.cfg.SMTPHost, Port: s.cfg.SMTPPort,
		User: s.cfg.SMTPUser, Password: s.cfg.SMTPPassword, From: s.cfg.SMTPFrom,
	}
	if err := sender.SendPasswordReset(user.Email, user.Username, link, ttl); err != nil {
		// Do not leak whether the address exists; the administrator sees the log.
		log.Printf("password reset: send to %s: %v", user.Username, err)
	}
	_ = s.auth.Audit(user.ID, "password_reset_requested", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleResetPassword consumes a single-use token and sets the new password.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if !auth.ValidPassword(req.NewPassword) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 10 characters"})
		return
	}
	hash := auth.HashToken(strings.TrimSpace(req.Token))
	user, err := s.auth.PasswordResetUser(hash, time.Now().UnixMilli())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired reset link"})
		return
	}
	encoded, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.SetPassword(user.ID, encoded); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.DeleteUserSessions(user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.UsePasswordReset(hash, time.Now().UnixMilli()); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reset link was already used"})
		return
	}
	_ = s.auth.Audit(user.ID, "password_reset", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func requestHost(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

// handleProfile lets a signed-in member change their display name and the DCS
// pilot names attached to their account. Role and status stay administrator
// decisions.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPut)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use PUT"})
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var req struct {
		DisplayName string   `json:"displayName"`
		Email       string   `json:"email"`
		DCSNames    []string `json:"dcsNames"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	email := strings.TrimSpace(req.Email)
	if email != "" && !auth.ValidEmail(email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email address"})
		return
	}
	if err := s.auth.UpdateUser(user.ID, strings.TrimSpace(req.DisplayName), user.Role, user.Active, cleanNames(req.DCSNames), ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.SetEmail(user.ID, email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	updated, err := s.auth.UserByID(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.auth.Audit(user.ID, "profile_updated", "")
	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// ---- Personal API tokens --------------------------------------------------
func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth/tokens"), "/")
	if path != "" {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE"})
			return
		}
		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid token id"})
			return
		}
		if err := s.auth.DeleteAPIToken(user.ID, id); errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
			return
		} else if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := s.auth.APITokens(user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if list == nil {
			list = []auth.APIToken{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"tokens": list})
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" || len(name) > 80 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "give the token a name (80 characters maximum)"})
			return
		}
		token, err := auth.NewToken()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		saved, err := s.auth.CreateAPIToken(user.ID, name, auth.HashToken(token))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = s.auth.Audit(user.ID, "token_created", name)
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "apiToken": saved})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

// ---- Administration -------------------------------------------------------

type userPayload struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Active      *bool    `json:"active"`
	DCSNames    []string `json:"dcsNames"`
	Password    string   `json:"password"`
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users"), "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			list, err := s.auth.Users()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if list == nil {
				list = []auth.User{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"users": list})
		case http.MethodPost:
			var req userPayload
			if err := decodeJSON(w, r, &req); err != nil {
				return
			}
			created, err := s.createUser(req)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			_ = s.auth.Audit(actor.ID, "user_created", created.Username)
			writeJSON(w, http.StatusOK, created)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		}
		return
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	switch r.Method {
	case http.MethodPut, http.MethodPost:
		var req userPayload
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		updated, err := s.updateUser(actor, id, req)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, sql.ErrNoRows) {
				status = http.StatusNotFound
			} else if errors.Is(err, auth.ErrLastAdmin) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		_ = s.auth.Audit(actor.ID, "user_updated", updated.Username)
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if id == actor.ID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "you cannot delete your own account"})
			return
		}
		target, err := s.auth.UserByID(id)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		} else if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if target.Role == auth.RoleAdmin && target.Active {
			admins, err := s.auth.CountActiveAdmins()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if admins <= 1 {
				writeJSON(w, http.StatusConflict, map[string]string{"error": auth.ErrLastAdmin.Error()})
				return
			}
		}
		if err := s.auth.DeleteUser(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = s.auth.Audit(actor.ID, "user_deleted", target.Username)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use PUT or DELETE"})
	}
}

func (s *Server) createUser(req userPayload) (auth.User, error) {
	username := strings.TrimSpace(req.Username)
	if !auth.ValidUsername(username) {
		return auth.User{}, errors.New("username must be 3-32 characters: letters, digits, dot, dash or underscore")
	}
	if !auth.ValidPassword(req.Password) {
		return auth.User{}, errors.New("password must be at least 10 characters")
	}
	email := strings.TrimSpace(req.Email)
	if email != "" && !auth.ValidEmail(email) {
		return auth.User{}, errors.New("invalid email address")
	}
	role := req.Role
	if role == "" {
		role = auth.RoleMember
	}
	if !auth.ValidRole(role) {
		return auth.User{}, errors.New("role must be admin or member")
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return auth.User{}, err
	}
	return s.auth.CreateUser(auth.User{
		Username:     username,
		DisplayName:  strings.TrimSpace(req.DisplayName),
		Email:        email,
		Role:         role,
		Active:       true,
		DCSNames:     cleanNames(req.DCSNames),
		PasswordHash: hash,
	})
}

func (s *Server) updateUser(actor auth.User, id int64, req userPayload) (auth.User, error) {
	existing, err := s.auth.UserByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, sql.ErrNoRows
	}
	if err != nil {
		return auth.User{}, err
	}
	role := req.Role
	if role == "" {
		role = existing.Role
	}
	if !auth.ValidRole(role) {
		return auth.User{}, errors.New("role must be admin or member")
	}
	active := existing.Active
	if req.Active != nil {
		active = *req.Active
	}
	if id == actor.ID && !active {
		return auth.User{}, errors.New("you cannot deactivate your own account")
	}
	losingAdmin := existing.Role == auth.RoleAdmin && existing.Active && (role != auth.RoleAdmin || !active)
	if losingAdmin {
		admins, err := s.auth.CountActiveAdmins()
		if err != nil {
			return auth.User{}, err
		}
		if admins <= 1 {
			return auth.User{}, auth.ErrLastAdmin
		}
	}
	hash := ""
	if req.Password != "" {
		hash, err = auth.HashPassword(req.Password)
		if err != nil {
			return auth.User{}, err
		}
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = existing.DisplayName
	}
	names := req.DCSNames
	if names == nil {
		names = existing.DCSNames
	}
	email := strings.TrimSpace(req.Email)
	if email != "" && !auth.ValidEmail(email) {
		return auth.User{}, errors.New("invalid email address")
	}
	if err := s.auth.UpdateUser(id, displayName, role, active, cleanNames(names), hash); err != nil {
		return auth.User{}, err
	}
	if email != "" {
		if err := s.auth.SetEmail(id, email); err != nil {
			return auth.User{}, err
		}
	}
	if hash != "" || !active {
		_ = s.auth.DeleteUserSessions(id)
	}
	return s.auth.UserByID(id)
}

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	limit := 100
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	entries, err := s.auth.AuditEntries(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []db.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func cleanNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// ---- JSON helper ----------------------------------------------------------

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed request"})
		return err
	}
	return nil
}

// ---- Login throttle -------------------------------------------------------

type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: make(map[string][]time.Time)}
}

func loginKey(r *http.Request, username string) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.ToLower(host + "|" + username)
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	var list []time.Time
	for _, at := range l.failures[key] {
		if at.After(cutoff) {
			list = append(list, at)
		}
	}
	l.failures[key] = list
	return len(list) < 5
}

func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.failures[key], time.Now())
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}
