// Command dcsmm is the DCS Mission Manager backend.
//
// It listens for telemetry from the DCS Lua scripts (UDP for positions, TCP for
// events and players), keeps an in-memory state, persists history to SQLite, and
// exposes everything to the web UI (HTTP + Server-Sent Events).
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
	"dcsmm/internal/db"
	"dcsmm/internal/debriefstore"
	"dcsmm/internal/ingest"
	"dcsmm/internal/live"
	"dcsmm/internal/model"
	"dcsmm/internal/state"
	"dcsmm/internal/stats"
	"dcsmm/internal/tcp"
	"dcsmm/internal/tracker"
	"dcsmm/internal/udp"
)

func main() {
	cfg := config.Load()

	store := state.New(cfg.UnitTTL, cfg.MaxUnits)
	classifier := category.New(cfg.CategoriesFile)
	liveStore := live.New(1000, 500)

	// ---- Optional persistence --------------------------------------------
	var database *db.DB
	if cfg.DBEnabled {
		var err error
		database, err = db.Open(cfg.DBPath)
		if err != nil {
			log.Printf("db: disabled, could not open %s: %v", cfg.DBPath, err)
			database = nil
		} else {
			defer database.Close()
			log.Printf("db: using %s", cfg.DBPath)
		}
	}

	// ---- UDP: unit positions ---------------------------------------------
	udpConn, err := udp.Listen(cfg.UDPAddr)
	if err != nil {
		log.Fatalf("udp: listen on %s: %v", cfg.UDPAddr, err)
	}
	defer udpConn.Close()
	listener := udp.NewListener(store, classifier)
	go listener.Serve(udpConn)
	log.Printf("udp: listening on %s", cfg.UDPAddr)

	// ---- TCP: events, players, chat (and future commands) ----------------
	tcpLn, err := tcp.Listen(cfg.TCPAddr)
	if err != nil {
		log.Fatalf("tcp: listen on %s: %v", cfg.TCPAddr, err)
	}
	defer tcpLn.Close()
	tcpListener := tcp.NewListener(liveStore)
	go tcpListener.Serve(tcpLn)
	log.Printf("tcp: listening on %s", cfg.TCPAddr)

	// ---- HTTP: API + UI ---------------------------------------------------
	statsService := stats.New(database, classifier)
	srv := api.New(cfg, store, liveStore, database, statsService)

	// Persist messages as they arrive, and mirror them over SSE.
	writer := ingest.New(database, liveStore)
	debriefs := debriefstore.New(database)
	debriefs.OnDebrief = func(d model.Debrief) {
		srv.BroadcastMessage(map[string]any{"type": "debrief", "debrief": d})
	}
	tcpListener.OnMessage = func(m model.Message) {
		// Debrief transfers are chunked and reassembled separately.
		if debriefs.Handle(m) {
			return
		}
		if database != nil {
			writer.Handle(m)
		}
		srv.BroadcastMessage(map[string]any{"type": "message", "message": m})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.RunBroadcast(ctx, time.Second)

	// ---- Tracking: positions, losses and sortie analysis -------------------
	track := tracker.New(database, store, tracker.Options{
		SampleEvery: cfg.TrackInterval,
		Grace:       cfg.TrackGrace,
	})
	if database != nil {
		track.SetMissionID(database.OpenMissionID())
		// Follow mission transitions: the ingest writer opens a mission when DCS
		// announces one, and the tracker picks it up on the next tick.
		track.SetMissionIDFunc(database.OpenMissionID)
		// If positions arrive with no mission at all (only Export.lua installed),
		// create a placeholder mission so tracking is still persisted.
		track.SetEnsureMission(func() int64 {
			id, err := database.EnsureMission("Session sans mission", cfg.Theatre)
			if err != nil {
				log.Printf("tracker: ensure mission: %v", err)
				return 0
			}
			return id
		})
	}
	trackStop := make(chan struct{})
	go track.Run(trackStop)
	go track.PruneLoop(trackStop, cfg.TrackRetention)
	log.Printf("tracker: sampling every %s (grace %s, retention %s)",
		cfg.TrackInterval, cfg.TrackGrace, cfg.TrackRetention)

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

	// ---- Shutdown ---------------------------------------------------------
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	close(trackStop)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = tcpLn.Close()
	_ = udpConn.Close()
}
