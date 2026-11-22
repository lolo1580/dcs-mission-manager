package flightplan

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"
)

func planTitle(p Plan) string {
	departure, arrival := firstAndLast(p.Route)
	if departure.Ident != "" && arrival.Ident != "" {
		return departure.Ident + " to " + arrival.Ident
	}
	if p.Name != "" {
		return p.Name
	}
	return "Flight plan"
}

func firstAndLast(route []Waypoint) (Waypoint, Waypoint) {
	if len(route) == 0 {
		return Waypoint{}, Waypoint{}
	}
	return route[0], route[len(route)-1]
}

// BriefingHTML renders a printable route brief for DCS. It is deliberately a
// standalone document: the pilot prints it or saves it as a PDF.
func BriefingHTML(p Plan, nav Navlog, lang string) ([]byte, error) {
	labels := briefingLabels(lang)
	view := struct {
		Labels map[string]string
		Plan   Plan
		Nav    Navlog
		Now    string
		Title  string
		Lang   string
	}{
		Labels: labels,
		Plan:   p,
		Nav:    nav,
		Now:    time.Now().Format("2006-01-02 15:04"),
		Title:  planTitle(p),
		Lang:   briefingLang(lang),
	}
	var buffer bytes.Buffer
	if err := briefingTemplate.Execute(&buffer, view); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func briefingLang(lang string) string {
	if strings.HasPrefix(strings.ToLower(lang), "fr") {
		return "fr"
	}
	return "en"
}

func briefingLabels(lang string) map[string]string {
	if briefingLang(lang) == "fr" {
		return map[string]string{
			"briefing":  "Briefing de vol",
			"aircraft":  "Appareil",
			"rules":     "Règles",
			"departure": "Départ",
			"arrival":   "Arrivée",
			"cruise":    "Altitude de croisière",
			"tas":       "TAS",
			"wind":      "Vent",
			"from":      "De",
			"to":        "Vers",
			"distance":  "Distance",
			"course":    "Cap vrai",
			"heading":   "Cap à suivre",
			"gs":        "Vitesse sol",
			"time":      "Temps",
			"fuel":      "Carburant",
			"total":     "Total",
			"reserve":   "Réserve",
			"required":  "Carburant requis",
			"notes":     "Notes",
			"route":     "Route",
			"trueCaps":  "Les caps sont vrais (pas de déclinaison magnétique).",
			"generated": "Généré le",
			"noRoute":   "Aucune route.",
		}
	}
	return map[string]string{
		"briefing":  "Flight briefing",
		"aircraft":  "Aircraft",
		"rules":     "Rules",
		"departure": "Departure",
		"arrival":   "Arrival",
		"cruise":    "Cruise altitude",
		"tas":       "TAS",
		"wind":      "Wind",
		"from":      "From",
		"to":        "To",
		"distance":  "Distance",
		"course":    "True course",
		"heading":   "Heading",
		"gs":        "Ground speed",
		"time":      "Time",
		"fuel":      "Fuel",
		"total":     "Total",
		"reserve":   "Reserve",
		"required":  "Required fuel",
		"notes":     "Notes",
		"route":     "Route",
		"trueCaps":  "Headings are true (no magnetic variation applied).",
		"generated": "Generated on",
		"noRoute":   "No route.",
	}
}

var briefingTemplate = template.Must(template.New("briefing").Funcs(template.FuncMap{
	"fmt0": func(v float64) string { return fmt.Sprintf("%.0f", v) },
	"fmt1": func(v float64) string { return fmt.Sprintf("%.1f", v) },
	"mins": func(v float64) string {
		minutes := int(math.Round(v))
		return fmt.Sprintf("%dh%02d", minutes/60, minutes%60)
	},
	"alt": func(v int) string { return fmt.Sprintf("%d ft", v) },
}).Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<title>{{.Title}} — {{.Labels.briefing}}</title>
<style>
  :root { color-scheme: light; }
  body { font-family: system-ui, Segoe UI, sans-serif; margin: 24px; color: #111; }
  h1 { margin: 0 0 4px; font-size: 22px; }
  h2 { margin: 22px 0 8px; font-size: 15px; text-transform: uppercase; letter-spacing: .06em; color: #555; }
  .meta { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px 18px; font-size: 13px; }
  .meta div span { display: block; color: #666; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; }
  table { width: 100%; border-collapse: collapse; font-size: 12px; margin-top: 6px; }
  th, td { border-bottom: 1px solid #ddd; padding: 5px 6px; text-align: left; }
  th { background: #f3f5f7; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; color: #555; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  .totals { margin-top: 10px; display: flex; gap: 22px; font-size: 13px; }
  .totals b { font-size: 16px; }
  .notes { white-space: pre-wrap; font-size: 12px; border: 1px solid #ddd; border-radius: 6px; padding: 8px; }
  .hint { color: #666; font-size: 11px; margin-top: 18px; }
  @media print { body { margin: 10mm; } .hint { display: none; } }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<div class="meta">
  <div><span>{{.Labels.aircraft}}</span>{{if .Plan.Aircraft}}{{.Plan.Aircraft}}{{else}}—{{end}}</div>
  <div><span>{{.Labels.rules}}</span>{{if .Plan.FlightRules}}{{.Plan.FlightRules}}{{else}}VFR{{end}}</div>
  <div><span>{{.Labels.cruise}}</span>{{alt .Plan.CruiseAltFt}}</div>
  <div><span>{{.Labels.tas}}</span>{{.Plan.CruiseTASKt}} kt</div>
  <div><span>{{.Labels.departure}}</span>{{.Plan.Departure}}</div>
  <div><span>{{.Labels.arrival}}</span>{{.Plan.Arrival}}</div>
  <div><span>{{.Labels.wind}}</span>{{.Plan.WindFromDeg}}° / {{.Plan.WindKt}} kt</div>
  <div><span>{{.Labels.generated}}</span>{{.Now}}</div>
</div>
<h2>{{.Labels.route}}</h2>
{{if .Nav.Legs}}
<table>
  <thead><tr><th>{{.Labels.from}}</th><th>{{.Labels.to}}</th><th>{{.Labels.distance}}</th><th>{{.Labels.course}}</th><th>{{.Labels.heading}}</th><th>{{.Labels.gs}}</th><th>{{.Labels.time}}</th><th>{{.Labels.fuel}}</th></tr></thead>
  <tbody>
  {{range .Nav.Legs}}
    <tr>
      <td>{{.From}}</td><td>{{.To}}</td>
      <td class="num">{{fmt1 .DistanceNM}} NM</td>
      <td class="num">{{fmt0 .CourseTrue}}°</td>
      <td class="num">{{fmt0 .HeadingTrue}}°</td>
      <td class="num">{{fmt0 .GroundSpeedKt}} kt</td>
      <td class="num">{{mins .TimeMin}}</td>
      <td class="num">{{fmt0 .Fuel}}</td>
    </tr>
  {{end}}
  </tbody>
</table>
<div class="totals">
  <div>{{.Labels.total}} <b>{{fmt1 .Nav.TotalDistanceNM}} NM</b> · <b>{{mins .Nav.TotalTimeMin}}</b></div>
  <div>{{.Labels.fuel}} <b>{{fmt0 .Nav.TotalFuel}}</b></div>
  <div>{{.Labels.reserve}} <b>{{fmt0 .Nav.ReserveFuel}}</b></div>
  <div>{{.Labels.required}} <b>{{fmt0 .Nav.RequiredFuel}}</b></div>
</div>
{{else}}
<p>{{.Labels.noRoute}}</p>
{{end}}
{{if .Plan.Notes}}<h2>{{.Labels.notes}}</h2><div class="notes">{{.Plan.Notes}}</div>{{end}}
<p class="hint">{{.Labels.trueCaps}}</p>
</body>
</html>`))
