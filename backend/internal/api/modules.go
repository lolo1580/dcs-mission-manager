package api

import "net/http"

// handleModules returns DCS's own module inventory (terrains, aircraft,
// campaigns, tech packs), read from the Saved Games installation at startup.
//
// Query: ?owned=1 to keep only the modules the player owns.
func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("owned") == "1" {
		owned := make([]any, 0, s.modules.Owned)
		for _, m := range s.modules.Modules {
			if m.Owned {
				owned = append(owned, m)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"modules": owned,
			"count":   len(owned),
			"owned":   len(owned),
			"total":   s.modules.Total,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"modules": s.modules.Modules,
		"count":   len(s.modules.Modules),
		"owned":   s.modules.Owned,
		"total":   s.modules.Total,
	})
}
