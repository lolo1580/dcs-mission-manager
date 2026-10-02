package api

import "net/http"

// handleMissions returns the mission library: the .miz found in the Saved Games
// missions folder, with the metadata DCS stores inside each one.
//
// Query: ?theatre=Caucasus to keep one theatre only.
func (s *Server) handleMissions(w http.ResponseWriter, r *http.Request) {
	theatre := r.URL.Query().Get("theatre")
	list := make([]any, 0, len(s.missions))
	for _, m := range s.missions {
		if theatre != "" && m.Theatre != theatre {
			continue
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"missions": list,
		"count":    len(list),
		"total":    len(s.missions),
	})
}
