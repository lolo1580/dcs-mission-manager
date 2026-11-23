package db

import (
	"testing"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/school"
)

func schoolUser(t *testing.T, d *DB, name string) auth.User {
	t.Helper()
	u, err := d.CreateUser(auth.User{
		Username: name, DisplayName: name, PasswordHash: "x",
		Role: auth.RoleMember, Active: true,
	})
	if err != nil {
		t.Fatalf("create user %s: %v", name, err)
	}
	return u
}

func approveSchool(t *testing.T, d *DB, uid string) school.Item {
	t.Helper()
	it, err := d.ReviewSchoolItem(uid, 1, true, "")
	if err != nil {
		t.Fatalf("approve %s: %v", uid, err)
	}
	if it.Status != school.StatusPublished {
		t.Fatalf("status = %q, want published", it.Status)
	}
	return it
}

func TestSchoolTaxonomySeeded(t *testing.T) {
	d := openTestDB(t)
	list, err := d.SchoolTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 10 {
		t.Fatalf("taxonomy has %d entries, want the defaults", len(list))
	}
	var avions bool
	for _, entry := range list {
		if entry.Kind == "domain" && entry.Slug == "avions" {
			avions = true
		}
	}
	if !avions {
		t.Fatal("the default avions domain is missing")
	}
}

func TestSchoolModerationAndVisibility(t *testing.T) {
	d := openTestDB(t)
	author := schoolUser(t, d, "author")
	reader := schoolUser(t, d, "reader")

	it, err := d.CreateSchoolItem(author.ID, school.Item{
		Kind:    school.KindCard,
		Title:   "F-16C",
		Summary: "Chasseur multirôle",
		Domain:  "avions",
		Era:     "coldwar",
	})
	if err != nil {
		t.Fatal(err)
	}
	if it.Status != school.StatusPending {
		t.Fatalf("new content status = %q, want pending", it.Status)
	}

	if list, _ := d.SchoolItems(school.Filter{ViewerID: reader.ID}); len(list) != 0 {
		t.Fatalf("reader sees %d pending items, want 0", len(list))
	}
	if list, _ := d.SchoolItems(school.Filter{ViewerID: author.ID}); len(list) != 1 {
		t.Fatalf("author sees %d of their own items, want 1", len(list))
	}
	if list, _ := d.SchoolItems(school.Filter{ViewerID: 1, Admin: true, Status: school.StatusPending}); len(list) != 1 {
		t.Fatalf("admin pending queue = %d, want 1", len(list))
	}
	if _, err := d.SchoolItem(it.UID, reader.ID, false); err == nil {
		t.Fatal("reader must not read a pending item")
	}

	approveSchool(t, d, it.UID)
	if list, _ := d.SchoolItems(school.Filter{ViewerID: reader.ID}); len(list) != 1 {
		t.Fatalf("reader sees %d published items, want 1", len(list))
	}
	if _, err := d.SchoolItem(it.UID, reader.ID, false); err != nil {
		t.Fatalf("reader cannot read the published item: %v", err)
	}
}

func TestSchoolQuizAttemptAndLeaderboard(t *testing.T) {
	d := openTestDB(t)
	alice := schoolUser(t, d, "alice")

	quiz, err := d.CreateSchoolItem(alice.ID, school.Item{
		Kind:   school.KindQuiz,
		Title:  "Culture aéronautique",
		Domain: "avions",
		Questions: []school.Question{
			{Prompt: "Quel avion ?", Choices: []string{"F-16", "Su-27"}, CorrectIndex: school.Index(0)},
			{Prompt: "Quel radar ?", Choices: []string{"APG-68", "RWR"}, CorrectIndex: school.Index(0)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	approveSchool(t, d, quiz.UID)

	// The public item hides answers.
	loaded, err := d.SchoolItem(quiz.UID, alice.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Questions[0].CorrectIndex == nil {
		t.Fatal("the store must keep the answer index for server-side scoring")
	}

	attempt, err := d.SubmitSchoolAttempt(alice.ID, quiz.UID, []int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Score != 1 || attempt.MaxScore != 2 {
		t.Fatalf("score = %d/%d, want 1/2", attempt.Score, attempt.MaxScore)
	}
	if len(attempt.Correction) != 2 {
		t.Fatalf("correction has %d questions, want 2", len(attempt.Correction))
	}
	if history, _ := d.SchoolAttempts(alice.ID, quiz.UID, 10); len(history) != 1 {
		t.Fatalf("attempt history = %d, want 1", len(history))
	}

	board, err := d.SchoolLeaderboard(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 1 {
		t.Fatalf("leaderboard has %d entries, want 1", len(board))
	}
	if board[0].QuizPoints != 50 {
		t.Fatalf("quiz points = %d, want 50", board[0].QuizPoints)
	}
	if board[0].Points != 50 {
		t.Fatalf("points = %d, want 50", board[0].Points)
	}
}

func TestSchoolProgressAndLinks(t *testing.T) {
	d := openTestDB(t)
	author := schoolUser(t, d, "author")

	target, err := d.CreateSchoolItem(author.ID, school.Item{Kind: school.KindCard, Title: "Radar APG-68"})
	if err != nil {
		t.Fatal(err)
	}
	target = approveSchool(t, d, target.UID)

	lesson, err := d.CreateSchoolItem(author.ID, school.Item{
		Kind: school.KindLesson, Title: "Démarrage F-16", Links: []string{target.UID},
	})
	if err != nil {
		t.Fatal(err)
	}
	lesson = approveSchool(t, d, lesson.UID)

	if len(lesson.Links) != 1 || lesson.Links[0] != target.UID {
		t.Fatalf("links = %#v, want the radar", lesson.Links)
	}
	back, err := d.SchoolItem(target.UID, author.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Backlinks) != 1 || back.Backlinks[0] != lesson.UID {
		t.Fatalf("backlinks = %#v, want the lesson", back.Backlinks)
	}

	if err := d.SetSchoolProgress(author.ID, lesson.UID, school.ProgressCompleted, 0); err != nil {
		t.Fatal(err)
	}
	progress, err := d.SchoolProgress(author.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || progress[0].Status != school.ProgressCompleted || progress[0].Percent != 100 {
		t.Fatalf("progress = %#v", progress)
	}
	board, _ := d.SchoolLeaderboard(10)
	if len(board) != 1 || board[0].Lessons != 1 || board[0].Points != 10 {
		t.Fatalf("leaderboard = %#v, want one completed lesson", board)
	}
}

func TestSchoolRevisionNeedsApproval(t *testing.T) {
	d := openTestDB(t)
	author := schoolUser(t, d, "author")
	it, err := d.CreateSchoolItem(author.ID, school.Item{Kind: school.KindCard, Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	approveSchool(t, d, it.UID)

	updated, err := d.UpdateSchoolItem(it.UID, author.ID, false, school.Item{
		Kind: school.KindCard, Title: "Révisé",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RevisionPending {
		t.Fatal("editing a published item should create a pending revision")
	}
	live, _ := d.SchoolItem(it.UID, 0, true)
	if live.Title != "Original" {
		t.Fatalf("published title changed before approval: %q", live.Title)
	}
	approved, err := d.ReviewSchoolItem(it.UID, 1, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Title != "Révisé" {
		t.Fatalf("approved title = %q, want Révisé", approved.Title)
	}
	if approved.RevisionPending {
		t.Fatal("revision should no longer be pending")
	}
}
