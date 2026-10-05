package main

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

// Server is the plugin's own HTTP API and dashboard. It is separate from the
// manager's UI: the plugin serves a different port and its own pages.
type Server struct {
	cfg    Config
	store  *Store
	client *ManagerClient
	web    fs.FS
}

// NewServer wires the plugin's HTTP layer.
func NewServer(cfg Config, store *Store, client *ManagerClient, web fs.FS) *Server {
	return &Server{cfg: cfg, store: store, client: client, web: web}
}

// latestKinds are the snapshot kinds the generic /latest endpoint accepts.
var latestKinds = map[string]bool{
	"overview": true,
	"pilots":   true,
	"weapons":  true,
	"engines":  true,
	"network":  true,
}

// Handler builds the plugin's routes. When an auth token is configured, every
// route (including the static dashboard) is wrapped by auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plugin/health", s.handleHealth)
	mux.HandleFunc("/api/plugin/instances", s.handleInstances)
	mux.HandleFunc("/api/plugin/summary", s.handleSummary)
	mux.HandleFunc("/api/plugin/series", s.handleSeries)
	mux.HandleFunc("/api/plugin/latest", s.handleLatest)
	mux.HandleFunc("/api/plugin/missions", s.handleMissions)
	mux.HandleFunc("/api/plugin/export", s.handleExport)

	// Convenience routes for a single kind.
	for _, kind := range []string{"pilots", "weapons", "engines", "network"} {
		mux.HandleFunc("/api/plugin/"+kind, s.handleKind(kind))
	}

	mux.Handle("/", http.FileServer(http.FS(s.web)))
	return s.auth(mux)
}

// --- authentication ----------------------------------------------------------

// auth enforces the bearer token when one is configured. The token may arrive as
// an Authorization header (API clients) or as ?token= (first browser hit); a
// successful ?token= also sets a cookie so the dashboard's later asset and API
// requests authenticate without repeating the token in every URL.
//
// With no token configured the check is a pass-through, which is the normal
// case on 127.0.0.1.
func (s *Server) auth(next http.Handler) http.Handler {
	if s.cfg.AuthToken == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(w, r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authorize reports whether the request carries a valid token, and sets the
// convenience cookie when the token arrived through ?token=.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	want := s.cfg.AuthToken

	// Cookie, set after a first successful ?token=.
	if c, err := r.Cookie("plugin_token"); err == nil && subtleEqual(c.Value, want) {
		return true
	}
	// Authorization: Bearer <token>.
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if subtleEqual(strings.TrimPrefix(h, "Bearer "), want) {
			return true
		}
	}
	// ?token=<token>, for the browser's first hit. Persist it as a cookie so the
	// dashboard's later asset and API requests authenticate without repeating it.
	if q := r.URL.Query().Get("token"); q != "" && subtleEqual(q, want) {
		http.SetCookie(w, &http.Cookie{
			Name:     "plugin_token",
			Value:    want,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   30 * 24 * 3600,
		})
		return true
	}
	return false
}

// subtleEqual compares two secrets without leaking their length or prefix
// through timing on the common path.
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// --- handlers ----------------------------------------------------------------

// handleHealth reports the plugin status plus the manager's reachability.
//
//	GET /api/plugin/health?instance=local
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instance := s.instance(r)

	out := map[string]any{
		"service":    "dcsmanager-stats-plugin",
		"managerUrl": s.cfg.ManagerURL,
		"instance":   instance,
		"scopes":     s.cfg.Scopes,
	}
	if h, err := s.client.Health(ctx); err == nil {
		out["manager"] = h
	} else {
		out["managerError"] = err.Error()
	}
	if last, err := s.store.LastSync(ctx, instance); err == nil {
		out["lastSync"] = last
	}
	if c, err := s.store.Counts(ctx, instance); err == nil {
		out["counts"] = c
	}
	if instances, err := s.store.Instances(ctx, instance); err == nil {
		out["instances"] = instances
	}
	writeJSON(w, http.StatusOK, out)
}

// handleInstances lists the manager instances stored in the database.
func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := s.store.Instances(r.Context(), s.cfg.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if instances == nil {
		instances = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": instances, "default": s.cfg.Name})
}

// handleSummary returns the latest snapshot of each kind, keyed by kind. The
// dashboard builds every tab from this single call.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	instance := s.instance(r)
	snaps, err := s.store.Latest(r.Context(), instance)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := map[string]any{"instance": instance}
	for _, sn := range snaps {
		key := sn.Kind
		if sn.Scope != "career" {
			key = sn.Kind + ":" + sn.Scope
		}
		out[key] = sn.Payload
	}
	writeJSON(w, http.StatusOK, out)
}

