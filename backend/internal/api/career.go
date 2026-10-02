package api

import "net/http"

// handleCareer returns the player's logbook: career totals and a per-airframe
// breakdown (flight hours, landings, deaths, kills), read from DCS's own
// MissionEditor/logbook.lua.
func (s *Server) handleCareer(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"currentPlayer": s.logbook.CurrentPlayer,
		"players":       s.logbook.Players,
		"count":         len(s.logbook.Players),
	})
}
