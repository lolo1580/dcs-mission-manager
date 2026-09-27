package api

import (
	"log"

	"dcsmm/internal/visibility"
)

// ApplyMissionOptions records the mission's view/difficulty options and updates
// the fog-of-war policy. It is called from the TCP listener whenever DCS
// announces a mission with its options.
//
// DCS nests the view options under "difficulty":
//
//	{ difficulty = { optionsView = "optview_all", ... }, ... }
//
// Looking for "optionsView" at the top level (as this used to) never matched, so
// the mode stayed "unknown" and the map stayed restricted no matter what the
// mission allowed. The flat form is still accepted, so a future build that
// flattens the table does not break it again.
func (s *Server) ApplyMissionOptions(options map[string]any) {
	if s.live != nil {
		s.live.SetOptions(options)
	}

	raw := optionString(options, "optionsView")
	if raw == "" {
		log.Printf("visibility: no optionsView in the mission options (restrictive default kept)")
	}
	mode := visibility.FromDCSOption(raw)

	s.visibility.SetMode(mode)
	if s.visibility.Override() {
		log.Printf("visibility: optionsView=%q -> %s (filtering disabled)", raw, mode)
		return
	}
	log.Printf("visibility: optionsView=%q -> %s", raw, mode)

	// Tell connected clients the policy changed, so the banner updates.
	s.BroadcastMessage(map[string]any{
		"type":       "visibility",
		"visibility": s.visibility.Describe(),
	})
}

// optionString reads a string option, looking inside "difficulty" first because
// that is where DCS puts the view settings, then at the top level.
func optionString(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	if diff, ok := options["difficulty"].(map[string]any); ok {
		if v, ok := diff[key].(string); ok && v != "" {
			return v
		}
	}
	if v, ok := options[key].(string); ok {
		return v
	}
	return ""
}
