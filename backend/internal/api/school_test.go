package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/db"
	"dcsmanager/internal/school"
)

func openSchoolServer(t *testing.T) (*Server, *db.DB, auth.User, auth.User, auth.User) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "school.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	admin, err := database.CreateUser(auth.User{
		Username: "admin", DisplayName: "Admin", PasswordHash: "x",
		Role: auth.RoleAdmin, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	member, err := database.CreateUser(auth.User{
		Username: "member", DisplayName: "Member", PasswordHash: "x",
		Role: auth.RoleMember, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := database.CreateUser(auth.User{
		Username: "other", DisplayName: "Other", PasswordHash: "x",
		Role: auth.RoleMember, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.SetAuth(database)
	s.SetMode("server")
	return s, database, admin, member, other
}

func schoolReq(method, path string, body any, user auth.User) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(context.WithValue(req.Context(), authContextKey{}, user))
}

func decodeItem(t *testing.T, rec *httptest.ResponseRecorder) school.Item {
	t.Helper()
	var body struct {
		Item school.Item `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode item: %v (%s)", err, rec.Body.String())
	}
	return body.Item
}

func TestSchoolIsServerOnly(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleSchool(rec, httptest.NewRequest(http.MethodGet, "/api/school/items", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("desktop school endpoint = %d, want 404", rec.Code)
	}
}

func TestSchoolModerationFlow(t *testing.T) {
	s, _, admin, member, other := openSchoolServer(t)

	rec := httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodPost, "/api/school/items", school.Item{
		Kind: school.KindCard, Title: "F-16C", Summary: "Chasseur", Domain: "avions", Era: "coldwar",
	}, member))
	if rec.Code != http.StatusOK {
		t.Fatalf("create = %d (%s)", rec.Code, rec.Body.String())
	}
	item := decodeItem(t, rec)
	if item.Status != school.StatusPending || item.UID == "" {
		t.Fatalf("created item = %#v", item)
	}

	// The author sees their pending item; another member does not.
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/items", nil, member))
	var list struct {
		Count int           `json:"count"`
		Items []school.Item `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Count != 1 {
		t.Fatalf("author list = %d, want 1", list.Count)
	}
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/items", nil, other))
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Count != 0 {
		t.Fatalf("other member list = %d, want 0", list.Count)
	}

	// The admin queue sees it and approves it.
	rec = httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodGet, "/api/admin/school/pending", nil, admin))
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Count != 1 {
		t.Fatalf("pending queue = %d, want 1", list.Count)
	}
	rec = httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodPost,
		"/api/admin/school/items/"+item.UID+"/review",
		map[string]string{"decision": "approve"}, admin))
	if rec.Code != http.StatusOK {
		t.Fatalf("review = %d (%s)", rec.Code, rec.Body.String())
	}
	approved := decodeItem(t, rec)
	if approved.Status != school.StatusPublished {
		t.Fatalf("approved status = %q", approved.Status)
	}

	// It is now visible to everyone.
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/items/"+item.UID, nil, other))
	if rec.Code != http.StatusOK {
		t.Fatalf("published item = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestSchoolQuizHidesAnswersAndScores(t *testing.T) {
	s, _, admin, member, other := openSchoolServer(t)

	rec := httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodPost, "/api/school/items", school.Item{
		Kind: school.KindQuiz, Title: "Quiz avions", Domain: "avions",
		Questions: []school.Question{
			{Prompt: "F-16 ?", Choices: []string{"US", "RU"}, CorrectIndex: school.Index(0), Explanation: "USAF"},
			{Prompt: "Su-27 ?", Choices: []string{"US", "RU"}, CorrectIndex: school.Index(1)},
		},
	}, member))
	if rec.Code != http.StatusOK {
		t.Fatalf("create quiz = %d (%s)", rec.Code, rec.Body.String())
	}
	quiz := decodeItem(t, rec)
	rec = httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodPost,
		"/api/admin/school/items/"+quiz.UID+"/review",
		map[string]string{"decision": "approve"}, admin))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve quiz = %d (%s)", rec.Code, rec.Body.String())
	}

	// A member reading the quiz must not receive the answers.
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/items/"+quiz.UID, nil, other))
	public := decodeItem(t, rec)
	if len(public.Questions) != 2 {
		t.Fatalf("public quiz has %d questions", len(public.Questions))
	}
	for _, q := range public.Questions {
		if q.CorrectIndex != nil || q.Explanation != "" {
			t.Fatalf("public quiz leaks an answer: %#v", q)
		}
	}

	// The server scores the attempt.
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodPost,
		"/api/school/quizzes/"+quiz.UID+"/attempts",
		map[string][]int{"answers": {0, 1}}, other))
	if rec.Code != http.StatusOK {
		t.Fatalf("attempt = %d (%s)", rec.Code, rec.Body.String())
	}
	var attempt struct {
		Attempt school.Attempt `json:"attempt"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &attempt)
	if attempt.Attempt.Score != 2 || attempt.Attempt.MaxScore != 2 {
		t.Fatalf("attempt = %d/%d, want 2/2", attempt.Attempt.Score, attempt.Attempt.MaxScore)
	}
	if len(attempt.Attempt.Correction) != 2 {
		t.Fatalf("correction has %d questions", len(attempt.Attempt.Correction))
	}
	if attempt.Attempt.Correction[0].CorrectIndex == nil {
		t.Fatal("the correction should include answers")
	}

	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/leaderboard", nil, other))
	var board struct {
		Leaderboard []school.LeaderboardEntry `json:"leaderboard"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &board)
	if len(board.Leaderboard) != 3 {
		t.Fatalf("leaderboard has %d entries, want 3 active accounts", len(board.Leaderboard))
	}
	if board.Leaderboard[0].Username != "other" || board.Leaderboard[0].QuizPoints != 100 {
		t.Fatalf("leaderboard top = %#v", board.Leaderboard[0])
	}
}

