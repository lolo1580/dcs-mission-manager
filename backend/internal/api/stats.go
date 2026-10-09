package api

import (
	"net/http"
	"strconv"

	"dcsmanager/internal/db"
	"dcsmanager/internal/stats"
)

// scopeFromRequest builds a stats scope from query parameters.
//
// ?scope=career (default) aggregates everything.
// ?scope=mission&missionId=N restricts to one mission; when missionId is absent
// the most recent mission is used.
func (s *Server) scopeFromRequest(r *http.Request) stats.Scope {
	q := r.URL.Query()
	sc := stats.Scope{Mode: q.Get("scope")}
	if sc.Mode == "" {
		sc.Mode = "career"
	}
	// Test sessions are excluded unless explicitly requested, so the dashboard
	// never presents simulated data as a real career.
	sc.IncludeTest = q.Get("includeTest") == "1"
	if from, err := strconv.ParseInt(q.Get("from"), 10, 64); err == nil && from > 0 {
		sc.FromMs = from
	}
	if before, err := strconv.ParseInt(q.Get("before"), 10, 64); err == nil && before > 0 {
		sc.BeforeMs = before
	}
	if sc.Mode == "mission" {
		if id, err := strconv.ParseInt(q.Get("missionId"), 10, 64); err == nil && id > 0 {
			sc.MissionID = id
		} else if s.db != nil {
			// Only live missions appear by default: the newest test run must not
			// make the Mission view look empty while career stats exclude tests.
			if list, err := s.db.MissionsWithSource(db.SourceLive, 1); err == nil && len(list) > 0 {
				sc.MissionID = list[0].ID
			}
		}
	}
	return sc
}

func (s *Server) handleStatsMissions(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	list, err := s.db.MissionsWithSource(db.SourceLive, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"missions": list})
}

func (s *Server) handleStatsTrend(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	points, err := s.stats.TrendWithScope(r.URL.Query().Get("ucid"), 20, s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"missions": points})
}

// statsReady reports whether statistics can be served, and answers the request
// itself when they cannot.
//
// The database can legitimately be absent: DCSMANAGER_DB_ENABLED=false keeps
// everything in memory. stats.New still returns a service in that case, so the
// guard must check the database too — otherwise the handlers dereference a nil
// *db.DB and panic instead of degrading.
func (s *Server) statsReady(w http.ResponseWriter) bool {
	if s.stats == nil || s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return false
	}
	return true
}

// handleStatsOverview returns the dashboard summary.
func (s *Server) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	o, err := s.stats.Overview(s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleStatsPilots returns per-player statistics.
func (s *Server) handleStatsPilots(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	list, err := s.stats.Pilots(s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "pilots": list})
}

// handleStatsWeapons returns weapon performance.
func (s *Server) handleStatsWeapons(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	list, err := s.stats.Weapons(s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "weapons": list})
}

// handleStatsEngines returns per-unit-type performance.
func (s *Server) handleStatsEngines(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	list, err := s.stats.Engines(s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "engines": list})
}

// handleStatsNetwork returns connection quality.
func (s *Server) handleStatsNetwork(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	list, err := s.stats.Network(s.scopeFromRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "network": list})
}
