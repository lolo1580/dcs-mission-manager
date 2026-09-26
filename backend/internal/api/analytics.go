package api

import (
	"net/http"
	"strconv"

	"dcsmm/internal/tracker"
)

// analyticsMissionID returns the mission to analyse: ?missionId=N, otherwise the
// open mission. 0 means "all missions".
func (s *Server) analyticsMissionID(r *http.Request) int64 {
	q := r.URL.Query()
	if q.Get("all") == "1" {
		return 0
	}
	if id, err := strconv.ParseInt(q.Get("missionId"), 10, 64); err == nil && id > 0 {
		return id
	}
	if s.db != nil {
		if id := s.db.OpenMissionID(); id != 0 {
			return id
		}
		if list, err := s.db.Missions(1); err == nil && len(list) > 0 {
			return list[0].ID
		}
	}
	return 0
}

// handleHeatmap returns aggregated positions or losses.
//
// Query: ?source=positions|losses, ?grid=0.05 (degrees), ?limit=2000.
func (s *Server) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"points": []any{}})
		return
	}
	q := r.URL.Query()
	source := q.Get("source")
	if source == "" {
		source = "positions"
	}
	if source != "positions" && source != "losses" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source invalide"})
		return
	}
	grid := 0.05
	if v, err := strconv.ParseFloat(q.Get("grid"), 64); err == nil && v > 0 {
		grid = v
	}
	limit := 2000
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = v
	}

	points, err := s.db.Heatmap(s.analyticsMissionID(r), source, grid, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source": source,
		"grid":   grid,
		"count":  len(points),
		"points": points,
	})
}

// handleTracks returns flight trails for the most active aircraft, plus per-unit
// sortie statistics.
func (s *Server) handleTracks(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"trails": map[string]any{}, "stats": []any{}})
		return
	}
	q := r.URL.Query()
	limitUnits := 10
	if v, err := strconv.Atoi(q.Get("units")); err == nil && v > 0 {
		limitUnits = v
	}

	trails, err := s.db.Trails(s.analyticsMissionID(r), limitUnits, 400)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trails": trails,
		"stats":  tracker.Analyse(trails),
	})
}

// handleSorties returns per-unit sortie statistics only.
func (s *Server) handleSorties(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"stats": []any{}})
		return
	}
	trails, err := s.db.Trails(s.analyticsMissionID(r), 50, 1000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stats := tracker.Analyse(trails)
	writeJSON(w, http.StatusOK, map[string]any{"count": len(stats), "stats": stats})
}
