// Package flightplan contains the flight plan model, its navlog calculations
// and the printable DCS briefing.
package flightplan

import "math"

// EarthRadiusNM is the mean Earth radius in nautical miles.
const EarthRadiusNM = 3440.065

// Waypoint is one point of the route. The departure is the first point and the
// arrival the last, so a plan always describes a complete route.
type Waypoint struct {
	Ident  string  `json:"ident"`
	Name   string  `json:"name,omitempty"`
	Kind   string  `json:"kind"` // airport | user
	ICAO   string  `json:"icao,omitempty"`
	Source string  `json:"source,omitempty"` // ourairports | dcs
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	// ElevationFt is the airport elevation when known; user points leave it 0.
	ElevationFt int `json:"elevationFt,omitempty"`
}

// Plan is a saved flight plan. The same route feeds the navlog and the
// printable DCS briefing.
type Plan struct {
	ID   int64  `json:"id"`
	UID  string `json:"uid"`
	Name string `json:"name"`
	// Revision increments on every save; it will let two databases detect
	// concurrent edits when synchronisation is enabled.
	Revision int64 `json:"revision"`
	// OwnerID is the server account that owns the plan. It stays 0 on the
	// desktop, where a single local player owns everything.
	OwnerID int64 `json:"ownerId,omitempty"`
	// Visibility is "private" or "squadron" on a server.
	Visibility string `json:"visibility"`
	// DeletedAt is a tombstone used by synchronisation; 0 means alive.
	DeletedAt int64 `json:"deletedAt,omitempty"`
	// SyncRevision is the server-wide sequence used by pull requests.
	SyncRevision int64      `json:"syncRevision,omitempty"`
	Aircraft     string     `json:"aircraft,omitempty"`
	Departure    string     `json:"departure,omitempty"`
	Arrival      string     `json:"arrival,omitempty"`
	CruiseAltFt  int        `json:"cruiseAltFt"`
	CruiseTASKt  int        `json:"cruiseTasKt"`
	FuelBurnPPH  float64    `json:"fuelBurnPph"`
	ReserveMin   int        `json:"reserveMin"`
	WindFromDeg  int        `json:"windFromDeg"`
	WindKt       int        `json:"windKt"`
	FlightRules  string     `json:"flightRules"` // VFR | IFR
	Notes        string     `json:"notes,omitempty"`
	Route        []Waypoint `json:"route"`
	CreatedAt    int64      `json:"createdAt,omitempty"`
	UpdatedAt    int64      `json:"updatedAt,omitempty"`
}

// Leg is the computed navigation log for one branch of the route.
type Leg struct {
	From          string  `json:"from"`
	To            string  `json:"to"`
	DistanceNM    float64 `json:"distanceNm"`
	CourseTrue    float64 `json:"courseTrue"`
	HeadingTrue   float64 `json:"headingTrue"`
	GroundSpeedKt float64 `json:"groundSpeedKt"`
	TimeMin       float64 `json:"timeMin"`
	Fuel          float64 `json:"fuel"`
}

// Navlog is the full result returned to the UI and used by the exports.
type Navlog struct {
	Legs            []Leg   `json:"legs"`
	TotalDistanceNM float64 `json:"totalDistanceNm"`
	TotalTimeMin    float64 `json:"totalTimeMin"`
	TotalFuel       float64 `json:"totalFuel"`
	ReserveFuel     float64 `json:"reserveFuel"`
	RequiredFuel    float64 `json:"requiredFuel"`
	AverageGS       float64 `json:"averageGs"`
}

// Calculate builds the navlog. Distance and course use great-circle geometry;
// the wind triangle corrects the heading and the ground speed.
func Calculate(p Plan) Navlog {
	nav := Navlog{Legs: []Leg{}}
	for i := 0; i+1 < len(p.Route); i++ {
		from, to := p.Route[i], p.Route[i+1]
		distance := DistanceNM(from.Lat, from.Lon, to.Lat, to.Lon)
		course := BearingTrue(from.Lat, from.Lon, to.Lat, to.Lon)
		heading, groundSpeed := WindTriangle(course, float64(p.CruiseTASKt), float64(p.WindFromDeg), float64(p.WindKt))
		minutes, fuel := 0.0, 0.0
		if groundSpeed > 0 {
			minutes = distance / groundSpeed * 60
			fuel = p.FuelBurnPPH * minutes / 60
		}
		nav.Legs = append(nav.Legs, Leg{
			From:          label(from),
			To:            label(to),
			DistanceNM:    distance,
			CourseTrue:    course,
			HeadingTrue:   heading,
			GroundSpeedKt: groundSpeed,
			TimeMin:       minutes,
			Fuel:          fuel,
		})
		nav.TotalDistanceNM += distance
		nav.TotalTimeMin += minutes
		nav.TotalFuel += fuel
	}
	nav.ReserveFuel = p.FuelBurnPPH * float64(p.ReserveMin) / 60
	nav.RequiredFuel = nav.TotalFuel + nav.ReserveFuel
	if nav.TotalTimeMin > 0 {
		nav.AverageGS = nav.TotalDistanceNM / (nav.TotalTimeMin / 60)
	}
	return nav
}

// DistanceNM is the great-circle distance between two coordinates.
func DistanceNM(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := radians(lat1), radians(lat2)
	dPhi := phi2 - phi1
	dLambda := radians(lon2 - lon1)
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	// Rounding can push the haversine just outside [0,1] for nearly antipodal
	// points; clamp before the square roots so the result is never NaN.
	a = math.Max(0, math.Min(1, a))
	return 2 * EarthRadiusNM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// BearingTrue is the initial great-circle course in degrees true.
func BearingTrue(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := radians(lat1), radians(lat2)
	dLambda := radians(lon2 - lon1)
	y := math.Sin(dLambda) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(dLambda)
	return normalize360(degrees(math.Atan2(y, x)))
}

// WindTriangle returns the true heading to fly and the resulting ground speed.
// windFrom is the direction the wind comes from, in degrees true. When the
// crosswind exceeds the true airspeed, no heading can hold the course; the
// aircraft is reported with a zero ground speed rather than a fake solution.
func WindTriangle(courseTrue, tasKt, windFrom, windKt float64) (heading, groundSpeed float64) {
	if tasKt <= 0 {
		return normalize360(courseTrue), 0
	}
	relative := radians(windFrom - courseTrue)
	crosswind := windKt * math.Sin(relative)
	headwind := windKt * math.Cos(relative)
	ratio := crosswind / tasKt
	if ratio > 1 || ratio < -1 {
		return normalize360(courseTrue), 0
	}
	wca := degrees(math.Asin(ratio))
	groundSpeed = tasKt*math.Cos(radians(wca)) - headwind
	if groundSpeed < 0 {
		groundSpeed = 0
	}
	return normalize360(courseTrue + wca), groundSpeed
}

func label(w Waypoint) string {
	if w.Ident != "" {
		return w.Ident
	}
	if w.Name != "" {
		return w.Name
	}
	return "WP"
}

func radians(degrees float64) float64 { return degrees * math.Pi / 180 }

func degrees(radians float64) float64 { return radians * 180 / math.Pi }

func normalize360(angle float64) float64 {
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}
	// Tiny negative crosswind components produce -1e-16; without this the
	// heading would be reported as 360° instead of 0°.
	if angle > 360-1e-9 || angle < 1e-9 {
		return 0
	}
	return angle
}
