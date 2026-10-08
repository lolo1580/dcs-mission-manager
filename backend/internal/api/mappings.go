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
				"outputs": s.mappings.OutputsEnabled(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"profile": s.mappings.Profile(aircraft),
			"enabled": s.mappings.Enabled(),
			"outputs": s.mappings.OutputsEnabled(),
		})

	case http.MethodPost, http.MethodPut:
		if aircraft == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aircraft is required"})
			return
		}
		var body struct {
			Bindings []mapping.Binding        `json:"bindings"`
			Outputs  []mapping.OutputBinding  `json:"outputs"`
			Displays []mapping.DisplayBinding `json:"displays"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
			return
		}
		// Outputs and displays drive the panels' LEDs and LCD. A client that does not
		// know one of them omits the field: keep what exists rather than wiping it.
		existing := s.mappings.Profile(aircraft)
		outputs := existing.Outputs
		if body.Outputs != nil {
			outputs = body.Outputs
		}
		displays := existing.Displays
		if body.Displays != nil {
			displays = body.Displays
		}
		if err := s.mappings.SetProfile(mapping.Profile{Aircraft: aircraft, Bindings: body.Bindings, Outputs: outputs, Displays: displays}); err != nil {
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
	if !body.Enabled && !s.TestMode() && s.panelPlugin != nil {
		_ = s.panelPlugin.SetTrimEnabled(false)
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

// LoadCatalog returns an aircraft's DCS-BIOS catalogue, caching it. It is the
// public form of controlsFor, so other parts of the manager (the LED driver) read
// the same metadata without each loading its own copy.
func (s *Server) LoadCatalog(aircraft string) (*biosmeta.Catalog, error) {
	return s.controlsFor(aircraft)
}

// handleOutputsSafety turns the panel outputs (LEDs, LCD) on or off. It is separate
// from the command-sending switch: outputs only read DCS-BIOS and write the panels,
// they never command the aircraft, so turning them on needs no transport.
func (s *Server) handleOutputsSafety(w http.ResponseWriter, r *http.Request) {
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
	s.mappings.SetOutputsEnabled(body.Enabled)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.mappings.OutputsEnabled()})
}

// handleMappingTest turns the live mapping test on or off. It is a separate
// endpoint from the sending switch: the test can send to DCS-BIOS to verify a
// binding, and it resets on restart.
func (s *Server) handleMappingTest(w http.ResponseWriter, r *http.Request) {
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
	s.SetTestMode(body.Enabled)
	if !body.Enabled && (s.mappings == nil || !s.mappings.Enabled()) && s.panelPlugin != nil {
		_ = s.panelPlugin.SetTrimEnabled(false)
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.TestMode()})
}

// handleAircraft lists the aircraft names that have a profile to edit.
func (s *Server) handleAircraft(w http.ResponseWriter, _ *http.Request) {
	if s.mappings == nil {
		writeJSON(w, http.StatusOK, map[string]any{"aircraft": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aircraft": s.mappings.Aircraft()})
}

// handleDebug reports and toggles the runtime debug switch.
//
// GET returns the state and the recorded lines; POST sets it. POST is a separate
// call from GET so a page load never flips it.
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
	cat = biosmeta.WithPanelPlugin(cat)
	s.controls[aircraft] = cat
	return cat, nil
}
