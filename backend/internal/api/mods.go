package api

import "net/http"

// handleScripts returns the state of the DCS side: the manager's own managed
// files (installed / outdated / missing), the tools merged into Export.lua, and
// other tools' hook files.
func (s *Server) handleScripts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.scripts)
}

// handleMods returns the mods installed under Saved Games\DCS\Mods.
func (s *Server) handleMods(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mods":  s.mods,
		"count": len(s.mods),
	})
}
