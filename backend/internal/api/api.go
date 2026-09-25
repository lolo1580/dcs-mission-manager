// Package api exposes the manager's HTTP interface: a small JSON API, a
// Server-Sent Events stream for live updates, and the embedded web UI.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"dcsmm/internal/state"
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
<html lang="fr">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>DCS Mission Manager</title>
    <style>
      :root { color-scheme: dark; }
      body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
             background: #0f1419; color: #e6edf3; display: grid; place-items: center; min-height: 100vh; }
      main { max-width: 640px; padding: 2rem; }
      h1 { font-size: 1.5rem; margin: 0 0 .5rem; }
      p { line-height: 1.6; color: #9da7b3; }
      code { background: #1f2630; padding: .1rem .35rem; border-radius: 4px; }
      a { color: #58a6ff; }
      .status { margin-top: 1.5rem; font-size: .95rem; }
      .dot { color: #3fb950; }
    </style>
  </head>
  <body>
    <main>
      <h1>DCS Mission Manager — backend actif</h1>
      <p>Le backend fonctionne, mais le frontend n'a pas encore été buildé. C'est la page de repli embarquée.</p>
      <p>Pour builder l'interface :</p>
      <pre><code>cd frontend
npm install
npm run build</code></pre>
      <p class="status"><span class="dot">●</span> API : <a href="/api/health">/api/health</a> ·
        <a href="/api/state">/api/state</a> · <a href="/api/events">/api/events</a></p>
    </main>
  </body>
</html>
`

// Server wires the unit store to the HTTP handlers.
type Server struct {
	store *state.Store
	hub   *hub
}

// New creates a server backed by store.
func New(store *state.Store) *Server {
	return &Server{store: store, hub: newHub()}
}

// Handler returns the HTTP router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/events", s.handleEvents)
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
		}
	}
}

func (s *Server) stateJSON() ([]byte, error) {
	units := s.store.Snapshot()
	payload := map[string]any{
		"type":  "state",
		"count": len(units),
		"units": units,
		"ts":    time.Now().UnixMilli(),
	}
	return json.Marshal(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"units":  s.store.Count(),
	})
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	units := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(units),
		"units": units,
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
