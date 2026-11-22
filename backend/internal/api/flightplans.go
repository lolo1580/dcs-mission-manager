package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"dcsmanager/internal/airportdata"
	"dcsmanager/internal/auth"
	"dcsmanager/internal/flightplan"
)

// handleAirports searches the embedded OurAirports dataset and, for the chosen
// DCS theatre, the simulator's own airfields. DCS entries win on a shared ICAO
// because they carry the exact tower/TACAN/ILS frequencies.
func (s *Server) handleAirports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 1 {
		writeJSON(w, http.StatusOK, map[string]any{"airports": []any{}, "count": 0})
		return
	}
	catalog, err := airportdata.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	limit := 30
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 100 {
		limit = n
	}

	base := catalog.Search(query, limit)
	theatre := r.URL.Query().Get("theatre")
	if theatre == "" {
		theatre = s.cfg.Theatre
	}
	var extras []airportdata.Result
	if s.aerodromes != nil {
		for _, field := range s.aerodromes.ByTheatre(theatre) {
			item := airportdata.Airport{
				Ident:        field.ID,
				ICAO:         strings.ToUpper(field.ICAOCode),
				Name:         field.Name,
				Type:         "medium",
				Lat:          field.Lat,
				Lon:          field.Lng,
				ElevationFt:  int(math.Round(field.ElevationM * 3.28084)),
				Country:      "",
				Municipality: field.Theatre,
				Source:       "dcs",
				Theatre:      field.Theatre,
				Tower:        field.Tower,
				TACAN:        field.TACAN,
			}
			for _, ils := range field.ILS {
				item.ILS = append(item.ILS, airportdata.ILS{Runway: ils.Runway, MHz: ils.MHz})
			}
			score := airportdata.ScoreQuery(query, item)
			if score > 0 {
				extras = append(extras, airportdata.Result{Airport: item, Score: score})
			}
		}
	}
	merged := airportdata.Dedupe(base, extras, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"airports":  merged,
		"count":     len(merged),
		"generated": catalog.Generated(),
		"theatre":   theatre,
	})
}

func (s *Server) handleFlightPlans(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/flightplans"), "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			s.listFlightPlans(w, r)
		case http.MethodPost:
			s.saveFlightPlan(w, r, 0)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		}
		return
	}
	if path == "calculate" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
			return
		}
		s.calculateFlightPlan(w, r)
		return
	}

	parts := strings.Split(path, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown flight plan"})
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.getFlightPlan(w, r, id)
		case http.MethodPut, http.MethodPost:
			s.saveFlightPlan(w, r, id)
		case http.MethodDelete:
			s.deleteFlightPlan(w, r, id)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET, PUT or DELETE"})
		}
		return
	}
	if len(parts) == 2 && parts[1] == "briefing" && r.Method == http.MethodGet {
		s.briefingFlightPlan(w, r, id)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown flight plan resource"})
}

// documentScope returns the account id and administrator flag used to filter
// documents. The desktop has no accounts: 0/true means "everything is local".
func (s *Server) documentScope(r *http.Request) (int64, bool) {
	if s.Mode() != "server" {
		return 0, true
	}
	if user, ok := currentUser(r); ok {
		return user.ID, user.Role == auth.RoleAdmin
	}
	return 0, false
}

func (s *Server) listFlightPlans(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "plans": []any{}})
		return
	}
	ownerID, admin := s.documentScope(r)
	plans, err := s.db.FlightPlansFor(ownerID, admin, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// The sidebar only needs the labels. Routes can be hundreds of waypoints;
	// the full plan is fetched when the user selects it.
	for i := range plans {
		plans[i].Route = nil
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "plans": plans, "count": len(plans)})
}

