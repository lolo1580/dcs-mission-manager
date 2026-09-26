// Package api exposes the manager's HTTP interface: a small JSON API, a
// Server-Sent Events stream for live updates, map tiles, and the embedded web UI.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dcsmm/internal/aerodrome"
	"dcsmm/internal/basemap"
	"dcsmm/internal/config"
	"dcsmm/internal/db"
	"dcsmm/internal/live"
	"dcsmm/internal/state"
	"dcsmm/internal/stats"
	"dcsmm/internal/theatre"
	"dcsmm/internal/visibility"
)

// The frontend build is written here by `npm run build` (see
// frontend/vite.config.js) and embedded into the binary. The `all:` prefix
// includes dotfiles, which is what makes the embed pattern match on a fresh
// checkout that only contains the placeholder file.
//
//go:embed all:dist
var webFS embed.FS

// fallbackPage is served when the frontend has not been built yet, so that the
// backend is still usable and self-explanatory out of the box.
const fallbackPage = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>DCS Mission Manager</title>
    <style>
      :root { color-scheme: dark; }
      body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
             background: #0f1419; color: #e6edf3; display: grid; place-items: center; min-height: 100vh; }
      main { max-width: 640px; padding: 2rem; }
      h1 { font-size: 1.5rem; margin: 0 0 1rem; }
      p { line-height: 1.6; color: #9da7b3; }
      code { background: #1f2630; padding: .1rem .35rem; border-radius: 4px; }
      a { color: #58a6ff; }
      .status { margin-top: 1.5rem; font-size: .95rem; }
      .dot { color: #3fb950; }
    </style>
  </head>
  <body>
    <main>
      <h1>DCS Mission Manager — backend is up</h1>
      <p>The backend is running, but the frontend has not been built yet. This is the embedded fallback page.</p>
      <p>To build the UI:</p>
      <pre><code>cd frontend
npm install
npm run build</code></pre>
      <p class="status"><span class="dot">●</span> API : <a href="/api/health">/api/health</a> ·
        <a href="/api/state">/api/state</a> · <a href="/api/events">/api/events</a> ·
        <a href="/api/theatres">/api/theatres</a></p>
    </main>
  </body>
</html>
`

// Server wires the unit store to the HTTP handlers.
type Server struct {
	cfg        config.Config
	store      *state.Store
	live       *live.Store
	db         *db.DB
	stats      *stats.Service
	aerodromes *aerodrome.Catalog
	visibility *visibility.Policy
	hub        *hub
	theatres   []theatre.Theatre
	tilesDir   string
	basemaps   []basemap.Basemap
}

// New creates a server backed by store. live, database and statsService may be nil.
func New(cfg config.Config, store *state.Store, liveStore *live.Store, database *db.DB, statsService *stats.Service, aerodromes *aerodrome.Catalog, vis *visibility.Policy) *Server {
	theatres := theatre.All()
	for i := range theatres {
		theatres[i].Tiles = hasTiles(cfg.TilesDir, theatres[i].ID)
	}
	if vis == nil {
		vis = visibility.New(false)
	}
	return &Server{
		cfg:        cfg,
		store:      store,
		live:       liveStore,
		db:         database,
		stats:      statsService,
		aerodromes: aerodromes,
		visibility: vis,
		hub:        newHub(),
		theatres:   theatres,
		tilesDir:   cfg.TilesDir,
		basemaps:   basemap.All(cfg.BasemapURL),
	}
}

// Handler returns the HTTP router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/theatres", s.handleTheatres)
	mux.HandleFunc("/api/visibility", s.handleVisibility)
	mux.HandleFunc("/api/units/", s.handleUnit)
	mux.HandleFunc("/api/tiles/", s.handleTiles)
	mux.HandleFunc("/api/game-events", s.handleGameEvents)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/players", s.handlePlayers)
	mux.HandleFunc("/api/mission", s.handleMission)
	mux.HandleFunc("/api/history/events", s.handleHistoryEvents)
	mux.HandleFunc("/api/history/chat", s.handleHistoryChat)
	mux.HandleFunc("/api/history/missions", s.handleHistoryMissions)
	mux.HandleFunc("/api/debriefs", s.handleDebriefs)
	mux.HandleFunc("/api/debriefs/", s.handleDebrief)
	mux.HandleFunc("/api/stats/overview", s.handleStatsOverview)
	mux.HandleFunc("/api/stats/pilots", s.handleStatsPilots)
	mux.HandleFunc("/api/stats/weapons", s.handleStatsWeapons)
	mux.HandleFunc("/api/stats/engines", s.handleStatsEngines)
	mux.HandleFunc("/api/stats/network", s.handleStatsNetwork)
	mux.HandleFunc("/api/analytics/heatmap", s.handleHeatmap)
	mux.HandleFunc("/api/analytics/tracks", s.handleTracks)
	mux.HandleFunc("/api/analytics/sorties", s.handleSorties)
	mux.HandleFunc("/api/aerodromes", s.handleAerodromes)
	mux.HandleFunc("/api/aerodromes/", s.handleAerodrome)
	mux.HandleFunc("/api/maintenance", s.handleMaintenance)
	mux.HandleFunc("/api/maintenance/purge", s.handlePurge)
	mux.Handle("/", s.webHandler())
	return mux
}

// RunBroadcast pushes the current state to every SSE client at the given
// interval until ctx is cancelled.
func (s *Server) RunBroadcast(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if b, err := s.stateJSON(); err == nil {
				s.hub.broadcast(b)
			}
			if b, err := s.sessionJSON(); err == nil {
				s.hub.broadcast(b)
			}
		}
	}
}

// sessionJSON is the compact "everything that changed" frame: recent game
// events, players, chat and mission. It is small enough to resend each second.
func (s *Server) sessionJSON() ([]byte, error) {
	if s.live == nil {
		return nil, nil
	}
	payload := map[string]any{
		"type":    "session",
		"events":  s.live.Events(),
		"players": s.live.Players(),
		"chat":    s.live.Chat(),
	}
	if m, ok := s.live.Mission(); ok {
		payload["mission"] = m
	}
	return json.Marshal(payload)
}

// BroadcastMessage lets other components push an arbitrary SSE frame (for
// example a newly persisted event) to connected clients.
func (s *Server) BroadcastMessage(v any) {
	if b, err := json.Marshal(v); err == nil {
		s.hub.broadcast(b)
	}
}

// Summary aggregates unit counts for the UI legend.
type Summary struct {
	ByCategory  map[string]int `json:"byCategory"`
	ByCoalition map[string]int `json:"byCoalition"`
}

func summarise(units []state.Unit) Summary {
	s := Summary{
		ByCategory:  make(map[string]int),
		ByCoalition: make(map[string]int),
	}
	for _, u := range units {
		cat := u.Category
		if cat == "" {
			cat = "other"
		}
		s.ByCategory[cat]++
		co := u.Coalition
		if co == "" {
			co = "neutral"
		}
		s.ByCoalition[co]++
	}
	return s
}

func (s *Server) stateJSON() ([]byte, error) {
	// Fog-of-war filtering happens here, once, so every consumer (SSE, REST)
	// sees exactly the same, mission-authorised view.
	units := s.visibility.Filter(s.store.Snapshot())
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	payload := map[string]any{
		"type":       "state",
		"count":      len(units),
		"units":      units,
		"summary":    summarise(units),
		"visibility": s.visibility.Describe(),
		"ts":         time.Now().UnixMilli(),
	}
	return json.Marshal(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"units":  s.store.Count(),
	})
}

// handleState returns the current units, optionally filtered.
//
// Query parameters: category, coalition, ownship=true, q (type/label substring).
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	units := filter(s.visibility.Filter(s.store.Snapshot()), r)
	writeJSON(w, http.StatusOK, map[string]any{
		"count":      len(units),
		"units":      units,
		"summary":    summarise(units),
		"visibility": s.visibility.Describe(),
	})
}

func filter(units []state.Unit, r *http.Request) []state.Unit {
	q := r.URL.Query()
	category := q.Get("category")
	coalition := q.Get("coalition")
	ownship := q.Get("ownship") == "true"
	search := strings.ToLower(q.Get("q"))

	if category == "" && coalition == "" && !ownship && search == "" {
		return units
	}
	out := make([]state.Unit, 0, len(units))
	for _, u := range units {
		if category != "" && u.Category != category {
			continue
		}
		if coalition != "" && u.Coalition != coalition {
			continue
		}
		if ownship && !u.Ownship {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(u.Type+" "+u.Label), search) {
			continue
		}
		out = append(out, u)
	}
	return out
}

// handleUnit returns a single unit by id.
func (s *Server) handleUnit(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/units/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing unit id"})
		return
	}
	for _, u := range s.visibility.Filter(s.store.Snapshot()) {
		if u.ID == id {
			writeJSON(w, http.StatusOK, u)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "unit not found"})
}

// handleVisibility reports the active fog-of-war policy.
func (s *Server) handleVisibility(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.visibility.Describe())
}

func (s *Server) handleTheatres(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"default":  s.cfg.Theatre,
		"theatres": s.theatres,
		"basemaps": s.basemaps,
		"basemap":  s.cfg.Basemap,
	})
}

// handleTiles serves DCS map tiles laid out as <tilesDir>/<theatre>/<z>/<x>/<y>.png.
func (s *Server) handleTiles(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/tiles/")
	parts := strings.Split(rest, "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	th, z, x, y := parts[0], parts[1], parts[2], parts[3]
	if !validTilePart(th) || !validTilePart(z) || !validTilePart(x) || !validTilePart(y) {
		http.NotFound(w, r)
		return
	}
	// Guard against path traversal by rejecting separators and dots already
	// covered by validTilePart; join and ensure the result stays under tilesDir.
	full := filepath.Join(s.tilesDir, th, z, x, y+".png")
	absBase, err := filepath.Abs(s.tilesDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absFull, err := filepath.Abs(full)
	if err != nil || !strings.HasPrefix(absFull, absBase) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(absFull); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, absFull)
}

func validTilePart(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func hasTiles(dir, theatreID string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, theatreID))
	return err == nil && info.IsDir()
}

// handleEvents implements a Server-Sent Events stream.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan []byte, 8)
	s.hub.add(ch)
	defer s.hub.remove(ch)

	if b, err := s.stateJSON(); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// webHandler serves the embedded frontend. If it has not been built, the
// fallback page is served instead. Unknown paths fall back to index.html so
// that client-side routing keeps working.
func (s *Server) webHandler() http.Handler {
	sub, err := fs.Sub(webFS, "dist")
	if err != nil {
		panic(err)
	}

	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(fallbackPage))
		})
	}

	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(sub, p); err != nil {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// hub fans out messages to connected SSE clients.
type hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newHub() *hub {
	return &hub{clients: make(map[chan []byte]struct{})}
}

func (h *hub) add(c chan []byte) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *hub) remove(c chan []byte) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *hub) broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- b:
		default:
			// Slow client: drop the frame rather than block the broadcaster.
		}
	}
}
