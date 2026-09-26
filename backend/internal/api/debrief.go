package api

import (
	"net/http"
	"strconv"
	"strings"
)

// handleDebriefs lists stored debriefs (metadata only).
func (s *Server) handleDebriefs(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"debriefs": []any{}})
		return
	}
	list, err := s.db.Debriefs(limitParam(r, 50))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "debriefs": list})
}

// handleDebrief returns one debrief, including its raw text when ?raw=1.
func (s *Server) handleDebrief(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "persistence disabled"})
		return
	}
	idText := strings.TrimPrefix(r.URL.Path, "/api/debriefs/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	rec, err := s.db.Debrief(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "debrief not found"})
		return
	}
	if r.URL.Query().Get("raw") == "1" {
		// Raw is excluded from the normal JSON (it is large and rarely needed),
		// so it is exposed explicitly when requested.
		writeJSON(w, http.StatusOK, map[string]any{
			"id":        rec.ID,
			"missionId": rec.MissionID,
			"mission":   rec.Mission,
			"theatre":   rec.Theatre,
			"size":      rec.Size,
			"createdAt": rec.CreatedAt,
			"parsed":    rec.Parsed,
			"raw":       rec.Raw,
		})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}
