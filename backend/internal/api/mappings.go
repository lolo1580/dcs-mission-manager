package api

import (
	"encoding/json"
	"net/http"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/mapping"
)

// handleMappings serves the panel-to-command bindings: GET the profiles, POST to
// replace one aircraft's, DELETE to remove one.
//
// Query: ?aircraft=F-16C_50 to name the profile.
func (s *Server) handleMappings(w http.ResponseWriter, r *http.Request) {
	if s.mappings == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mappings unavailable"})
		return
	}
	aircraft := r.URL.Query().Get("aircraft")

	switch r.Method {
	case http.MethodGet:
		if aircraft == "" {
			writeJSON(w, http.StatusOK, map[string]any{
				"enabled": s.mappings.Enabled(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"profile": s.mappings.Profile(aircraft),
			"enabled": s.mappings.Enabled(),
		})

	case http.MethodPost, http.MethodPut:
		if aircraft == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aircraft is required"})
			return
		}
		var body struct {
			Bindings []mapping.Binding `json:"bindings"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
			return
		}
		if err := s.mappings.SetProfile(mapping.Profile{Aircraft: aircraft, Bindings: body.Bindings}); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profile": s.mappings.Profile(aircraft)})

	case http.MethodDelete:
		if aircraft == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aircraft is required"})
			return
		}
		if err := s.mappings.DeleteProfile(aircraft); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET, POST or DELETE"})
	}
}

// handleMappingSafety turns command sending on or off. It is a separate endpoint
// with its own method, so the switch cannot be flipped by a stray profile save.
func (s *Server) handleMappingSafety(w http.ResponseWriter, r *http.Request) {
	if s.mappings == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mappings unavailable"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
		return
	}
	if err := s.mappings.SetEnabled(body.Enabled); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.mappings.Enabled()})
}

// handleControls serves the DCS-BIOS control catalogue of one aircraft, which is
// what a mapping editor offers: the commands a control can be bound to.
//
// Query: ?aircraft=F-16C_50 (required), ?writable=1 to keep only the commands.
func (s *Server) handleControls(w http.ResponseWriter, r *http.Request) {
	aircraft := r.URL.Query().Get("aircraft")
	if aircraft == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aircraft is required"})
		return
	}

	// The catalogue is cached: it is a large file and it changes only when
	// DCS-BIOS rewrites it (on a DCS update).
	cat, err := s.controlsFor(aircraft)
	if err != nil {
		// An aircraft with no metadata is not an error to the caller: it just has
		// no bindable controls (DCS running an aircraft DCS-BIOS does not document).
		writeJSON(w, http.StatusOK, map[string]any{
			"aircraft":  aircraft,
			"available": false,
			"controls":  []any{},
			"count":     0,
		})
		return
	}

	controls := cat.Controls
	if r.URL.Query().Get("writable") == "1" {
		controls = cat.Writable()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"aircraft":   aircraft,
		"available":  true,
		"module":     cat.Module,
		"categories": cat.Categories(),
		"controls":   controls,
		"count":      len(controls),
	})
}

// controlsFor returns an aircraft's catalogue, caching it.
func (s *Server) controlsFor(aircraft string) (*biosmeta.Catalog, error) {
	s.controlsMu.Lock()
	defer s.controlsMu.Unlock()
	if s.controls == nil {
		s.controls = map[string]*biosmeta.Catalog{}
	}
	if cat, ok := s.controls[aircraft]; ok {
		return cat, nil
	}
	cat, err := biosmeta.LoadModule(s.cfg.SavedGames, aircraft)
	if err != nil {
		return nil, err
	}
	s.controls[aircraft] = cat
	return cat, nil
}
