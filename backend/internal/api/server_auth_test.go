package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/category"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/live"
	"dcsmanager/internal/state"
	"dcsmanager/internal/stats"
)

func newServerTest(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	hash, err := auth.HashPassword("verysecret123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateUser(auth.User{
		Username:     "admin",
		DisplayName:  "Administrator",
		Role:         auth.RoleAdmin,
		Active:       true,
		PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{HTTPAddr: "0.0.0.0:0", Mode: "server"}
	s := New(cfg, state.New(time.Minute, 100), live.New(10, 10), database,
		stats.New(database, category.New("")), nil, nil, nil)
	s.SetVersion("test")
	s.SetMode("server")
	s.SetAuth(database)
	s.SetLocalOnly(false)
	return s, database
}

func TestServerAuthAndUploadFlow(t *testing.T) {
	s, database := newServerTest(t)
	handler := s.Handler()

	// Unauthenticated API access is refused.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats/overview", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated stats = %d", rec.Code)
	}

	// Login sets an HttpOnly session cookie.
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"verysecret123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login = %d %s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].HttpOnly {
		t.Fatalf("session cookie = %+v", cookies)
	}

	// The session endpoint reports the account.
	sessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	for _, cookie := range cookies {
		sessionReq.AddCookie(cookie)
	}
	sessionRec := httptest.NewRecorder()
	handler.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK || !strings.Contains(sessionRec.Body.String(), "admin") {
		t.Fatalf("session = %d %s", sessionRec.Code, sessionRec.Body.String())
	}

	// A desktop-only route is hidden, even with a session.
	maintReq := httptest.NewRequest(http.MethodGet, "/api/maintenance", nil)
	for _, cookie := range cookies {
		maintReq.AddCookie(cookie)
	}
	maintRec := httptest.NewRecorder()
	handler.ServeHTTP(maintRec, maintReq)
	if maintRec.Code != http.StatusNotFound {
		t.Fatalf("maintenance = %d", maintRec.Code)
	}

	// The local content manager is desktop-only too.
	contentReq := httptest.NewRequest(http.MethodGet, "/api/content/status", nil)
	for _, cookie := range cookies {
		contentReq.AddCookie(cookie)
	}
	contentRec := httptest.NewRecorder()
	handler.ServeHTTP(contentRec, contentReq)
	if contentRec.Code != http.StatusNotFound {
		t.Fatalf("content = %d", contentRec.Code)
	}

	// Create a personal token.
	tokenReq := httptest.NewRequest(http.MethodPost, "/api/auth/tokens",
		strings.NewReader(`{"name":"Desktop"}`))
	tokenReq.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		tokenReq.AddCookie(cookie)
	}
	tokenRec := httptest.NewRecorder()
	handler.ServeHTTP(tokenRec, tokenReq)
	if tokenRec.Code != http.StatusOK {
		t.Fatalf("token = %d %s", tokenRec.Code, tokenRec.Body.String())
	}
	var tokenBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenBody); err != nil {
		t.Fatal(err)
	}
	if tokenBody.Token == "" {
		t.Fatal("empty token")
	}

	// Upload one mission with the token, twice: the second call must not
	// duplicate it.
	now := time.Now().UnixMilli()
	payload := `{"clientId":"desk-test","missions":[{"externalId":7,"name":"Uploaded",` +
		`"theatre":"Caucasus","source":"live","startedAt":` + strconv.FormatInt(now-1000, 10) +
		`,"endedAt":` + strconv.FormatInt(now, 10) + `,` +
		`"players":[{"ucid":"u1","name":"Viper","killsAir":2,"landings":1}],` +
		`"debriefs":[{"externalId":1,"mission":"Uploaded","parsed":{"summary":{"kills":2}},"size":1,"createdAt":` +
		strconv.FormatInt(now, 10) + `}]}]}`
	for attempt := 0; attempt < 2; attempt++ {
		uploadReq := httptest.NewRequest(http.MethodPost, "/api/sync/upload", strings.NewReader(payload))
		uploadReq.Header.Set("Content-Type", "application/json")
		uploadReq.Header.Set("Authorization", "Bearer "+tokenBody.Token)
		uploadRec := httptest.NewRecorder()
		handler.ServeHTTP(uploadRec, uploadReq)
		if uploadRec.Code != http.StatusOK {
			t.Fatalf("upload %d = %d %s", attempt+1, uploadRec.Code, uploadRec.Body.String())
		}
	}
	missions, err := database.Missions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 || missions[0].Name != "Uploaded" {
		t.Fatalf("missions = %+v", missions)
	}
}

func TestServerRejectsBadPassword(t *testing.T) {
	s, _ := newServerTest(t)
	handler := s.Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin","password":"wrong-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d %s", rec.Code, rec.Body.String())
	}
}

