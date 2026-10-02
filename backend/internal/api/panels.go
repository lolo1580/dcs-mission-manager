package api

import (
	"net/http"

	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/panel"
	"dcsmanager/internal/panelservice"
)

// PanelEventJSON renders a panel event for the SSE stream. The interface reads
// JSON, so the Go types are flattened to a stable shape rather than exposed raw.
func PanelEventJSON(e panelservice.Event) map[string]any {
	out := map[string]any{
		"kind":   panelKindName(e.Kind),
		"at":     e.At.UnixMilli(),
		"device": e.Device,
		"model":  string(e.Model),
	}
	switch e.Kind {
	case panelservice.KindConnected:
		out["info"] = map[string]any{
			"path":         e.Info.Path,
			"vendorId":     e.Info.VendorID,
			"productId":    e.Info.ProductID,
			"product":      e.Info.Product,
			"manufacturer": e.Info.Manufacturer,
			"serial":       e.Info.Serial,
		}
	case panelservice.KindInput:
		out["input"] = map[string]any{
			"control":   e.Input.Control.ID,
			"kind":      panelControlKindName(e.Input.Control.Kind),
			"active":    e.Input.Active,
			"clockwise": e.Input.Control.Clockwise,
		}
	case panelservice.KindError:
		if e.Err != nil {
			out["error"] = e.Err.Error()
		}
	}
	return out
}

func panelKindName(k panelservice.Kind) string {
	switch k {
	case panelservice.KindConnected:
		return "connected"
	case panelservice.KindDisconnected:
		return "disconnected"
	case panelservice.KindInput:
		return "input"
	case panelservice.KindError:
		return "error"
	}
	return "unknown"
}

func panelControlKindName(k panel.ControlKind) string {
	switch k {
	case panel.Toggle:
		return "toggle"
	case panel.Button:
		return "button"
	case panel.EncoderPulse:
		return "encoder"
	}
	return "unknown"
}

// SetPanels gives the server the panel service and the DCS-BIOS client, so the
// API can report their state and drive output.
func (s *Server) SetPanels(svc *panelservice.Service, bios *dcsbios.Client) {
	s.panels = svc
	s.bios = bios
}

// handlePanels reports the connected panels: model, identity, and whether the
// service is running at all.
func (s *Server) handlePanels(w http.ResponseWriter, _ *http.Request) {
	if s.panels == nil {
		writeJSON(w, http.StatusOK, map[string]any{"supported": false, "devices": []any{}})
		return
	}
	devices := s.panels.Devices()
	out := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		// The model is what the interface shows first, so it is part of the
		// payload rather than something the frontend has to infer from the ids.
		model, _ := panel.ModelFor(d.VendorID, d.ProductID)
		out = append(out, map[string]any{
			"path":         d.Path,
			"model":        string(model),
			"vendorId":     d.VendorID,
			"productId":    d.ProductID,
			"product":      d.Product,
			"manufacturer": d.Manufacturer,
			"serial":       d.Serial,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": true,
		"count":     len(out),
		"devices":   out,
		"bios":      s.biosState(),
	})
}

// handleDCSBIOS reports the DCS-BIOS link: whether frames are arriving and which
// aircraft is active.
func (s *Server) handleDCSBIOS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.biosState())
}

// biosState renders the DCS-BIOS state, or a "not running" shape when the client
// could not start (no listener, for instance).
func (s *Server) biosState() map[string]any {
	if s.bios == nil {
		return map[string]any{"available": false, "connected": false, "aircraft": ""}
	}
	st := s.bios.State()
	return map[string]any{
		"available": true,
		"connected": st.Connected,
		"aircraft":  st.Aircraft,
		"frames":    st.Frames,
	}
}
