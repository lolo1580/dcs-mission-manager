// Package api exposes the manager's HTTP interface: a small JSON API, a
// Server-Sent Events stream for live updates, and the embedded web UI.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dcsmanager/internal/aerodrome"
	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/charts"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/dcsdata"
	"dcsmanager/internal/debuglog"
	"dcsmanager/internal/live"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panelplugin"
	"dcsmanager/internal/panelservice"
	"dcsmanager/internal/state"
	"dcsmanager/internal/stats"
	"dcsmanager/internal/theatre"
	"dcsmanager/internal/updatecheck"
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
    <title>DCS Manager</title>
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
      <h1>DCS Manager — backend is up</h1>
      <p>The backend is running, but the frontend has not been built yet. This is the embedded fallback page.</p>
      <p>To build the UI:</p>
      <pre><code>cd frontend
npm install
npm run build</code></pre>
      <p class="status"><span class="dot">●</span> API : <a href="/api/health">/api/health</a> ·
        <a href="/api/events">/api/events</a> ·
        <a href="/api/theatres">/api/theatres</a></p>
    </main>
  </body>
</html>
`

// Commander pushes commands back to DCS over the hook connection, and reports
// whether a hook is connected. It is implemented by internal/tcp.Listener; the
// interface keeps package api free of a dependency on the transport.
type Commander interface {
	// SendCommand serializes v as one JSON line to every connected hook and
	// returns how many received it. Zero means no hook is connected.
	SendCommand(v any) int
	// Connected reports how many hook connections are open.
	Connected() int
}

// Server wires the session stores to the HTTP handlers.
type Server struct {
	cfg             config.Config
	store           *state.Store
	live            *live.Store
	db              db.Store
	stats           *stats.Service
	aerodromes      *aerodrome.Catalog
	charts          *charts.Catalog
	modules         dcsdata.ModuleInventory
	logbook         dcsdata.Logbook
	mods            []dcsdata.InstalledMod
	scripts         dcsdata.ScriptStatus
	scriptInstallMu sync.Mutex
	dcsRunning      func() (bool, error)
	panels          *panelservice.Service
	panelPlugin     *panelplugin.Client
	bios            *dcsbios.Client
	mappings        *mapping.Store
	controls        map[string]*biosmeta.Catalog
	controlsMu      sync.Mutex
	commander       Commander
	hub             *hub
	theatres        []theatre.Theatre
	chartsDir       string
	// localOnly is true when the server is bound to the loopback interface, in
	// which case it also refuses requests whose Host is not local (DNS
	// rebinding).
	localOnly bool
	// testMode, when on, lets panel inputs reach DCS-BIOS regardless of the sending
	// switch, so the mapping can be checked live in the Panels test view. Off by
	// default and off again on restart.
	testMode atomic.Bool
	// debug is the runtime debug switch and in-memory log. It may be nil (some
	// tests build a Server without it), so every use goes through helpers below.
	debug   *debuglog.Logger
	version string
	updates *updatecheck.Checker
}

// SetDebug installs the debug logger. It is optional: the endpoints then just
// report debug as unavailable.
func (s *Server) SetDebug(l *debuglog.Logger) {
	s.debug = l
}

// DebugEnabled reports whether debug logging is on.
func (s *Server) DebugEnabled() bool {
	return s.debug != nil && s.debug.Enabled()
}

// debugf records a debug line if a logger is installed. The call sites do not have
// to check for nil.
func (s *Server) debugf(area, format string, args ...any) {
	if s.debug != nil {
		s.debug.Infof(area, format, args...)
	}
}

// New creates a server backed by store. live, database, statsService and
// commander may be nil.
func New(cfg config.Config, store *state.Store, liveStore *live.Store, database db.Store, statsService *stats.Service, aerodromes *aerodrome.Catalog, chartCatalog *charts.Catalog, commander Commander) *Server {
	return &Server{
		cfg:        cfg,
		store:      store,
		live:       liveStore,
		db:         database,
		stats:      statsService,
		aerodromes: aerodromes,
		charts:     chartCatalog,
		commander:  commander,
		hub:        newHub(),
		theatres:   theatre.All(),
		chartsDir:  cfg.ChartsDir,
		localOnly:  isLoopbackAddr(cfg.HTTPAddr),
		updates:    updatecheck.New(),
	}
}

// SetVersion sets the version embedded by the release build.
func (s *Server) SetVersion(version string) { s.version = version }

// SetLocalOnly overrides the loopback decision after the real listen address is
// known. The server is built before `net.Listen`, so `New` can only guess from
// the configured string; this corrects it to match the socket actually bound.
func (s *Server) SetLocalOnly(local bool) {
	s.localOnly = local
}

// SetTestMode turns the live mapping test on or off. In test mode a panel input is
// sent to DCS-BIOS even when command sending is not armed, so the mapping can be
// verified against the cockpit directly from the Panels view.
func (s *Server) SetTestMode(on bool) {
	s.testMode.Store(on)
}

// TestMode reports whether the live mapping test is on.
func (s *Server) TestMode() bool {
	return s.testMode.Load()
}

// SetModules installs the DCS module inventory read from the installation. It is
// a setter rather than a constructor argument: the inventory is optional data,
// and the constructor already carries the optional collaborators.
func (s *Server) SetModules(inv dcsdata.ModuleInventory) {
	s.modules = inv
}

// SetLogbook installs the player's career logbook read from the installation.
func (s *Server) SetLogbook(lb dcsdata.Logbook) {
	s.logbook = lb
}

// SetMods installs the list of mods found under Saved Games\DCS\Mods.
func (s *Server) SetMods(mods []dcsdata.InstalledMod) {
	s.mods = mods
}

// SetScripts installs the DCS-side script status picture.
func (s *Server) SetScripts(st dcsdata.ScriptStatus) {
	s.scripts = st
}

// Handler returns the HTTP router.
//
// Two guards wrap the router. The origin guard is outermost, so a cross-origin
// or non-local-Host browser request is refused before anything else. The token
// guard sits just below and is the opt-in protection for the documented case
// where the whole manager is exposed beyond loopback.
func (s *Server) Handler() http.Handler {
	mux := s.routes()
	return s.originGuard(s.tokenGuard(s.debugMiddleware(mux)))
}

// debugMiddleware records every API request (method, path, status) when debug is
// on. It is the "what is the UI actually asking for?" view, and it costs a no-op
// string check when debug is off.
func (s *Server) debugMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.debug == nil || !s.debug.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		s.debug.Infof("http", "%s %s -> %d (%s)", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// statusRecorder captures the status code for the debug log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// originGuard protects an API that has a destructive endpoint and no
// authentication from being driven by a web page.
//
// Two browser-borne attacks matter for a local server:
//
//   - CSRF: a page on another site sends a request to http://127.0.0.1:8080. A
//     plain POST is a "simple request" and reaches the server without a
//     preflight, so the Origin must be checked explicitly.
//   - DNS rebinding: a page served from a domain that is later re-pointed at
//     127.0.0.1. Origin and Host then agree, so the Origin check alone is not
//     enough; the Host must also name the local machine.
//
// When the operator deliberately binds to a non-loopback address (the
// documented opt-in for reaching the UI from another device), the Host check is
// skipped: they asked for network access, and the README states the API is then
// unauthenticated. Requests without an Origin (curl, the CLI) are always
// allowed, since no browser is involved.
func (s *Server) originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		if s.localOnly && !loopbackHost(r.Host) {
			http.Error(w, "request refused: the manager only accepts local requests", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin reports whether an Origin header matches the request's Host.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == host
}

// loopbackHost reports whether a Host header names the local machine.
//
// An empty host is not loopback: in a listen address (":8080") it means "every
// interface", and treating it as local would let the server believe it is
// unreachable while it is exposed.
func loopbackHost(host string) bool {
	if host == "" {
		return false
	}
	h := host
	if hostOnly, _, err := net.SplitHostPort(host); err == nil {
		h = hostOnly
	}
	if h == "" {
		// ":8080" — a port with no host, i.e. all interfaces.
		return false
	}
	switch strings.Trim(h, "[]") {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// isLoopbackAddr reports whether a listen address only accepts local traffic.
func isLoopbackAddr(addr string) bool {
	return loopbackHost(addr)
}

// IsLoopbackAddr exposes the loopback decision so the app can re-derive
// "local only" from the address actually bound, rather than from the configured
// string the server was built with.
func IsLoopbackAddr(addr string) bool {
	return isLoopbackAddr(addr)
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/update", s.handleUpdate)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/theatres", s.handleTheatres)
	mux.HandleFunc("/api/units/", s.handleUnit)
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
	mux.HandleFunc("/api/stats/missions", s.handleStatsMissions)
	mux.HandleFunc("/api/stats/trend", s.handleStatsTrend)
	mux.HandleFunc("/api/stats/pilots", s.handleStatsPilots)
	mux.HandleFunc("/api/career/insights", s.handleCareerInsights)
	mux.HandleFunc("/api/stats/weapons", s.handleStatsWeapons)
	mux.HandleFunc("/api/stats/engines", s.handleStatsEngines)
	mux.HandleFunc("/api/stats/network", s.handleStatsNetwork)
	mux.HandleFunc("/api/aerodromes", s.handleAerodromes)
	mux.HandleFunc("/api/aerodromes/", s.handleAerodrome)
	mux.HandleFunc("/api/charts", s.handleCharts)
	mux.HandleFunc("/api/charts/file/", s.handleChartFile)
	mux.HandleFunc("/api/maintenance", s.handleMaintenance)
	mux.HandleFunc("/api/maintenance/purge", s.handlePurge)
	mux.HandleFunc("/api/modules", s.handleModules)
	mux.HandleFunc("/api/career", s.handleCareer)
	mux.HandleFunc("/api/backup", s.handleBackup)
	mux.HandleFunc("/api/backup/categories", s.handleBackupCategories)
	mux.HandleFunc("/api/backup/restore", s.handleBackupRestore)
	mux.HandleFunc("/api/backup/download/", s.handleBackupDownload)
	mux.HandleFunc("/api/mods", s.handleMods)
	mux.HandleFunc("/api/scripts", s.handleScripts)
	mux.HandleFunc("/api/scripts/install", s.handleScriptInstall)
	mux.HandleFunc("/api/panels", s.handlePanels)
	mux.HandleFunc("/api/panels/plugin", s.handlePanelPlugin)
	mux.HandleFunc("/api/dcsbios", s.handleDCSBIOS)
	mux.HandleFunc("/api/mappings", s.handleMappings)
	mux.HandleFunc("/api/mappings/safety", s.handleMappingSafety)
	mux.HandleFunc("/api/mappings/outputs", s.handleOutputsSafety)
	mux.HandleFunc("/api/mappings/test", s.handleMappingTest)
	mux.HandleFunc("/api/aircraft", s.handleAircraft)
	mux.HandleFunc("/api/controls", s.handleControls)
	mux.HandleFunc("/api/display/preview", s.handleDisplayPreview)
	mux.HandleFunc("/api/display/test", s.handleDisplayTest)
	mux.HandleFunc("/api/debug", s.handleDebug)
	mux.HandleFunc("/api/log", s.handleLog)
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
		// A silent export is not proof that DCS is paused: the mission may have
		// stopped or the Lua/UDP connection may be unavailable.
		"feedStopped": s.store.FeedStopped(),
	}
	if age, ok := s.store.FeedAge(); ok {
		payload["feedAgeMs"] = age.Milliseconds()
		payload["feedSeen"] = true
	} else {
		payload["feedSeen"] = false
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

// Summary aggregates unit counts for the session description.
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
	units := s.store.Snapshot()
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	payload := map[string]any{
		"type":    "state",
		"count":   len(units),
		"units":   units,
		"summary": summarise(units),
		"ts":      time.Now().UnixMilli(),
	}
	return json.Marshal(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	units := 0
	if s.store != nil {
		units = s.store.Count()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "dcsmanager",
		"units":   units,
	})
}

// handleState returns the current units, optionally filtered.
//
// Query parameters: category, coalition, ownship=true, q (type/label substring).
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	units := filter(s.store.Snapshot(), r)
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(units),
		"units":   units,
		"summary": summarise(units),
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
	for _, u := range s.store.Snapshot() {
		if u.ID == id {
			writeJSON(w, http.StatusOK, u)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "unit not found"})
}

func (s *Server) handleTheatres(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"default":  s.cfg.Theatre,
		"theatres": s.theatres,
	})
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

	// Send the current session immediately, so a client that connects between
	// two broadcasts is not left with an empty view for a whole interval.
	if b, err := s.sessionJSON(); err == nil && b != nil {
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
