package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"dcsmanager/internal/idgen"
	"dcsmanager/internal/school"
)

// defaultSchoolTaxonomy is inserted once, when the table is empty. An
// administrator can then add, rename or remove entries without a deployment.
var defaultSchoolTaxonomy = []school.Taxonomy{
	{Kind: "domain", Slug: "avions", LabelFR: "Avions", LabelEN: "Aircraft", Position: 10},
	{Kind: "domain", Slug: "armement", LabelFR: "Armement", LabelEN: "Armament", Position: 20},
	{Kind: "domain", Slug: "radars", LabelFR: "Radars", LabelEN: "Radars", Position: 30},
	{Kind: "domain", Slug: "epoques", LabelFR: "Époques", LabelEN: "Eras", Position: 40},
	{Kind: "domain", Slug: "navires", LabelFR: "Navires", LabelEN: "Ships", Position: 50},
	{Kind: "domain", Slug: "vehicules", LabelFR: "Véhicules", LabelEN: "Vehicles", Position: 60},
	{Kind: "domain", Slug: "guerre-electronique", LabelFR: "Guerre électronique", LabelEN: "Electronic warfare", Position: 70},
	{Kind: "domain", Slug: "moteurs", LabelFR: "Moteurs", LabelEN: "Engines", Position: 80},
	{Kind: "domain", Slug: "pays-forces", LabelFR: "Pays et forces", LabelEN: "Countries and forces", Position: 90},
	{Kind: "era", Slug: "ww1", LabelFR: "1914-1918", LabelEN: "1914-1918", Position: 10},
	{Kind: "era", Slug: "interwar", LabelFR: "Entre-deux-guerres", LabelEN: "Interwar", Position: 20},
	{Kind: "era", Slug: "ww2", LabelFR: "1939-1945", LabelEN: "1939-1945", Position: 30},
	{Kind: "era", Slug: "coldwar", LabelFR: "Guerre froide", LabelEN: "Cold War", Position: 40},
	{Kind: "era", Slug: "modern", LabelFR: "Moderne", LabelEN: "Modern", Position: 50},
}

