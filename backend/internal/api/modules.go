package api

import "net/http"

// handleModules returns DCS's own module inventory (terrains, aircraft,
// campaigns, tech packs), read from the Saved Games installation at startup.
//
// "owned" is what the player bought (the store's have="1"); "installed" is what is
// actually on disk. They differ when a purchased module is uninstalled.
//
// Query: ?owned=1 keeps only the modules the player owns; ?installed=1 keeps only
// those present on disk.
func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("owned") == "1" || q.Get("installed") == "1" {
		ownedOnly := q.Get("owned") == "1"
		installedOnly := q.Get("installed") == "1"
		filtered := make([]any, 0, len(s.modules.Modules))
		for _, m := range s.modules.Modules {
			if ownedOnly && !m.Owned {
				continue
			}
			if installedOnly && !(m.InstallKnown && m.Installed) {
				continue
			}
			filtered = append(filtered, m)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"modules":   filtered,
			"count":     len(filtered),
			"owned":     s.modules.Owned,
			"installed": s.modules.Installed,
			"total":     s.modules.Total,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"modules":   s.modules.Modules,
		"count":     len(s.modules.Modules),
		"owned":     s.modules.Owned,
		"installed": s.modules.Installed,
		"total":     s.modules.Total,
	})
}
