// Command stats-web is the DCS Manager statistics plugin.
//
// PostgreSQL is the shared library: the manager writes to it (it runs with
// DCSMANAGER_DB_DRIVER=postgres), and this plugin reads the manager's own tables
// directly to serve a statistics dashboard. It never calls the manager over
// HTTP, and its database session is read-only (see OpenStore).
//
// The manager can therefore stay bound to loopback: the only thing the plugin
// needs is the database DSN. See ../docs/stats-plugin.md for the design.
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

//go:embed web
var webFS embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("stats-plugin: ")

	cfg := LoadConfig()
	log.Printf("listen=%s includeTest=%v dsn=%t", cfg.ListenAddr, cfg.IncludeTest, cfg.ManagerDSN != "")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store, err := OpenStore(ctx, cfg.ManagerDSN, cfg.IncludeTest)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer store.Close()
	log.Printf("postgres: connected (read-only)")

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}
	srv := NewServer(cfg, store, web)

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
