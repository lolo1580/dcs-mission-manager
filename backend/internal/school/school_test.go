package school

import (
	"strings"
	"testing"
)

func TestValidateItem(t *testing.T) {
	it := Item{
		Kind:    KindCard,
		Title:   "  F-16C  ",
		Summary: "Chasseur multirôle",
		Tags:    []string{" avion ", "USAF", "avion"},
		Specs:   []Spec{{Label: "Vitesse", Value: "2 120", Unit: "km/h"}},
		Sources: []Source{{Label: "USAF", URL: "https://www.af.mil"}},
	}
	if err := ValidateItem(&it); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	if it.Title != "F-16C" {
		t.Fatalf("title not trimmed: %q", it.Title)
	}
	if len(it.Tags) != 2 || it.Tags[0] != "avion" || it.Tags[1] != "USAF" {
		t.Fatalf("tags not normalised: %#v", it.Tags)
	}
}

func TestValidateItemRejectsBadInput(t *testing.T) {
	cases := []Item{
		{Kind: "unknown", Title: "x"},
		{Kind: KindCard, Title: ""},
		{Kind: KindCard, Title: "x", ImageURL: "javascript:alert(1)"},
		{Kind: KindCard, Title: "x", Sources: []Source{{Label: "x", URL: "ftp://example.com"}}},
		{Kind: KindCard, Title: "x", ServiceStart: 2000, ServiceEnd: 1990},
		{Kind: KindQuiz, Title: "Quiz", Questions: nil},
	}
	for i, it := range cases {
		if err := ValidateItem(&it); err == nil {
			t.Fatalf("case %d should have been rejected", i)
		}
	}
}

func TestValidateQuestionsAndScore(t *testing.T) {
	questions := []Question{
		{Prompt: "Capitale de la France ?", Choices: []string{"Paris", "Lyon"}, CorrectIndex: Index(0)},
		{Prompt: "2 + 2 ?", Choices: []string{"3", "4"}, CorrectIndex: Index(1)},
	}
	if err := ValidateQuestions(questions); err != nil {
		t.Fatal(err)
	}
	if questions[0].Position != 0 || questions[1].Position != 1 {
		t.Fatalf("positions not assigned: %#v", questions)
	}
	score, max := Score(questions, []int{0, 1})
	if score != 2 || max != 2 {
		t.Fatalf("score = %d/%d, want 2/2", score, max)
	}
	score, _ = Score(questions, []int{0})
	if score != 1 {
		t.Fatalf("missing answer should be wrong, score = %d", score)
	}
	score, _ = Score(questions, []int{1, 0})
	if score != 0 {
		t.Fatalf("wrong answers scored %d", score)
	}
}

func TestQuestionPublicHidesAnswer(t *testing.T) {
	q := Question{Prompt: "x", Choices: []string{"a", "b"}, CorrectIndex: Index(1), Explanation: "because"}
	pub := q.Public()
	if pub.CorrectIndex != nil || pub.Explanation != "" {
		t.Fatalf("public question leaks the answer: %#v", pub)
	}
}

func TestPoints(t *testing.T) {
	if got := Points(2, 3, 80); got != 20+6+80 {
		t.Fatalf("points = %d", got)
	}
}

func TestValidSlug(t *testing.T) {
	for _, ok := range []string{"avions", "guerre-electronique", "ww2"} {
		if !ValidSlug(ok) {
			t.Fatalf("%q should be a valid slug", ok)
		}
	}
	for _, bad := range []string{"Avions", "../x", "", "a b"} {
		if ValidSlug(bad) {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}

func TestValidateItemTruncatesLongBody(t *testing.T) {
	it := Item{Kind: KindLesson, Title: "Long", BodyMD: strings.Repeat("x", MaxBody+1)}
	if err := ValidateItem(&it); err == nil {
		t.Fatal("oversized body should be rejected")
	}
}
