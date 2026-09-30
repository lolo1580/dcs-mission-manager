package api

// ApplyMissionOptions records the mission's view/difficulty options.
//
// It is called from the TCP listener whenever DCS announces a mission with its
// options. The options are kept with the session so the recorded data carries
// the mission's own settings; nothing is filtered any more now that the live map
// has been removed.
//
// DCS nests the view options under "difficulty":
//
//	{ difficulty = { optionsView = "optview_all", ... }, ... }
func (s *Server) ApplyMissionOptions(options map[string]any) {
	if s.live != nil {
		s.live.SetOptions(options)
	}
}
