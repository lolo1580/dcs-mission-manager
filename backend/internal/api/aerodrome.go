package api

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"dcsmm/internal/aerodrome"
)

// handleAerodromes returns the airfield reference data.
//
// Query: ?theatre=Caucasus to filter, ?lat=&lng= to annotate distances and sort
// by proximity (useful to find the nearest field).
func (s *Server) handleAerodromes(w http.ResponseWriter, r *http.Request) {
	if s.aerodromes == nil {
		writeJSON(w, http.StatusOK, map[string]any{"aerodromes": []any{}})
		return
	}

	q := r.URL.Query()
	theatre := q.Get("theatre")

	var list []aerodrome.Aerodrome
	if theatre != "" {
		list = s.aerodromes.ByTheatre(theatre)
	} else {
		for _, th := range s.aerodromes.Theatres() {
			list = append(list, s.aerodromes.ByTheatre(th)...)
		}
	}

	// Annotate distances when a reference position is provided.
	if lat, ok1 := parseFloat(q.Get("lat")); ok1 {
		if lng, ok2 := parseFloat(q.Get("lng")); ok2 {
			for i := range list {
				list[i].DistanceKm = haversineKm(lat, lng, list[i].Lat, list[i].Lng)
			}
			sortByDistance(list)
		}
	}

	// Report where the data came from: "dcs" when read from the simulator's own
	// terrain files, "embedded" when served from the dataset in the binary.
	source := aerodrome.SourceEmbedded
	for _, a := range list {
		if a.Source == aerodrome.SourceDCS {
			source = aerodrome.SourceDCS
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":      len(list),
		"theatres":   s.aerodromes.Theatres(),
		"source":     source,
		"aerodromes": list,
	})
}

// handleTowns returns the settlements of a theatre, read from DCS's
// map/towns.lua. They are real, geolocated place names, useful as a map layer
// and available with no mission data at all.
//
// Query: ?theatre=Caucasus (required).
func (s *Server) handleTowns(w http.ResponseWriter, r *http.Request) {
	if s.aerodromes == nil {
		writeJSON(w, http.StatusOK, map[string]any{"towns": []any{}})
		return
	}
	theatre := r.URL.Query().Get("theatre")
	towns := s.aerodromes.Towns(theatre)
	writeJSON(w, http.StatusOK, map[string]any{
		"theatre": theatre,
		"count":   len(towns),
		"towns":   towns,
	})
}

// handleAerodrome returns one airfield by id (case-insensitive).
func (s *Server) handleAerodrome(w http.ResponseWriter, r *http.Request) {
	if s.aerodromes == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "data unavailable"})
		return
	}
	id := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/api/aerodromes/"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	a, ok := s.aerodromes.ByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "airfield not found"})
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	la1 := lat1 * math.Pi / 180
	la2 := lat2 * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(la1)*math.Cos(la2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func sortByDistance(list []aerodrome.Aerodrome) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].DistanceKm < list[j].DistanceKm })
}
