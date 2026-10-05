package main

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"log"
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
// through timing. It delegates to crypto/subtle, matching the manager's own
// token guard.
func subtleEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
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
		s.fail(w, "query", err)
		return
	}
	out["overview"] = overview

	if pilots, err := s.store.Pilots(ctx); err == nil {
		out["pilots"] = map[string]any{"count": len(pilots), "pilots": pilots}
	} else {
		s.fail(w, "query", err)
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
		s.fail(w, "query", err)
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
		s.fail(w, "query", err)
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
		s.fail(w, "query", err)
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
			s.fail(w, "query", err)
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
			s.fail(w, "series", err)
			return
		}
		if csvOut {
			writeCSV(w, "series.csv",
				[]string{"date", "count"},
				toRows(points, func(p Point) []string {
					return []string{p.At.Format("2006-01-02"), strconv.FormatFloat(p.Value, 'f', 0, 64)}
				}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"event": eventKind, "points": points})

	case "pilots":
		rows, err := s.store.Pilots(r.Context())
		if err != nil {
			s.fail(w, "query", err)
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

// fail logs the real error server-side and returns a generic message to the
// caller: raw PostgreSQL errors name views and constraints, and there is no
// reason to hand that to a client.
func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	log.Printf("plugin: %s: %v", what, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func respond(w http.ResponseWriter, err error, key string, v any) {
	if err != nil {
		log.Printf("plugin: query failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{key: v})
}

func limitParam(r *http.Request, def int) int {
	const maxLimit = 10000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > maxLimit {
				return maxLimit
			}
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
	_ = cw.Write(sanitizeCSVRow(header))
	for _, row := range rows {
		_ = cw.Write(sanitizeCSVRow(row))
	}
}

// sanitizeCSVRow neutralises CSV formula injection: a cell beginning with
// = + - @ (or a tab/CR) is executed as a formula by Excel/Sheets. Names, UCIDs
// and theatre strings come from DCS/pilot input, so a player literally called
// "=cmd…" must not become a live formula when the export is opened.
func sanitizeCSVRow(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		if cell != "" {
			switch cell[0] {
			case '=', '+', '-', '@', '\t', '\r':
				cell = "'" + cell
			}
		}
		out[i] = cell
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
