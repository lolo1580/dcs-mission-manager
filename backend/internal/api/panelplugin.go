package api

import (
	"dcsmanager/internal/panelplugin"
	"encoding/json"
	"net/http"
)

func (s *Server) SetPanelPlugin(c *panelplugin.Client) { s.panelPlugin = c }
func (s *Server) handlePanelPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if s.panelPlugin == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "plugin unavailable"})
			return
		}
		var body struct {
			TrimEnabled *bool `json:"trimEnabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil || body.TrimEnabled == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trimEnabled requis"})
			return
		}
		if err := s.panelPlugin.SetTrimEnabled(*body.TrimEnabled); err != nil {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "state": s.panelPlugin.State()})
			return
		}
		writeJSON(w, http.StatusOK, s.panelPlugin.State())
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		return
	}
	if s.panelPlugin == nil {
		writeJSON(w, http.StatusOK, panelplugin.State{})
		return
	}
	writeJSON(w, http.StatusOK, s.panelPlugin.State())
}