func (s *Server) getFlightPlan(w http.ResponseWriter, r *http.Request, id int64) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "persistence disabled"})
		return
	}
	ownerID, admin := s.documentScope(r)
	plan, err := s.db.FlightPlanFor(id, ownerID, admin)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) saveFlightPlan(w http.ResponseWriter, r *http.Request, id int64) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "persistence disabled"})
		return
	}
	ownerID, admin := s.documentScope(r)
	var plan flightplan.Plan
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&plan); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed flight plan"})
		return
	}
	if id != 0 {
		plan.ID = id
		// Updating requires seeing the existing plan; this keeps a member from
		// editing another member's private plan by guessing its id.
		existing, err := s.db.FlightPlanFor(id, ownerID, admin)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
			return
		}
		if !canEditDocument(ownerID, admin, existing.OwnerID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the owner can edit this flight plan"})
			return
		}
	}
	if err := validateFlightPlan(&plan, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	saved, err := s.db.SaveFlightPlanFor(ownerID, plan)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteFlightPlan(w http.ResponseWriter, r *http.Request, id int64) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "persistence disabled"})
		return
	}
	ownerID, admin := s.documentScope(r)
	existing, err := s.db.FlightPlanFor(id, ownerID, admin)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !canEditDocument(ownerID, admin, existing.OwnerID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the owner can delete this flight plan"})
		return
	}
	if err := s.db.DeleteFlightPlanFor(id, ownerID, admin); errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (s *Server) calculateFlightPlan(w http.ResponseWriter, r *http.Request) {
	var plan flightplan.Plan
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&plan); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed flight plan"})
		return
	}
	if err := validateFlightPlan(&plan, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"navlog": flightplan.Calculate(plan)})
}

func (s *Server) briefingFlightPlan(w http.ResponseWriter, r *http.Request, id int64) {
	ownerID, admin := s.documentScope(r)
	plan, ok := s.loadFlightPlan(w, id, ownerID, admin)
	if !ok {
		return
	}
	nav := flightplan.Calculate(plan)
	body, err := flightplan.BriefingHTML(plan, nav, r.URL.Query().Get("lang"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func (s *Server) loadFlightPlan(w http.ResponseWriter, id, ownerID int64, admin bool) (flightplan.Plan, bool) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "persistence disabled"})
		return flightplan.Plan{}, false
	}
	plan, err := s.db.FlightPlanFor(id, ownerID, admin)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flight plan not found"})
		return flightplan.Plan{}, false
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return flightplan.Plan{}, false
	}
	return plan, true
}

func validateFlightPlan(plan *flightplan.Plan, requireName bool) error {
	plan.Name = strings.TrimSpace(plan.Name)
	if requireName && plan.Name == "" {
		return errors.New("a flight plan needs a name")
	}
	if len(plan.Route) < 2 {
		return errors.New("add a departure and an arrival")
	}
	if len(plan.Route) > 200 {
		return errors.New("too many waypoints (200 maximum)")
	}
	for i, w := range plan.Route {
		if math.IsNaN(w.Lat) || math.IsInf(w.Lat, 0) || w.Lat < -90 || w.Lat > 90 {
			return fmt.Errorf("waypoint %d: invalid latitude", i+1)
		}
		if math.IsNaN(w.Lon) || math.IsInf(w.Lon, 0) || w.Lon < -180 || w.Lon > 180 {
			return fmt.Errorf("waypoint %d: invalid longitude", i+1)
		}
		if w.Kind == "" {
			plan.Route[i].Kind = "user"
		}
	}
	if plan.CruiseAltFt < 0 || plan.CruiseAltFt > 70000 {
		return errors.New("cruise altitude must be between 0 and 70000 ft")
	}
	if plan.CruiseTASKt < 0 || plan.CruiseTASKt > 2000 {
		return errors.New("TAS must be between 0 and 2000 kt")
	}
	if plan.FuelBurnPPH < 0 || plan.FuelBurnPPH > 1000000 {
		return errors.New("invalid fuel burn")
	}
	if plan.ReserveMin < 0 || plan.ReserveMin > 1440 {
		return errors.New("reserve must be between 0 and 1440 minutes")
	}
	if plan.WindFromDeg < 0 {
		plan.WindFromDeg = 0
	}
	plan.WindFromDeg %= 360
	if plan.WindKt < 0 || plan.WindKt > 300 {
		return errors.New("wind must be between 0 and 300 kt")
	}
	switch strings.ToUpper(plan.FlightRules) {
	case "", "VFR":
		plan.FlightRules = "VFR"
	case "IFR":
		plan.FlightRules = "IFR"
	default:
		return errors.New("flight rules must be VFR or IFR")
	}
	switch strings.ToLower(plan.Visibility) {
	case "", "private":
		plan.Visibility = "private"
	case "squadron":
		plan.Visibility = "squadron"
	default:
		return errors.New("visibility must be private or squadron")
	}
	plan.Departure = strings.TrimSpace(plan.Departure)
	plan.Arrival = strings.TrimSpace(plan.Arrival)
	if plan.Departure == "" {
		plan.Departure = plan.Route[0].Ident
	}
	if plan.Arrival == "" {
		plan.Arrival = plan.Route[len(plan.Route)-1].Ident
	}
	return nil
}
