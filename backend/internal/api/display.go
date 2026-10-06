package api

import (
	"encoding/json"
	"net/http"

	"dcsmanager/internal/led"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panel"
)

// handleDisplayPreview resolves a PZ70 display binding against the current DCS-BIOS
// memory and returns what the LCD would show: the raw exported value, the converted
// value and the formatted line. It touches no hardware — it is the "does this source
// read what I think?" check for the display editor.
//
// POST body: {aircraft, command, export, scale, offset, line}.
func (s *Server) handleDisplayPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var body struct {
		Aircraft string  `json:"aircraft"`
		Command  string  `json:"command"`
		Export   int     `json:"export"`
		Scale    float64 `json:"scale"`
		Offset   float64 `json:"offset"`
		Line     string  `json:"line"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
		return
	}
	if body.Aircraft == "" {
		body.Aircraft = s.biosAircraft()
	}
	if body.Aircraft == "" {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "no aircraft"})
		return
	}
	if s.bios == nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "no DCS-BIOS"})
		return
	}
	cat, err := s.controlsFor(body.Aircraft)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "no metadata"})
		return
	}
	ctrl, ok := cat.ByID(body.Command)
	if !ok || body.Export < 0 || body.Export >= len(ctrl.Outputs) {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "no such export"})
		return
	}
	raw, ok := led.ReadExportInt(ctrl.Outputs[body.Export], s.bios.Memory())
	if !ok {
		// The address has not been delivered yet: not an error, just "no value".
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "reason": "value not delivered"})
		return
	}
	value := led.Convert(raw, mapping.DisplayBinding{Scale: body.Scale, Offset: body.Offset})
	line := panel.LineUpper
	if body.Line == "lower" {
		line = panel.LineLower
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"raw":       raw,
		"value":     value,
		"text":      panel.FormatPZ70Line(line, value),
	})
}

// handleDisplayTest writes chosen numbers to every connected PZ70, so each of the
// LCD's thirteen cells can be confirmed on the real hardware before a value source
// is wired to it. It is independent of command sending.
//
// POST body: {upper, lower} (either may be omitted).
func (s *Server) handleDisplayTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var body struct {
		Upper *int `json:"upper"`
		Lower *int `json:"lower"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed body"})
		return
	}
	if s.panels == nil {
		writeJSON(w, http.StatusOK, map[string]any{"written": 0, "reason": "no panels"})
		return
	}
	report := panel.EncodePZ70Panel(panel.PZ70Panel{UpperDisplay: body.Upper, LowerDisplay: body.Lower})
	written := 0
	for _, d := range s.panels.Devices() {
		if model, _ := panel.ModelFor(d.VendorID, d.ProductID); model != panel.PZ70 {
			continue
		}
		if err := s.panels.Write(d.Path, report); err != nil {
			continue
		}
		written++
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": written})
}

// biosAircraft returns the active aircraft or "".
func (s *Server) biosAircraft() string {
	if s.bios == nil {
		return ""
	}
	return s.bios.State().Aircraft
}
