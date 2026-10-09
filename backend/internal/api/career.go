package api

import (
	"net/http"
	"time"

	"dcsmanager/internal/dcsdata"
)

// handleCareerInsights provides period breakdowns and a fixed 30-day activity
// calendar for one recorded pilot. It never includes simulated sessions.
func (s *Server) handleCareerInsights(w http.ResponseWriter, r *http.Request) {
	if !s.statsReady(w) {
		return
	}
	sc := s.scopeFromRequest(r)
	sc.Mode = "career"
	sc.IncludeTest = false
	insights, err := s.stats.CareerInsightsForPilot(sc, r.URL.Query().Get("ucid"), r.URL.Query().Get("name"), time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, insights)
}

// handleCareer returns the player's logbook: career totals and a per-airframe
// breakdown (flight hours, landings, deaths, kills), read from DCS's own
// MissionEditor/logbook.lua.
func (s *Server) handleCareer(w http.ResponseWriter, _ *http.Request) {
	// Re-read the logbook on every request: DCS rewrites it as the player flies,
	// so a copy read once at startup goes stale as soon as a session completes and
	// the career tab would never update. Fall back to the last-read copy when the
	// file is momentarily unreadable (DCS may be mid-write).
	lb := s.logbook
	if fresh, err := dcsdata.LoadLogbook(s.cfg.SavedGames); err == nil && (len(fresh.Players) > 0 || fresh.CurrentPlayer != "") {
		lb = fresh
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentPlayer": lb.CurrentPlayer,
		"players":       lb.Players,
		"count":         len(lb.Players),
	})
}