func TestSchoolProgress(t *testing.T) {
	s, _, admin, member, other := openSchoolServer(t)

	rec := httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodPost, "/api/school/items", school.Item{
		Kind: school.KindLesson, Title: "Démarrage",
	}, member))
	lesson := decodeItem(t, rec)
	rec = httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodPost,
		"/api/admin/school/items/"+lesson.UID+"/review",
		map[string]string{"decision": "approve"}, admin))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve lesson = %d (%s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodPost,
		"/api/school/items/"+lesson.UID+"/progress",
		map[string]any{"status": school.ProgressCompleted}, other))
	if rec.Code != http.StatusOK {
		t.Fatalf("progress = %d (%s)", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/progress", nil, other))
	var body struct {
		Progress []school.Progress `json:"progress"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Progress) != 1 || body.Progress[0].Status != school.ProgressCompleted || body.Progress[0].Percent != 100 {
		t.Fatalf("progress = %#v", body.Progress)
	}
}

func TestSchoolTaxonomyAdmin(t *testing.T) {
	s, _, admin, member, _ := openSchoolServer(t)

	// A member cannot modify the taxonomy.
	rec := httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodPost, "/api/admin/school/taxonomy", school.Taxonomy{
		Kind: "domain", Slug: "test", LabelFR: "Test", LabelEN: "Test",
	}, member))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member taxonomy = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleAdminSchool(rec, schoolReq(http.MethodPost, "/api/admin/school/taxonomy", school.Taxonomy{
		Kind: "domain", Slug: "test", LabelFR: "Test", LabelEN: "Test", Position: 5,
	}, admin))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin taxonomy = %d (%s)", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleSchool(rec, schoolReq(http.MethodGet, "/api/school/taxonomy", nil, member))
	if rec.Code != http.StatusOK {
		t.Fatalf("taxonomy list = %d", rec.Code)
	}
}
