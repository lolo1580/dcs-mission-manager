package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/db"
	"dcsmanager/internal/school"
)

// requireSchool hides every École endpoint outside the multi-user server. The
// tables exist in the shared schema, but the desktop must never expose them.
func (s *Server) requireSchool(w http.ResponseWriter, r *http.Request) (*db.DB, bool) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return nil, false
	}
	return s.auth, true
}

func schoolViewer(r *http.Request) (auth.User, bool) {
	return currentUser(r)
}

func (s *Server) handleSchool(w http.ResponseWriter, r *http.Request) {
	store, ok := s.requireSchool(w, r)
	if !ok {
		return
	}
	user, ok := schoolViewer(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	admin := user.Role == auth.RoleAdmin
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/school"), "/")
	parts := strings.Split(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	switch parts[0] {
	case "taxonomy":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
			return
		}
		list, err := store.SchoolTaxonomy()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"taxonomy": list})
	case "items":
		s.handleSchoolItems(w, r, store, user, admin, parts)
	case "progress":
		if len(parts) != 1 || r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET /api/school/progress"})
			return
		}
		list, err := store.SchoolProgress(user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"progress": list})
	case "quizzes":
		s.handleSchoolQuiz(w, r, store, user, admin, parts)
	case "leaderboard":
		if len(parts) != 1 || r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET /api/school/leaderboard"})
			return
		}
		list, err := store.SchoolLeaderboard(limitParam(r, 10))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"leaderboard": list})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSchoolItems(w http.ResponseWriter, r *http.Request, store *db.DB, user auth.User, admin bool, parts []string) {
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			list, err := store.SchoolItems(school.Filter{
				Kind:     r.URL.Query().Get("kind"),
				Domain:   r.URL.Query().Get("domain"),
				Era:      r.URL.Query().Get("era"),
				Country:  r.URL.Query().Get("country"),
				Status:   r.URL.Query().Get("status"),
				Query:    r.URL.Query().Get("q"),
				ViewerID: user.ID,
				Admin:    admin,
				Limit:    limitParam(r, 50),
				Offset:   offsetParam(r),
			})
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "items": list})
		case http.MethodPost:
			var item school.Item
			if err := decodeJSON(w, r, &item); err != nil {
				return
			}
			created, err := store.CreateSchoolItem(user.ID, item)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"item": publicSchoolItem(created, user.ID, admin)})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		}
		return
	}

	uid := parts[1]
	if len(parts) == 3 && parts[2] == "progress" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		if _, err := store.SchoolItem(uid, user.ID, admin); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "content not found"})
			return
		}
		var req struct {
			Status  string `json:"status"`
			Percent int    `json:"percent"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		if err := store.SetSchoolProgress(user.ID, uid, req.Status, req.Percent); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := store.SchoolItem(uid, user.ID, admin)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "content not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": publicSchoolItem(item, user.ID, admin)})
	case http.MethodPut, http.MethodPost:
		var item school.Item
		if err := decodeJSON(w, r, &item); err != nil {
			return
		}
		updated, err := store.UpdateSchoolItem(uid, user.ID, admin, item)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": publicSchoolItem(updated, user.ID, admin)})
	case http.MethodDelete:
		if err := store.ArchiveSchoolItem(uid, user.ID, admin); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET, PUT or DELETE"})
	}
}

func (s *Server) handleSchoolQuiz(w http.ResponseWriter, r *http.Request, store *db.DB, user auth.User, admin bool, parts []string) {
	if len(parts) != 3 || parts[2] != "attempts" {
		http.NotFound(w, r)
		return
	}
	uid := parts[1]
	switch r.Method {
	case http.MethodGet:
		list, err := store.SchoolAttempts(user.ID, uid, limitParam(r, 10))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attempts": list})
	case http.MethodPost:
		var req struct {
			Answers []int `json:"answers"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		attempt, err := store.SubmitSchoolAttempt(user.ID, uid, req.Answers)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attempt": attempt})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

// publicSchoolItem hides quiz answers from members who do not own the item.
func publicSchoolItem(item school.Item, viewerID int64, admin bool) school.Item {
	if item.Kind == school.KindQuiz && !admin && item.AuthorID != viewerID {
		for i := range item.Questions {
			item.Questions[i] = item.Questions[i].Public()
		}
	}
	return item
}

func (s *Server) handleAdminSchool(w http.ResponseWriter, r *http.Request) {
	store, ok := s.requireSchool(w, r)
	if !ok {
		return
	}
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/school"), "/")

	switch {
	case path == "pending":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
			return
		}
		list, err := store.SchoolItems(school.Filter{
			Admin: true, Status: school.StatusPending, Limit: 200,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "items": list})
	case strings.HasPrefix(path, "items/") && strings.HasSuffix(path, "/review"):
		uid := strings.TrimSuffix(strings.TrimPrefix(path, "items/"), "/review")
		if uid == "" || strings.Contains(uid, "/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		var req struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		approve := req.Decision == "approve"
		if !approve && req.Decision != "reject" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision must be approve or reject"})
			return
		}
		item, err := store.ReviewSchoolItem(uid, user.ID, approve, req.Note)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "content not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	case path == "taxonomy":
		switch r.Method {
		case http.MethodGet:
			list, err := store.SchoolTaxonomy()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"taxonomy": list})
		case http.MethodPost, http.MethodPut:
			var entry school.Taxonomy
			if err := decodeJSON(w, r, &entry); err != nil {
				return
			}
			saved, err := store.SaveSchoolTaxonomy(entry)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"taxonomy": saved})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		}
	case strings.HasPrefix(path, "taxonomy/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "taxonomy/"), 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE"})
			return
		}
		if err := store.DeleteSchoolTaxonomy(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.NotFound(w, r)
	}
}

func offsetParam(r *http.Request) int {
	const maxOffset = 100000
	v := r.URL.Query().Get("offset")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	if n > maxOffset {
		return maxOffset
	}
	return n
}
