package api

import (
	"encoding/json"
	"net/http"
)

// handleDebug exposes the runtime debug switch. GET reports it, POST sets it.
func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	if s.debug == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "lines": []any{}, "available": false})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":   s.debug.Enabled(),
			"available": true,
			"lines":     s.debug.Lines(),
		})
	case http.MethodPost:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
			return
		}
		s.debug.SetEnabled(body.Enabled)
		s.debug.Infof("debug", "debug logging %s", onOff(body.Enabled))
		writeJSON(w, http.StatusOK, map[string]any{"enabled": s.debug.Enabled()})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

// handleLog returns the in-memory debug log, or clears it. GET is the log the UI
// shows next to the switch; POST with {"clear":true} empties it.
func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	if s.debug == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "lines": []any{}})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": s.debug.Enabled(),
			"lines":   s.debug.Lines(),
		})
	case http.MethodPost:
		var body struct {
			Clear bool `json:"clear"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body)
		if body.Clear {
			s.debug.Clear()
		}
		writeJSON(w, http.StatusOK, map[string]any{"enabled": s.debug.Enabled(), "lines": s.debug.Lines()})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
