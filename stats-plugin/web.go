package main

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

// Server is the plugin's own HTTP API and dashboard. It reads the manager's
// PostgreSQL database directly; it never calls the manager over HTTP.
type Server struct {
	cfg   Config
	store *Store
	web   fs.FS
}

// NewServer wires the plugin's HTTP layer.
func NewServer(cfg Config, store *Store, web fs.FS) *Server {
	return &Server{cfg: cfg, store: store, web: web}
}

// Handler builds the plugin's routes, all wrapped by the optional auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plugin/health", s.handleHealth)
	mux.HandleFunc("/api/plugin/summary", s.handleSummary)
	mux.HandleFunc("/api/plugin/overview", s.handleOverview)
	mux.HandleFunc("/api/plugin/series", s.handleSeries)
	mux.HandleFunc("/api/plugin/missions", s.handleMissions)
	mux.HandleFunc("/api/plugin/export", s.handleExport)

	// Convenience routes, one per aggregate.
	for _, kind := range []string{"pilots", "weapons", "engines", "network"} {
		mux.HandleFunc("/api/plugin/"+kind, s.handleAggregate(kind))
	}

	mux.Handle("/", http.FileServer(http.FS(s.web)))
	return s.auth(mux)
}

// --- authentication ----------------------------------------------------------

// auth enforces the bearer token when one is configured (Authorization header,
// ?token= which sets a cookie, or the cookie). With no token it is a
// pass-through, which is the normal case on 127.0.0.1.
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

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	want := s.cfg.AuthToken
	if c, err := r.Cookie("plugin_token"); err == nil && subtleEqual(c.Value, want) {
		return true
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if subtleEqual(strings.TrimPrefix(h, "Bearer "), want) {
			return true
		}
	}
	if q := r.URL.Query().Get("token"); q != "" && subtleEqual(q, want) {
		http.SetCookie(w, &http.Cookie{
			Name: "plugin_token", Value: want, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600,
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

// handleHealth reports the plugin's status and the manager database's size.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"service":     "dcsmanager-stats-plugin",
		"mode":        "postgres-reader",
		"includeTest": s.cfg.IncludeTest,
	}
	if c, err := s.store.CountTotals(r.Context()); err == nil {
		out["counts"] = c
	} else {
		out["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// handleSummary builds every dashboard tab in one call, exactly like the
// manager's stats service but read straight from the tables.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	out := map[string]any{}

	overview, err := s.store.Overview(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out["overview"] = overview

	if pilots, err := s.store.Pilots(ctx); err == nil {
		out["pilots"] = map[string]any{"count": len(pilots), "pilots": pilots}
	} else {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if weapons, err := s.store.Weapons(ctx); err == nil {
		out["weapons"] = map[string]any{"count": len(weapons), "weapons": weapons}
	}
	if engines, err := s.store.Engines(ctx); err == nil {
		out["engines"] = map[string]any{"count": len(engines), "engines": engines}
	}
	if network, err := s.store.Network(ctx); err == nil {
		out["network"] = map[string]any{"count": len(network), "network": network}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleOverview returns just the dashboard summary.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	o, err := s.store.Overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleAggregate serves one of the per-kind aggregates.
func (s *Server) handleAggregate(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch kind {
		case "pilots":
			v, err := s.store.Pilots(ctx)
			respond(w, err, "pilots", v)
		case "weapons":
			v, err := s.store.Weapons(ctx)
			respond(w, err, "weapons", v)
		case "engines":
			v, err := s.store.Engines(ctx)
			respond(w, err, "engines", v)
		case "network":
			v, err := s.store.Network(ctx)
			respond(w, err, "network", v)
		}
	}
}

// handleSeries returns a daily event series.
//
//	GET /api/plugin/series?event=kill&days=30
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	eventKind := q.Get("event")
	days := 30
	if v := q.Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	points, err := s.store.EventSeries(r.Context(), eventKind, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if points == nil {
		points = []Point{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": eventKind, "days": days, "points": points})
}

// handleMissions returns the manager's missions, newest first.
func (s *Server) handleMissions(w http.ResponseWriter, r *http.Request) {
	missions, err := s.store.Missions(r.Context(), limitParam(r, 200))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if missions == nil {
		missions = []MissionRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(missions), "missions": missions})
}

// handleExport streams data as CSV or JSON.
//
//	GET /api/plugin/export?type=missions|series&format=csv
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("type")
	if kind == "" {
		kind = "missions"
	}
	csvOut := q.Get("format") == "csv"

	switch kind {
	case "missions":
		rows, err := s.store.Missions(r.Context(), limitParam(r, 10000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if csvOut {
			writeCSV(w, "missions.csv",
				[]string{"id", "name", "theatre", "source", "startedAt", "endedAt", "winner"},
				toRows(rows, func(m MissionRow) []string {
					return []string{
						strconv.FormatInt(m.ID, 10), m.Name, m.Theatre, m.Source,
						strconv.FormatInt(m.StartedAt, 10), strconv.FormatInt(m.EndedAt, 10), m.Winner,
					}
				}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "missions": rows})

	case "series":
		eventKind := q.Get("event")
		points, err := s.store.EventSeries(r.Context(), eventKind, 90)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"event": eventKind, "points": points})

	case "pilots":
		rows, err := s.store.Pilots(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if csvOut {
			writeCSV(w, "pilots.csv",
				[]string{"name", "ucid", "missions", "score", "kills", "deaths", "kd", "avgPing"},
				toRows(rows, func(p PilotStats) []string {
					return []string{
						p.Name, p.UCID, strconv.Itoa(p.Missions), strconv.Itoa(p.Score),
						strconv.Itoa(p.Kills), strconv.Itoa(p.Deaths),
						strconv.FormatFloat(p.KD, 'f', 2, 64), strconv.FormatFloat(p.AvgPing, 'f', 1, 64),
					}
				}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "pilots": rows})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown type: " + kind})
	}
}

// --- helpers -----------------------------------------------------------------

func respond(w http.ResponseWriter, err error, key string, v any) {
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{key: v})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
