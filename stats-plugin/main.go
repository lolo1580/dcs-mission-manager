// Command stats-web is the DCS Manager statistics plugin (PoC, phase P0).
//
// It is an EXTERNAL consumer of the manager's REST API: it periodically polls
// the /api/stats/* endpoints, stores timestamped snapshots in PostgreSQL and
// serves its own read-only dashboard. The manager itself is never modified and
// does not even know the plugin exists.
//
// See ../docs/stats-plugin.md for the full design.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed schema.sql
var schemaSQL string

//go:embed web
var webFS embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("stats-plugin: ")

	cfg := LoadConfig()
	log.Printf("manager=%s listen=%s interval=%s scopes=%v includeTest=%v",
		cfg.ManagerURL, cfg.ListenAddr, cfg.SyncInterval, cfg.Scopes, cfg.IncludeTest)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store, err := OpenStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx, schemaSQL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("postgres: ready")

	client := NewManagerClient(cfg)

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}
	srv := NewServer(cfg, store, client, web)

	syncer := NewSyncer(cfg, client, store)
	go syncer.Run(ctx)

	httpSrv := &http.Server{Addr: cfg.ListenAddr, Handler: srv.Handler()}
	go func() {
		log.Printf("http: dashboard on http://%s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = httpSrv.Shutdown(shutdownCtx)
}