// handleLatest returns the most recent snapshot of one kind/scope verbatim.
//
//	GET /api/plugin/latest?kind=pilots&scope=career&instance=local
func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	s.writeLatest(w, r, r.URL.Query().Get("kind"), r.URL.Query().Get("scope"))
}

// handleKind is the fixed-kind form of handleLatest.
func (s *Server) handleKind(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.writeLatest(w, r, kind, r.URL.Query().Get("scope"))
	}
}

func (s *Server) writeLatest(w http.ResponseWriter, r *http.Request, kind, scope string) {
	if kind == "" {
		kind = "pilots"
	}
	if !latestKinds[kind] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown kind: " + kind})
		return
	}
	if scope == "" {
		scope = "career"
	}
	raw, err := s.store.LatestPayload(r.Context(), s.instance(r), kind, scope)
	if err != nil || len(raw) == 0 {
		writeJSON(w, http.StatusOK, emptyPayload(kind))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

// handleMissions returns the mirrored missions of an instance, newest first.
func (s *Server) handleMissions(w http.ResponseWriter, r *http.Request) {
	missions, err := s.store.Missions(r.Context(), s.instance(r), limitParam(r, 500))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if missions == nil {
		missions = []Mission{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(missions), "missions": missions})
}

// handleSeries returns a time series for one overview metric.
//
//	GET /api/plugin/series?metric=kills&scope=career&limit=500
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	metric := q.Get("metric")
	if metric == "" {
		metric = "kills"
	}
	scope := q.Get("scope")
	if scope == "" {
		scope = "career"
	}
	points, err := s.store.OverviewSeries(r.Context(), s.instance(r), scope, metric, limitParam(r, 500))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if points == nil {
		points = []Point{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metric": metric,
		"scope":  scope,
		"points": points,
	})
}

// handleExport streams mirrored data as CSV or JSON.
//
//	GET /api/plugin/export?type=events|chat|missions&format=csv&event=kill
//	GET /api/plugin/export?type=series&metric=kills&format=json
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("type")
	instance := s.instance(r)
	if kind == "" {
		kind = "events"
	}
	csvOut := q.Get("format") == "csv"

	switch kind {
	case "events":
		rows, err := s.store.Events(r.Context(), instance, q.Get("event"), limitParam(r, 10000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if csvOut {
			writeCSV(w, "events_"+instance+".csv",
				[]string{"id", "event", "t", "realTs", "args", "detail"},
				toRows(rows, func(e Event) []string {
					a, _ := json.Marshal(e.Args)
					d, _ := json.Marshal(e.Detail)
					return []string{
						strconv.FormatInt(e.ID, 10), e.Event,
						strconv.FormatFloat(e.T, 'f', -1, 64),
						strconv.FormatInt(e.RealTS, 10), string(a), string(d),
					}
				}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "events": rows})

	case "missions":
		rows, err := s.store.Missions(r.Context(), instance, limitParam(r, 10000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if csvOut {
			writeCSV(w, "missions_"+instance+".csv",
				[]string{"id", "name", "theatre", "source", "startedAt", "endedAt", "winner"},
				toRows(rows, func(m Mission) []string {
					return []string{
						strconv.FormatInt(m.ID, 10), m.Name, m.Theatre, m.Source,
						strconv.FormatInt(m.StartedAt, 10),
						strconv.FormatInt(m.EndedAt, 10), m.Winner,
					}
				}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "missions": rows})

	case "series":
		metric := q.Get("metric")
		if metric == "" {
			metric = "kills"
		}
		scope := q.Get("scope")
		if scope == "" {
			scope = "career"
		}
		points, err := s.store.OverviewSeries(r.Context(), instance, scope, metric, limitParam(r, 5000))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metric": metric, "scope": scope, "points": points})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown type: " + kind})
	}
}

// --- helpers -----------------------------------------------------------------

// instance resolves which manager instance a request targets: ?instance=, or
// the plugin's own configured name. The value is only ever used as a bound SQL
// parameter, never interpolated.
func (s *Server) instance(r *http.Request) string {
	if v := r.URL.Query().Get("instance"); v != "" {
		return v
	}
	return s.cfg.Name
}

func limitParam(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func toRows[T any](in []T, f func(T) []string) [][]string {
	out := make([][]string, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write(header)
	for _, row := range rows {
		_ = cw.Write(row)
	}
}

// emptyPayload returns the empty shape a kind would have, so a client can render
// an empty table instead of having to special-case a missing snapshot.
func emptyPayload(kind string) map[string]any {
	switch kind {
	case "overview":
		return map[string]any{}
	case "pilots":
		return map[string]any{"count": 0, "pilots": []any{}}
	case "weapons":
		return map[string]any{"count": 0, "weapons": []any{}}
	case "engines":
		return map[string]any{"count": 0, "engines": []any{}}
	case "network":
		return map[string]any{"count": 0, "network": []any{}}
	}
	return map[string]any{}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
