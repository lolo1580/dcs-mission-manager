// Command dcsmm is the DCS Mission Manager backend.
//
// It listens for telemetry sent by the DCS Lua scripts (UDP), keeps an in-memory
// state of the units, and exposes it to the web UI (HTTP + Server-Sent Events).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dcsmm/internal/api"
	"dcsmm/internal/category"
	"dcsmm/internal/config"
	"dcsmm/internal/state"
	"dcsmm/internal/udp"
)

func main() {
	cfg := config.Load()

	store := state.New(cfg.UnitTTL, cfg.MaxUnits)
	classifier := category.New(cfg.CategoriesFile)

	conn, err := udp.Listen(cfg.UDPAddr)
	if err != nil {
		log.Fatalf("udp: listen on %s: %v", cfg.UDPAddr, err)
	}
	defer conn.Close()

	listener := udp.NewListener(store, classifier)
	go listener.Serve(conn)
	log.Printf("udp: listening on %s", cfg.UDPAddr)

	srv := api.New(cfg, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.RunBroadcast(ctx, time.Second)

	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv.Handler(),
	}
	go func() {
		log.Printf("http: web ui on http://%s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
