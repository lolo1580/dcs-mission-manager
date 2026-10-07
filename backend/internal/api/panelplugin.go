package api

import (
	"dcsmanager/internal/panelplugin"
	"net/http"
)

func (s *Server) SetPanelPlugin(c *panelplugin.Client) { s.panelPlugin = c }
func (s *Server) handlePanelPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.panelPlugin == nil {
		writeJSON(w, http.StatusOK, panelplugin.State{})
		return
	}
	writeJSON(w, http.StatusOK, s.panelPlugin.State())
}
