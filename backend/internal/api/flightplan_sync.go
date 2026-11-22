package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/flightplan"
)

func (s *Server) documentScope(r *http.Request) (int64, bool) {
	if s.Mode() != "server" {
		return 0, true
	}
	if user, ok := currentUser(r); ok {
		return user.ID, user.Role == auth.RoleAdmin
	}
	return 0, false
}

// validateFlightPlan is kept for the server sync compatibility path. The
// flight-planning page and its public API were removed, but existing plans can
// still be exchanged with older desktop clients and remain in the database.
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