// seedSchoolTaxonomy fills the controlled vocabulary once. Existing databases
// keep whatever the administrator has configured.
func (d *DB) seedSchoolTaxonomy() error {
	var n int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM school_taxonomy`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO school_taxonomy(kind, slug, label_fr, label_en, position) VALUES(?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range defaultSchoolTaxonomy {
		if _, err := stmt.Exec(t.Kind, t.Slug, t.LabelFR, t.LabelEN, t.Position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SchoolTaxonomy lists the configured domains and eras.
func (d *DB) SchoolTaxonomy() ([]school.Taxonomy, error) {
	rows, err := d.sql.Query(
		`SELECT id, kind, slug, label_fr, label_en, position
		 FROM school_taxonomy ORDER BY kind, position, label_fr`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []school.Taxonomy
	for rows.Next() {
		var t school.Taxonomy
		if err := rows.Scan(&t.ID, &t.Kind, &t.Slug, &t.LabelFR, &t.LabelEN, &t.Position); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveSchoolTaxonomy creates or updates one domain or era.
func (d *DB) SaveSchoolTaxonomy(t school.Taxonomy) (school.Taxonomy, error) {
	t.Slug = strings.TrimSpace(t.Slug)
	t.LabelFR = strings.TrimSpace(t.LabelFR)
	t.LabelEN = strings.TrimSpace(t.LabelEN)
	if t.Kind != "domain" && t.Kind != "era" {
		return t, errors.New("taxonomy kind must be domain or era")
	}
	if !school.ValidSlug(t.Slug) || t.LabelFR == "" || t.LabelEN == "" {
		return t, errors.New("taxonomy needs a slug and both labels")
	}
	if t.ID > 0 {
		if _, err := d.sql.Exec(
			`UPDATE school_taxonomy SET kind = ?, slug = ?, label_fr = ?, label_en = ?, position = ? WHERE id = ?`,
			t.Kind, t.Slug, t.LabelFR, t.LabelEN, t.Position, t.ID,
		); err != nil {
			return t, err
		}
		return t, nil
	}
	res, err := d.sql.Exec(
		`INSERT INTO school_taxonomy(kind, slug, label_fr, label_en, position) VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(kind, slug) DO UPDATE SET label_fr = excluded.label_fr,
		   label_en = excluded.label_en, position = excluded.position`,
		t.Kind, t.Slug, t.LabelFR, t.LabelEN, t.Position,
	)
	if err != nil {
		return t, err
	}
	if t.ID, err = res.LastInsertId(); err != nil {
		return t, err
	}
	return t, nil
}

// DeleteSchoolTaxonomy removes an entry. Existing content keeps its slug.
func (d *DB) DeleteSchoolTaxonomy(id int64) error {
	_, err := d.sql.Exec(`DELETE FROM school_taxonomy WHERE id = ?`, id)
	return err
}

const schoolItemSelect = `SELECT i.id, i.uid, i.kind, i.title, i.designation, i.summary, i.body_md,
	i.domain, i.era, i.country, i.manufacturer, i.service_start, i.service_end, i.specs_json,
	i.sources_json, i.image_url, i.tags, i.author_id, COALESCE(u.username,''), i.status,
	i.review_note, i.reviewed_by, i.reviewed_at, i.created_at, i.updated_at, i.published_at,
	EXISTS(SELECT 1 FROM school_revisions r WHERE r.item_uid = i.uid AND r.status = 'pending')
	FROM school_items i LEFT JOIN users u ON u.id = i.author_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSchoolItem(row rowScanner) (school.Item, error) {
	var (
		it                                       school.Item
		serviceStart, serviceEnd                 sql.NullInt64
		reviewedBy, reviewedAt, publishedAt      sql.NullInt64
		specsJSON, sourcesJSON, tagsJSON, author string
	)
	if err := row.Scan(
		&it.ID, &it.UID, &it.Kind, &it.Title, &it.Designation, &it.Summary, &it.BodyMD,
		&it.Domain, &it.Era, &it.Country, &it.Manufacturer, &serviceStart, &serviceEnd,
		&specsJSON, &sourcesJSON, &it.ImageURL, &tagsJSON, &it.AuthorID, &author,
		&it.Status, &it.ReviewNote, &reviewedBy, &reviewedAt, &it.CreatedAt, &it.UpdatedAt,
		&publishedAt, &it.RevisionPending,
	); err != nil {
		return it, err
	}
	it.Author = author
	if serviceStart.Valid {
		it.ServiceStart = int(serviceStart.Int64)
	}
	if serviceEnd.Valid {
		it.ServiceEnd = int(serviceEnd.Int64)
	}
	if reviewedBy.Valid {
		it.ReviewedBy = reviewedBy.Int64
	}
	if reviewedAt.Valid {
		it.ReviewedAt = reviewedAt.Int64
	}
	if publishedAt.Valid {
		it.PublishedAt = publishedAt.Int64
	}
	_ = json.Unmarshal([]byte(specsJSON), &it.Specs)
	_ = json.Unmarshal([]byte(sourcesJSON), &it.Sources)
	it.Tags = splitTags(tagsJSON)
	return it, nil
}

func splitTags(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SchoolItems returns the visible page for one viewer. Members see published
// content plus their own drafts; administrators see everything.
func (d *DB) SchoolItems(f school.Filter) ([]school.Item, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	where := []string{"1=1"}
	var args []any
	if f.Kind != "" {
		where = append(where, "i.kind = ?")
		args = append(args, f.Kind)
	}
	if f.Domain != "" {
		where = append(where, "i.domain = ?")
		args = append(args, f.Domain)
	}
	if f.Era != "" {
		where = append(where, "i.era = ?")
		args = append(args, f.Era)
	}
	if f.Country != "" {
		where = append(where, "i.country = ?")
		args = append(args, f.Country)
	}
	if f.Admin && f.Status == school.StatusPending {
		// The queue contains new submissions and published items with an edit
		// waiting for approval.
		where = append(where, `(i.status = ? OR EXISTS (
			SELECT 1 FROM school_revisions r WHERE r.item_uid = i.uid AND r.status = ?))`)
		args = append(args, school.StatusPending, school.StatusPending)
	} else if f.Admin && f.Status != "" {
		where = append(where, "i.status = ?")
		args = append(args, f.Status)
	} else {
		// Archived content is hidden unless it is explicitly requested.
		where = append(where, "i.status <> ?")
		args = append(args, school.StatusArchived)
	}
	if !f.Admin {
		where = append(where, "(i.status = ? OR i.author_id = ?)")
		args = append(args, school.StatusPublished, f.ViewerID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(i.title LIKE ? OR i.designation LIKE ? OR i.summary LIKE ? OR i.body_md LIKE ?)")
		args = append(args, like, like, like, like)
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := d.sql.Query(
		schoolItemSelect+` WHERE `+strings.Join(where, " AND ")+
			` ORDER BY COALESCE(i.published_at, i.updated_at) DESC, i.id DESC LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []school.Item
	for rows.Next() {
		it, err := scanSchoolItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// SchoolItem loads one item and its questions, links and backlinks. It returns
// sql.ErrNoRows when the viewer may not see it.
func (d *DB) SchoolItem(uid string, viewerID int64, admin bool) (school.Item, error) {
	it, err := scanSchoolItem(d.sql.QueryRow(schoolItemSelect+` WHERE i.uid = ?`, uid))
	if err != nil {
		return it, err
	}
	if !admin && it.Status != school.StatusPublished && it.AuthorID != viewerID {
		return school.Item{}, sql.ErrNoRows
	}
	if it.Kind == school.KindQuiz {
		questions, err := d.schoolQuestions(uid)
		if err != nil {
			return it, err
		}
		it.Questions = questions
	}
	if it.Links, err = d.schoolLinkUIDs("to_uid", "from_uid", uid); err != nil {
		return it, err
	}
	if it.Backlinks, err = d.schoolLinkUIDs("from_uid", "to_uid", uid); err != nil {
		return it, err
	}
	return it, nil
}

func (d *DB) schoolLinkUIDs(selectCol, whereCol, uid string) ([]string, error) {
	rows, err := d.sql.Query(
		`SELECT `+selectCol+` FROM school_links WHERE `+whereCol+` = ? AND relation = ? ORDER BY `+selectCol,
		uid, school.RelationSeeAlso,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) schoolQuestions(quizUID string) ([]school.Question, error) {
	rows, err := d.sql.Query(
		`SELECT id, prompt, choices_json, correct_index, explanation, position
		 FROM school_questions WHERE quiz_uid = ? ORDER BY position, id`, quizUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []school.Question
	for rows.Next() {
		var q school.Question
		var choices string
		var correct int
		if err := rows.Scan(&q.ID, &q.Prompt, &choices, &correct, &q.Explanation, &q.Position); err != nil {
			return nil, err
		}
		q.CorrectIndex = school.Index(correct)
		_ = json.Unmarshal([]byte(choices), &q.Choices)
		out = append(out, q)
	}
	return out, rows.Err()
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func tagsCSV(tags []string) string {
	return strings.Join(tags, ",")
}

func (d *DB) replaceSchoolQuestions(tx *sql.Tx, quizUID string, questions []school.Question) error {
	if _, err := tx.Exec(`DELETE FROM school_questions WHERE quiz_uid = ?`, quizUID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO school_questions(quiz_uid, prompt, choices_json, correct_index, explanation, position)
		 VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, q := range questions {
		correct := 0
		if q.CorrectIndex != nil {
			correct = *q.CorrectIndex
		}
		if _, err := stmt.Exec(quizUID, q.Prompt, jsonString(q.Choices), correct, q.Explanation, i); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) replaceSchoolLinks(tx *sql.Tx, uid string, links []string) error {
	if _, err := tx.Exec(
		`DELETE FROM school_links WHERE from_uid = ? AND relation = ?`, uid, school.RelationSeeAlso,
	); err != nil {
		return err
	}
	if len(links) > school.MaxLinks {
		return fmt.Errorf("at most %d links are allowed", school.MaxLinks)
	}
	seen := make(map[string]bool, len(links))
	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO school_links(from_uid, to_uid, relation) VALUES(?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, target := range links {
		target = strings.TrimSpace(target)
		if target == "" || target == uid || seen[target] {
			continue
		}
		var exists int
		if err := tx.QueryRow(`SELECT 1 FROM school_items WHERE uid = ?`, target).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		seen[target] = true
		if _, err := stmt.Exec(uid, target, school.RelationSeeAlso); err != nil {
			return err
		}
	}
	return nil
}

// CreateSchoolItem stores a member submission in the pending queue.
func (d *DB) CreateSchoolItem(authorID int64, it school.Item) (school.Item, error) {
	if err := school.ValidateItem(&it); err != nil {
		return it, err
	}
	it.UID = idgen.New("sc_")
	it.AuthorID = authorID
	it.Status = school.StatusPending
	now := time.Now().UnixMilli()
	it.CreatedAt, it.UpdatedAt = now, now
	it.PublishedAt = 0
	it.ReviewedAt = 0
	it.ReviewedBy = 0

	tx, err := d.sql.Begin()
	if err != nil {
		return it, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO school_items(uid, kind, title, designation, summary, body_md, domain, era,
		   country, manufacturer, service_start, service_end, specs_json, sources_json, image_url,
		   tags, author_id, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.UID, it.Kind, it.Title, it.Designation, it.Summary, it.BodyMD, it.Domain, it.Era,
		it.Country, it.Manufacturer, nullInt(int64(it.ServiceStart)), nullInt(int64(it.ServiceEnd)),
		jsonString(it.Specs), jsonString(it.Sources), it.ImageURL, tagsCSV(it.Tags),
		authorID, it.Status, now, now,
	); err != nil {
		return it, err
	}
	if err := d.replaceSchoolQuestions(tx, it.UID, it.Questions); err != nil {
		return it, err
	}
	if err := d.replaceSchoolLinks(tx, it.UID, it.Links); err != nil {
		return it, err
	}
	if err := tx.Commit(); err != nil {
		return it, err
	}
	return d.SchoolItem(it.UID, authorID, true)
}

// UpdateSchoolItem edits an existing item. A member editing a published item
// creates a pending revision; the published version stays live until approval.
func (d *DB) UpdateSchoolItem(uid string, authorID int64, admin bool, it school.Item) (school.Item, error) {
	existing, err := d.SchoolItem(uid, authorID, admin)
	if err != nil {
		return it, err
	}
	if !admin && existing.AuthorID != authorID {
		return it, errors.New("only the author or an administrator can edit this content")
	}
	it.UID = uid
	it.AuthorID = existing.AuthorID
	if err := school.ValidateItem(&it); err != nil {
		return it, err
	}
	now := time.Now().UnixMilli()

	if existing.Status == school.StatusPublished && !admin {
		if err := d.saveSchoolRevision(existing, authorID, it, now); err != nil {
			return it, err
		}
		existing.RevisionPending = true
		return existing, nil
	}

	status := existing.Status
	if status == school.StatusRejected {
		status = school.StatusPending
	}
	if admin && school.ValidStatus(it.Status) {
		status = it.Status
	}
	publishedAt := existing.PublishedAt
	if status == school.StatusPublished && publishedAt == 0 {
		publishedAt = now
	}

	tx, err := d.sql.Begin()
	if err != nil {
		return it, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`UPDATE school_items SET kind = ?, title = ?, designation = ?, summary = ?, body_md = ?,
		   domain = ?, era = ?, country = ?, manufacturer = ?, service_start = ?, service_end = ?,
		   specs_json = ?, sources_json = ?, image_url = ?, tags = ?, status = ?, published_at = ?,
		   updated_at = ? WHERE uid = ?`,
		it.Kind, it.Title, it.Designation, it.Summary, it.BodyMD, it.Domain, it.Era,
		it.Country, it.Manufacturer, nullInt(int64(it.ServiceStart)), nullInt(int64(it.ServiceEnd)),
		jsonString(it.Specs), jsonString(it.Sources), it.ImageURL, tagsCSV(it.Tags),
		status, nullInt(publishedAt), now, uid,
	); err != nil {
		return it, err
	}
	if err := d.replaceSchoolQuestions(tx, uid, it.Questions); err != nil {
		return it, err
	}
	if err := d.replaceSchoolLinks(tx, uid, it.Links); err != nil {
		return it, err
	}
	if err := tx.Commit(); err != nil {
		return it, err
	}
	return d.SchoolItem(uid, authorID, true)
}

func (d *DB) saveSchoolRevision(existing school.Item, authorID int64, it school.Item, now int64) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO school_revisions(item_uid, title, designation, summary, body_md, domain, era,
		   country, manufacturer, service_start, service_end, specs_json, sources_json, image_url,
		   tags, links_json, questions_json, author_id, status, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		existing.UID, it.Title, it.Designation, it.Summary, it.BodyMD, it.Domain, it.Era,
		it.Country, it.Manufacturer, nullInt(int64(it.ServiceStart)), nullInt(int64(it.ServiceEnd)),
		jsonString(it.Specs), jsonString(it.Sources), it.ImageURL, tagsCSV(it.Tags),
		jsonString(it.Links), jsonString(it.Questions), authorID, school.StatusPending, now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ReviewSchoolItem approves or rejects a pending submission or revision.
func (d *DB) ReviewSchoolItem(uid string, adminID int64, approve bool, note string) (school.Item, error) {
	note = strings.TrimSpace(note)
	now := time.Now().UnixMilli()

	var revID int64
	var rev school.Item
	var specsJSON, sourcesJSON, tagsCSV, linksJSON, questionsJSON string
	var serviceStart, serviceEnd sql.NullInt64
	err := d.sql.QueryRow(
		`SELECT id, title, designation, summary, body_md, domain, era, country, manufacturer,
		        service_start, service_end, specs_json, sources_json, image_url, tags,
		        links_json, questions_json
		 FROM school_revisions WHERE item_uid = ? AND status = ? ORDER BY id DESC LIMIT 1`,
		uid, school.StatusPending,
	).Scan(&revID, &rev.Title, &rev.Designation, &rev.Summary, &rev.BodyMD, &rev.Domain, &rev.Era,
		&rev.Country, &rev.Manufacturer, &serviceStart, &serviceEnd, &specsJSON, &sourcesJSON,
		&rev.ImageURL, &tagsCSV, &linksJSON, &questionsJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return school.Item{}, err
	}
	if err == nil {
		if !approve {
			_, err := d.sql.Exec(
				`UPDATE school_revisions SET status = ?, review_note = ?, reviewed_at = ? WHERE id = ?`,
				school.StatusRejected, note, now, revID)
			if err != nil {
				return school.Item{}, err
			}
			return d.SchoolItem(uid, adminID, true)
		}
		if serviceStart.Valid {
			rev.ServiceStart = int(serviceStart.Int64)
		}
		if serviceEnd.Valid {
			rev.ServiceEnd = int(serviceEnd.Int64)
		}
		_ = json.Unmarshal([]byte(specsJSON), &rev.Specs)
		_ = json.Unmarshal([]byte(sourcesJSON), &rev.Sources)
		rev.Tags = splitTags(tagsCSV)
		var links []string
		var questions []school.Question
		_ = json.Unmarshal([]byte(linksJSON), &links)
		_ = json.Unmarshal([]byte(questionsJSON), &questions)

		tx, err := d.sql.Begin()
		if err != nil {
			return school.Item{}, err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(
			`UPDATE school_items SET title = ?, designation = ?, summary = ?, body_md = ?,
			   domain = ?, era = ?, country = ?, manufacturer = ?, service_start = ?, service_end = ?,
			   specs_json = ?, sources_json = ?, image_url = ?, tags = ?, status = ?,
			   review_note = ?, reviewed_by = ?, reviewed_at = ?,
			   published_at = CASE WHEN published_at IS NULL OR published_at = 0 THEN ? ELSE published_at END,
			   updated_at = ? WHERE uid = ?`,
			rev.Title, rev.Designation, rev.Summary, rev.BodyMD, rev.Domain, rev.Era, rev.Country,
			rev.Manufacturer, nullInt(int64(rev.ServiceStart)), nullInt(int64(rev.ServiceEnd)),
			specsJSON, sourcesJSON, rev.ImageURL, tagsCSV, school.StatusPublished, note,
			adminID, now, now, now, uid,
		); err != nil {
			return school.Item{}, err
		}
		if err := d.replaceSchoolQuestions(tx, uid, questions); err != nil {
			return school.Item{}, err
		}
		if err := d.replaceSchoolLinks(tx, uid, links); err != nil {
			return school.Item{}, err
		}
		if _, err := tx.Exec(
			`UPDATE school_revisions SET status = ?, review_note = ?, reviewed_at = ? WHERE id = ?`,
			school.StatusPublished, note, now, revID); err != nil {
			return school.Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return school.Item{}, err
		}
		return d.SchoolItem(uid, adminID, true)
	}

	// No revision: this is a first submission.
	status := school.StatusRejected
	if approve {
		status = school.StatusPublished
	}
	_, err = d.sql.Exec(
		`UPDATE school_items SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?,
		   published_at = CASE WHEN ? = ? AND (published_at IS NULL OR published_at = 0) THEN ? ELSE published_at END,
		   updated_at = ? WHERE uid = ?`,
		status, note, adminID, now, status, school.StatusPublished, now, now, uid,
	)
	if err != nil {
		return school.Item{}, err
	}
	return d.SchoolItem(uid, adminID, true)
}

// ArchiveSchoolItem soft-deletes an item.
func (d *DB) ArchiveSchoolItem(uid string, authorID int64, admin bool) error {
	existing, err := d.SchoolItem(uid, authorID, admin)
	if err != nil {
		return err
	}
	if !admin && existing.AuthorID != authorID {
		return errors.New("only the author or an administrator can archive this content")
	}
	_, err = d.sql.Exec(
		`UPDATE school_items SET status = ?, updated_at = ? WHERE uid = ?`,
		school.StatusArchived, time.Now().UnixMilli(), uid,
	)
	return err
}

// SetSchoolProgress records a member's progress for a lesson or a card.
func (d *DB) SetSchoolProgress(userID int64, uid, status string, percent int) error {
	if !school.ValidProgress(status) {
		return errors.New("unknown progress status")
	}
	if status == school.ProgressCompleted {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	var exists int
	if err := d.sql.QueryRow(`SELECT 1 FROM school_items WHERE uid = ?`, uid).Scan(&exists); err != nil {
		return err
	}
	_, err := d.sql.Exec(
		`INSERT INTO school_progress(user_id, item_uid, status, percent, updated_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, item_uid) DO UPDATE SET
		   status = excluded.status, percent = excluded.percent, updated_at = excluded.updated_at`,
		userID, uid, status, percent, time.Now().UnixMilli(),
	)
	return err
}

// SchoolProgress lists one member's progress.
func (d *DB) SchoolProgress(userID int64) ([]school.Progress, error) {
	rows, err := d.sql.Query(
		`SELECT user_id, item_uid, status, percent, updated_at
		 FROM school_progress WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []school.Progress
	for rows.Next() {
		var p school.Progress
		if err := rows.Scan(&p.UserID, &p.ItemUID, &p.Status, &p.Percent, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SubmitSchoolAttempt scores a quiz on the server and stores the attempt.
func (d *DB) SubmitSchoolAttempt(userID int64, quizUID string, answers []int) (school.Attempt, error) {
	quiz, err := d.SchoolItem(quizUID, userID, false)
	if err != nil {
		return school.Attempt{}, err
	}
	if quiz.Kind != school.KindQuiz {
		return school.Attempt{}, errors.New("this content is not a quiz")
	}
	if len(quiz.Questions) == 0 {
		return school.Attempt{}, errors.New("this quiz has no questions")
	}
	score, max := school.Score(quiz.Questions, answers)
	now := time.Now().UnixMilli()
	res, err := d.sql.Exec(
		`INSERT INTO school_attempts(quiz_uid, user_id, score, max_score, answers, created_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		quizUID, userID, score, max, jsonString(answers), now,
	)
	if err != nil {
		return school.Attempt{}, err
	}
	id, _ := res.LastInsertId()
	return school.Attempt{
		ID: id, QuizUID: quizUID, UserID: userID, Score: score, MaxScore: max,
		CreatedAt: now, Answers: answers, Correction: quiz.Questions,
	}, nil
}

// SchoolAttempts lists a member's attempts for one quiz, newest first.
func (d *DB) SchoolAttempts(userID int64, quizUID string, limit int) ([]school.Attempt, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := d.sql.Query(
		`SELECT id, quiz_uid, user_id, score, max_score, answers, created_at
		 FROM school_attempts WHERE user_id = ? AND quiz_uid = ? ORDER BY id DESC LIMIT ?`,
		userID, quizUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []school.Attempt
	for rows.Next() {
		var a school.Attempt
		var answers string
		if err := rows.Scan(&a.ID, &a.QuizUID, &a.UserID, &a.Score, &a.MaxScore, &answers, &a.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(answers), &a.Answers)
		out = append(out, a)
	}
	return out, rows.Err()
}

// SchoolLeaderboard aggregates completed lessons, read cards and the best quiz
// score for every active account.
func (d *DB) SchoolLeaderboard(limit int) ([]school.LeaderboardEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	lessons := map[int64]int{}
	cards := map[int64]int{}
	rows, err := d.sql.Query(
		`SELECT p.user_id,
		        SUM(CASE WHEN i.kind = ? THEN 1 ELSE 0 END),
		        SUM(CASE WHEN i.kind = ? THEN 1 ELSE 0 END)
		 FROM school_progress p JOIN school_items i ON i.uid = p.item_uid
		 WHERE p.status = ? AND i.status = ?
		 GROUP BY p.user_id`,
		school.KindLesson, school.KindCard, school.ProgressCompleted, school.StatusPublished)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userID int64
		var l, c int
		if err := rows.Scan(&userID, &l, &c); err != nil {
			rows.Close()
			return nil, err
		}
		lessons[userID] = l
		cards[userID] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	quizzes := map[int64]int{}
	rows, err = d.sql.Query(
		`SELECT a.user_id, MAX(CASE WHEN a.max_score > 0 THEN a.score * 100 / a.max_score ELSE 0 END)
		 FROM school_attempts a JOIN school_items i ON i.uid = a.quiz_uid
		 WHERE i.kind = ? AND i.status = ?
		 GROUP BY a.user_id`, school.KindQuiz, school.StatusPublished)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userID int64
		var pct int
		if err := rows.Scan(&userID, &pct); err != nil {
			rows.Close()
			return nil, err
		}
		quizzes[userID] = pct
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = d.sql.Query(
		`SELECT id, username, COALESCE(display_name,'') FROM users WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	var out []school.LeaderboardEntry
	for rows.Next() {
		var e school.LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.DisplayName); err != nil {
			rows.Close()
			return nil, err
		}
		e.Lessons = lessons[e.UserID]
		e.Cards = cards[e.UserID]
		e.QuizPoints = quizzes[e.UserID]
		e.Points = school.Points(e.Lessons, e.Cards, e.QuizPoints)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		if out[i].QuizPoints != out[j].QuizPoints {
			return out[i].QuizPoints > out[j].QuizPoints
		}
		return out[i].Username < out[j].Username
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
