package api

import "net/http"

// handleDCSConfig returns DCS's own configuration: the options.lua groups
// (graphics, difficulty, VR, sound…), the pluginsEnabled.lua toggles, the
// manager's own config, and the UI language.
//
// Query: ?section=graphics to return one section only.
func (s *Server) handleDCSConfig(w http.ResponseWriter, r *http.Request) {
	wanted := r.URL.Query().Get("section")
	if wanted == "" {
		writeJSON(w, http.StatusOK, s.dcsConfig)
		return
	}
	for _, sec := range s.dcsConfig.Sections {
		if sec.Name == wanted {
			writeJSON(w, http.StatusOK, map[string]any{
				"name":     sec.Name,
				"settings": sec.Settings,
				"count":    len(sec.Settings),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": wanted, "settings": []any{}, "count": 0})
}