func TestMemberCannotAdministrate(t *testing.T) {
	s, database := newServerTest(t)
	hash, err := auth.HashPassword("membersecret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateUser(auth.User{
		Username: "pilot", Role: auth.RoleMember, Active: true, PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"pilot","password":"membersecret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("member login = %d %s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()

	adminReq := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	for _, cookie := range cookies {
		adminReq.AddCookie(cookie)
	}
	adminRec := httptest.NewRecorder()
	handler.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusForbidden {
		t.Fatalf("member admin = %d %s", adminRec.Code, adminRec.Body.String())
	}
}

func loginCookies(t *testing.T, handler http.Handler, username, password string) []*http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s = %d %s", username, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}

func createToken(t *testing.T, handler http.Handler, cookies []*http.Cookie, name string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/tokens",
		strings.NewReader(`{"name":"`+name+`"}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token = %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Token
}

func TestServerDocumentSyncVisibility(t *testing.T) {
	s, database := newServerTest(t)
	hash, err := auth.HashPassword("membersecret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateUser(auth.User{
		Username: "pilot", Role: auth.RoleMember, Active: true, PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	adminToken := createToken(t, handler, loginCookies(t, handler, "admin", "verysecret123"), "admin-desktop")
	pilotToken := createToken(t, handler, loginCookies(t, handler, "pilot", "membersecret"), "pilot-desktop")

	push := func(token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/sync/push", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	route := `[{"ident":"A","kind":"airport","lat":1,"lon":2},{"ident":"B","kind":"airport","lat":3,"lon":4}]`
	adminBody := `{"clientId":"desk-admin","plans":[` +
		`{"uid":"fp_shared","name":"Shared","visibility":"squadron","updatedAt":1000,"revision":1,"flightRules":"VFR","route":` + route + `},` +
		`{"uid":"fp_private","name":"Private","visibility":"private","updatedAt":1000,"revision":1,"flightRules":"VFR","route":` + route + `}]}`
	if rec := push(adminToken, adminBody); rec.Code != http.StatusOK {
		t.Fatalf("admin push = %d %s", rec.Code, rec.Body.String())
	}

	pull := func(token string) []map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/api/sync/pull?since=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pull = %d %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Plans []map[string]any `json:"plans"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Plans
	}

	if plans := pull(pilotToken); len(plans) != 1 || plans[0]["uid"] != "fp_shared" {
		t.Fatalf("pilot pull = %+v", plans)
	}
	if plans := pull(adminToken); len(plans) != 2 {
		t.Fatalf("admin pull = %+v", plans)
	}

	// A member cannot overwrite the administrator's private plan.
	hijack := `{"clientId":"desk-pilot","plans":[` +
		`{"uid":"fp_private","name":"Hijack","visibility":"private","updatedAt":9000,"revision":2,"flightRules":"VFR","route":` + route + `}]}`
	rec := push(pilotToken, hijack)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"conflicts":1`) {
		t.Fatalf("hijack = %d %s", rec.Code, rec.Body.String())
	}
	if plans := pull(pilotToken); len(plans) != 1 {
		t.Fatalf("pilot still sees the private plan: %+v", plans)
	}
}

func TestPasswordResetAPI(t *testing.T) {
	s, database := newServerTest(t)
	hash, err := auth.HashPassword("oldpassword123")
	if err != nil {
		t.Fatal(err)
	}
	user, err := database.CreateUser(auth.User{
		Username: "pilot", Email: "pilot@example.com", Role: auth.RoleMember,
		Active: true, PasswordHash: hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	token := "reset-token-for-test"
	if err := database.CreatePasswordReset(user.ID, auth.HashToken(token), time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset",
		strings.NewReader(`{"token":"`+token+`","newPassword":"newpassword123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset = %d %s", rec.Code, rec.Body.String())
	}
	updated, err := database.UserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(updated.PasswordHash, "newpassword123") {
		t.Fatal("new password not stored")
	}

	// The token is single-use.
	reuse := httptest.NewRequest(http.MethodPost, "/api/auth/reset",
		strings.NewReader(`{"token":"`+token+`","newPassword":"anotherpassword123"}`))
	reuse.Header.Set("Content-Type", "application/json")
	reuseRec := httptest.NewRecorder()
	handler.ServeHTTP(reuseRec, reuse)
	if reuseRec.Code != http.StatusBadRequest {
		t.Fatalf("reused token = %d %s", reuseRec.Code, reuseRec.Body.String())
	}

	// Without SMTP, the forgot endpoint says so instead of pretending.
	forgot := httptest.NewRequest(http.MethodPost, "/api/auth/forgot",
		strings.NewReader(`{"identifier":"pilot"}`))
	forgot.Header.Set("Content-Type", "application/json")
	forgotRec := httptest.NewRecorder()
	handler.ServeHTTP(forgotRec, forgot)
	if forgotRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("forgot without SMTP = %d %s", forgotRec.Code, forgotRec.Body.String())
	}
}

func TestServerBackupDownload(t *testing.T) {
	s, _ := newServerTest(t)
	handler := s.Handler()
	cookies := loginCookies(t, handler, "admin", "verysecret123")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backup", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("backup = %d %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "zip") {
		t.Fatalf("content type = %q", contentType)
	}
	archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) == 0 || archive.File[0].Name != "dcsmanager.db" {
		t.Fatalf("archive = %+v", archive.File)
	}
}

func TestServerBackupRequiresAdmin(t *testing.T) {
	s, database := newServerTest(t)
	hash, err := auth.HashPassword("membersecret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateUser(auth.User{
		Username: "pilot", Role: auth.RoleMember, Active: true, PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backup", nil)
	for _, cookie := range loginCookies(t, handler, "pilot", "membersecret") {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member backup = %d", rec.Code)
	}
}
