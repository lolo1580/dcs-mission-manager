package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleCharts lists the aeronautical charts available on disk.
//
// Query: ?theatre=Caucasus to filter, ?icao=UG5X&name=Batumi to get the charts
// of one airfield (matched by file name).
func (s *Server) handleCharts(w http.ResponseWriter, r *http.Request) {
	if s.charts == nil {
		writeJSON(w, http.StatusOK, map[string]any{"charts": []any{}, "count": 0})
		return
	}

	q := r.URL.Query()
	icao, name := q.Get("icao"), q.Get("name")

	var list []any
	switch {
	case icao != "" || name != "":
		for _, c := range s.charts.ForAerodrome(icao, name) {
			list = append(list, c)
		}
	case q.Get("theatre") != "":
		for _, c := range s.charts.ByTheatre(q.Get("theatre")) {
			list = append(list, c)
		}
	default:
		for _, th := range s.charts.Theatres() {
			for _, c := range s.charts.ByTheatre(th) {
				list = append(list, c)
			}
		}
	}
	if list == nil {
		list = []any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":    len(list),
		"theatres": s.charts.Theatres(),
		"charts":   list,
	})
}

// handleChartFile serves one chart image.
//
// It serves only files present in the index: the relative path from the URL is
// looked up in the catalogue, which was built by walking the charts folder, so a
// crafted path cannot reach any other file. Traversal is rejected outright as
// well, for defence in depth.
func (s *Server) handleChartFile(w http.ResponseWriter, r *http.Request) {
	if s.charts == nil {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/api/charts/file/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") || strings.Contains(rel, `\`) {
		http.NotFound(w, r)
		return
	}

	ch, ok := s.charts.Get(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// ch.Path comes from the index, so it is a real relative path inside the
	// charts folder; joining it is safe.
	full := filepath.Join(s.chartsDir, filepath.FromSlash(ch.Path))
	if _, err := os.Stat(full); err != nil {
		http.NotFound(w, r)
		return
	}

	// Charts are scans: let the browser cache them, and always inline them so
	// they display rather than download.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, full)
}
