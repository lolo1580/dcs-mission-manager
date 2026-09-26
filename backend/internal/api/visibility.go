package api

import (
	"log"

	"dcsmm/internal/visibility"
)

// ApplyMissionOptions records the mission's view/difficulty options and updates
// the fog-of-war policy. It is called from the TCP listener whenever DCS
// announces a mission with its options.
//
// DCS exposes the whole `mission.options` table; the field that matters for
// visibility is `optionsView`. The complete option set is stored in the live
// session so it can be surfaced in the UI later.
func (s *Server) ApplyMissionOptions(options map[string]any) {
	if s.live != nil {
		s.live.SetOptions(options)
	}

	raw, _ := options["optionsView"].(string)
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
