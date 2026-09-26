// Package api's maintenance endpoints. They let an operator remove recorded
// sessions, which is needed because simulated data can be recorded by mistake
// (the test tools speak the same protocol as DCS), and because a beta tester
// should not have to delete the database file by hand.
package api

import (
	"net/http"
	"strconv"
	"strings"

	"dcsmm/internal/db"
)

// handlePurge removes recorded data. It is deliberately explicit about the
// scope, because the operation is irreversible.
//
//	DELETE /api/maintenance/purge?source=test   remove every simulated session
//	DELETE /api/maintenance/purge?missionId=3   remove one mission
//	DELETE /api/maintenance/purge?all=1         remove everything
//
// A request that names no scope is rejected rather than defaulting to a
// destructive action.
func (s *Server) handlePurge(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database disabled"})
		return
	}
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE or POST"})
		return
	}

	q := r.URL.Query()

	switch {
	case q.Get("all") == "1":
		res, err := s.db.PurgeAll()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"scope": "all", "result": res, "removed": res.Total()})

	case q.Get("source") != "":
		source := strings.ToLower(q.Get("source"))
		if !db.ValidSource(source) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source must be live or test"})
			return
		}
		res, err := s.db.PurgeSource(source)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"scope": source, "result": res, "removed": res.Total()})

	case q.Get("missionId") != "":
		id, err := strconv.ParseInt(q.Get("missionId"), 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missionId must be a positive integer"})
			return
		}
		res, err := s.db.PurgeMission(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"scope": "mission", "missionId": id, "result": res, "removed": res.Total()})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "nothing to do: pass one of source=test, missionId=N or all=1",
		})
	}
}

// handleMaintenance reports what is stored, so an operator can decide what to
// purge before doing it. It also exposes how many sessions of each source exist,
// which makes accidental test data immediately visible.
func (s *Server) handleMaintenance(w http.ResponseWriter, _ *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}

	live, err := s.db.CountMissions(db.SourceLive)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	test, err := s.db.CountMissions(db.SourceTest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	missions, err := s.db.MissionsWithSource("", 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"missions": map[string]int{"live": live, "test": test},
		"list":     missions,
	})
}
