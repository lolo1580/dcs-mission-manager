// Package school contains the École & Culture G content model: lessons,
// encyclopedic cards, quizzes, progress and the moderation rules shared by the
// database and the HTTP layer.
//
// The feature is server-only: the desktop profile never exposes it. Keeping the
// pure rules here makes them testable without a database or an HTTP request.
package school

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Kinds of content.
const (
	KindLesson = "lesson"
	KindCard   = "card"
	KindQuiz   = "quiz"
)

// Moderation states. Members create pending content; an administrator approves
// or rejects it.
const (
	StatusDraft     = "draft"
	StatusPending   = "pending"
	StatusPublished = "published"
	StatusRejected  = "rejected"
	StatusArchived  = "archived"
)

// Progress states.
const (
	ProgressInProgress = "in_progress"
	ProgressCompleted  = "completed"
)

// Link relations.
const (
	RelationSeeAlso = "see_also"
	RelationQuiz    = "quiz"
	RelationEra     = "era"
)

// Limits keep a single document (and the JSON returned to the UI) bounded.
const (
	MaxTitle       = 200
	MaxDesignation = 120
	MaxSummary     = 600
	MaxBody        = 64 * 1024
	MaxSpecs       = 50
	MaxSources     = 30
	MaxTags        = 30
	MaxTag         = 40
	MaxLinks       = 50
	MaxQuestions   = 100
	MaxChoices     = 8
	MaxExplanation = 4000
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Spec is one row of a fact sheet's specification table.
type Spec struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

// Source is a reference attached to a card or lesson.
type Source struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Question is a single-choice quiz question. CorrectIndex and Explanation must
// never be sent to a member before the attempt is submitted.
type Question struct {
	ID           int64    `json:"id,omitempty"`
	Prompt       string   `json:"prompt"`
	Choices      []string `json:"choices"`
	CorrectIndex *int     `json:"correctIndex,omitempty"`
	Explanation  string   `json:"explanation,omitempty"`
	Position     int      `json:"position"`
}

// Index returns a pointer to an answer index, for building a Question literal.
func Index(v int) *int { return &v }

// Public returns the question with the answer and explanation removed.
func (q Question) Public() Question {
	q.CorrectIndex = nil
	q.Explanation = ""
	return q
}

// Item is a lesson, an encyclopedic card or a quiz.
type Item struct {
	ID           int64      `json:"id"`
	UID          string     `json:"uid"`
	Kind         string     `json:"kind"`
	Title        string     `json:"title"`
	Designation  string     `json:"designation,omitempty"`
	Summary      string     `json:"summary,omitempty"`
	BodyMD       string     `json:"bodyMd,omitempty"`
	Domain       string     `json:"domain,omitempty"`
	Era          string     `json:"era,omitempty"`
	Country      string     `json:"country,omitempty"`
	Manufacturer string     `json:"manufacturer,omitempty"`
	ServiceStart int        `json:"serviceStart,omitempty"`
	ServiceEnd   int        `json:"serviceEnd,omitempty"`
	Specs        []Spec     `json:"specs,omitempty"`
	Sources      []Source   `json:"sources,omitempty"`
	ImageURL     string     `json:"imageUrl,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	AuthorID     int64      `json:"authorId,omitempty"`
	Author       string     `json:"author,omitempty"`
	Status       string     `json:"status"`
	ReviewNote   string     `json:"reviewNote,omitempty"`
	ReviewedBy   int64      `json:"reviewedBy,omitempty"`
	ReviewedAt   int64      `json:"reviewedAt,omitempty"`
	CreatedAt    int64      `json:"createdAt"`
	UpdatedAt    int64      `json:"updatedAt"`
	PublishedAt  int64      `json:"publishedAt,omitempty"`
	Questions    []Question `json:"questions,omitempty"`
	Links        []string   `json:"links,omitempty"`
	Backlinks    []string   `json:"backlinks,omitempty"`
	// RevisionPending is set when a published item has an edit waiting for an
	// administrator. The published version stays live in the meantime.
	RevisionPending bool `json:"revisionPending,omitempty"`
}

// Taxonomy is an administrator-managed domain or era.
type Taxonomy struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"` // domain | era
	Slug     string `json:"slug"`
	LabelFR  string `json:"labelFr"`
	LabelEN  string `json:"labelEn"`
	Position int    `json:"position"`
}

// Link is a directed cross-reference between two items.
type Link struct {
	FromUID  string `json:"fromUid"`
	ToUID    string `json:"toUid"`
	Relation string `json:"relation"`
}

// Progress is one member's state for one lesson or card.
type Progress struct {
	UserID    int64  `json:"userId"`
	ItemUID   string `json:"itemUid"`
	Status    string `json:"status"`
	Percent   int    `json:"percent"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Attempt is one submitted quiz.
type Attempt struct {
	ID        int64  `json:"id"`
	QuizUID   string `json:"quizUid"`
	UserID    int64  `json:"userId"`
	Score     int    `json:"score"`
	MaxScore  int    `json:"maxScore"`
	CreatedAt int64  `json:"createdAt"`
	Answers   []int  `json:"answers,omitempty"`
	// Correction is only filled by the submission response, after scoring.
	Correction []Question `json:"correction,omitempty"`
}

// LeaderboardEntry aggregates a member's École activity.
type LeaderboardEntry struct {
	UserID      int64  `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Points      int    `json:"points"`
	Lessons     int    `json:"lessons"`
	Cards       int    `json:"cards"`
	QuizPoints  int    `json:"quizPoints"`
}

// Filter selects a page of content.
type Filter struct {
	Kind     string
	Domain   string
	Era      string
	Country  string
	Status   string
	Query    string
	ViewerID int64
	Admin    bool
	Limit    int
	Offset   int
}

// ValidKind reports whether kind is one of the three content types.
func ValidKind(kind string) bool {
	return kind == KindLesson || kind == KindCard || kind == KindQuiz
}

// ValidStatus reports whether status is a known moderation state.
func ValidStatus(status string) bool {
	switch status {
	case StatusDraft, StatusPending, StatusPublished, StatusRejected, StatusArchived:
		return true
	}
	return false
}

// ValidProgress reports whether status is a known progress state.
func ValidProgress(status string) bool {
	return status == ProgressInProgress || status == ProgressCompleted
}

// ValidSlug reports whether a taxonomy slug is safe to use in a query string.
func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

// ValidateItem normalises and validates a submission. It trims text, drops
// empty tags and enforces the limits documented in the plan.
func ValidateItem(it *Item) error {
	if it == nil {
		return errors.New("item is required")
	}
	it.Title = strings.TrimSpace(it.Title)
	it.Designation = strings.TrimSpace(it.Designation)
	it.Summary = strings.TrimSpace(it.Summary)
	it.Domain = strings.TrimSpace(it.Domain)
	it.Era = strings.TrimSpace(it.Era)
	it.Country = strings.TrimSpace(it.Country)
	it.Manufacturer = strings.TrimSpace(it.Manufacturer)
	it.ImageURL = strings.TrimSpace(it.ImageURL)

	if !ValidKind(it.Kind) {
		return fmt.Errorf("unknown content kind %q", it.Kind)
	}
	if it.Title == "" || len(it.Title) > MaxTitle {
		return fmt.Errorf("title is required and must be at most %d characters", MaxTitle)
	}
	if len(it.Designation) > MaxDesignation || len(it.Summary) > MaxSummary || len(it.BodyMD) > MaxBody {
		return errors.New("content exceeds the size limits")
	}
	for name, slug := range map[string]string{"domain": it.Domain, "era": it.Era} {
		if slug != "" && !ValidSlug(slug) {
			return fmt.Errorf("%s must be a lowercase slug", name)
		}
	}
	if it.ServiceStart != 0 && (it.ServiceStart < 1000 || it.ServiceStart > 2200) {
		return errors.New("service start year is out of range")
	}
	if it.ServiceEnd != 0 && (it.ServiceEnd < 1000 || it.ServiceEnd > 2200) {
		return errors.New("service end year is out of range")
	}
	if it.ServiceStart != 0 && it.ServiceEnd != 0 && it.ServiceEnd < it.ServiceStart {
		return errors.New("service end must be after service start")
	}
	if len(it.Specs) > MaxSpecs {
		return fmt.Errorf("at most %d specifications are allowed", MaxSpecs)
	}
	for i := range it.Specs {
		it.Specs[i].Label = strings.TrimSpace(it.Specs[i].Label)
		it.Specs[i].Value = strings.TrimSpace(it.Specs[i].Value)
		it.Specs[i].Unit = strings.TrimSpace(it.Specs[i].Unit)
		if it.Specs[i].Label == "" || it.Specs[i].Value == "" {
			return errors.New("a specification needs a label and a value")
		}
	}
	if len(it.Sources) > MaxSources {
		return fmt.Errorf("at most %d sources are allowed", MaxSources)
	}
	for i := range it.Sources {
		it.Sources[i].Label = strings.TrimSpace(it.Sources[i].Label)
		it.Sources[i].URL = strings.TrimSpace(it.Sources[i].URL)
		if it.Sources[i].Label == "" || !validHTTPURL(it.Sources[i].URL) {
			return errors.New("a source needs a label and an http(s) URL")
		}
	}
	if it.ImageURL != "" && !validHTTPURL(it.ImageURL) {
		return errors.New("image URL must use http or https")
	}

	seen := make(map[string]bool, len(it.Tags))
	out := it.Tags[:0]
	for _, tag := range it.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if len(tag) > MaxTag {
			return fmt.Errorf("a tag must be at most %d characters", MaxTag)
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	it.Tags = out
	if len(it.Tags) > MaxTags {
		return fmt.Errorf("at most %d tags are allowed", MaxTags)
	}

	if it.Kind == KindQuiz {
		if err := ValidateQuestions(it.Questions); err != nil {
			return err
		}
	} else {
		it.Questions = nil
	}
	return nil
}

// ValidateQuestions checks a quiz definition and assigns positions.
func ValidateQuestions(questions []Question) error {
	if len(questions) == 0 {
		return errors.New("a quiz needs at least one question")
	}
	if len(questions) > MaxQuestions {
		return fmt.Errorf("a quiz accepts at most %d questions", MaxQuestions)
	}
	for i := range questions {
		q := &questions[i]
		q.Prompt = strings.TrimSpace(q.Prompt)
		q.Explanation = strings.TrimSpace(q.Explanation)
		q.Position = i
		if q.Prompt == "" {
			return errors.New("a question needs a prompt")
		}
		if len(q.Choices) < 2 || len(q.Choices) > MaxChoices {
			return fmt.Errorf("a question needs between 2 and %d choices", MaxChoices)
		}
		for j := range q.Choices {
			q.Choices[j] = strings.TrimSpace(q.Choices[j])
			if q.Choices[j] == "" {
				return errors.New("a choice cannot be empty")
			}
		}
		if q.CorrectIndex == nil || *q.CorrectIndex < 0 || *q.CorrectIndex >= len(q.Choices) {
			return errors.New("the correct choice is out of range")
		}
		if len(q.Explanation) > MaxExplanation {
			return errors.New("a question explanation is too long")
		}
	}
	return nil
}

// Score returns the number of correct answers and the maximum score. Missing or
// out-of-range answers count as wrong; the client can never provide the score.
func Score(questions []Question, answers []int) (score, max int) {
	max = len(questions)
	for i, q := range questions {
		if q.CorrectIndex != nil && i < len(answers) && answers[i] == *q.CorrectIndex {
			score++
		}
	}
	return score, max
}

// Points is the leaderboard formula: 10 per completed lesson, 2 per read card
// and the best quiz percentage.
func Points(lessons, cards, quizPoints int) int {
	return 10*lessons + 2*cards + quizPoints
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
