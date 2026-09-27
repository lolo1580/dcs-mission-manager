package api

import (
	"net/http"
	"strings"
)

// handleVectors lists the GeoJSON terrain layers imported for a theatre, so the
// UI knows what can be drawn on top of the map.
//
// Query: ?theatre=Caucasus to filter.
func (s *Server) handleVectors(w http.ResponseWriter, r *http.Request) {
	theatre := strings.TrimSpace(r.URL.Query().Get("theatre"))
	if s.vectors == nil {
		writeJSON(w, http.StatusOK, map[string]any{"theatres": []string{}, "layers": []any{}})
		return
	}
	if theatre != "" {
		layers := s.vectors.ByTheatre(theatre)
		writeJSON(w, http.StatusOK, map[string]any{
			"theatre": theatre,
			"layers":  layers,
			"count":   len(layers),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"theatres": s.vectors.Theatres(),
		"count":    s.vectors.Count(),
	})
}

// handleVectorFile serves one layer as GeoJSON. The layer is addressed by theatre
// and name, and only a name the catalogue indexed resolves — never a path.
func (s *Server) handleVectorFile(w http.ResponseWriter, r *http.Request) {
	if s.vectors == nil {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/vectors/file/")
	// theatre/name.geojson
	parts := strings.SplitN(strings.Trim(rest, "/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	theatre := parts[0]
	name := strings.TrimSuffix(parts[1], ".geojson")
	if name == "" || strings.ContainsAny(theatre, `/\`) || strings.ContainsAny(name, `/\`) {
		http.NotFound(w, r)
		return
	}
	full, ok := s.vectors.Resolve(theatre, name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/geo+json")
	// The layers change only when the user re-imports them.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, full)
}
