// Package app runs the manager: it wires the UDP/TCP ingestion, the in-memory
// state, the SQLite persistence, the tracking, the HTTP API and the embedded
// web UI, and blocks until the process is asked to stop.
//
// Run is the single source of truth for that wiring, shared by the two entry
// points of the binary: the headless/CLI mode (cmd/dcsmanager) and the desktop
// window (internal/desktop). Keeping it in one place means the two modes can
// never drift apart, which matters because the desktop mode is meant to be the
// same manager with a native window instead of a browser tab.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"dcsmanager/internal/aerodrome"
	"dcsmanager/internal/api"
	"dcsmanager/internal/category"
	"dcsmanager/internal/charts"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/dcsdir"
	"dcsmanager/internal/debriefstore"
	"dcsmanager/internal/ingest"
	"dcsmanager/internal/live"
	"dcsmanager/internal/model"
	"dcsmanager/internal/source"
	"dcsmanager/internal/state"
	"dcsmanager/internal/stats"
	"dcsmanager/internal/tcp"
	"dcsmanager/internal/tracker"
	"dcsmanager/internal/udp"
)

// Version is injected at build time (-ldflags "-X main.Version=...").
var Version = "dev"

// stopCh lets a non-signal actor (the desktop window) ask Run to shut down.
// Closing the window, for instance, must quit the manager rather than leave it
// running with no interface.
var (
	stopCh   = make(chan struct{})
	stopOnce sync.Once
)

// Stop asks a running manager to shut down and return from Run. It is safe to
// call more than once.
func Stop() {
	stopOnce.Do(func() { close(stopCh) })
}

// ErrAlreadyRunning is returned by Run when another manager instance already
// answers on the configured HTTP address. It is a distinct, expected outcome:
// the caller can surface it (rather than appear to do nothing) and, in window
// mode, focus the instance already running instead of starting a second one.
var ErrAlreadyRunning = errors.New("another dcsmanager instance is already running")

// Run starts the manager and blocks until it is told to shut down. It returns
// ErrAlreadyRunning without starting anything when another instance already
// serves the configured HTTP address.
//
// onReady, when non-nil, is called once every listener is up and the HTTP server
// is accepting connections. It receives the resolved HTTP address (which may
// differ from cfg.HTTPAddr when a port of 0 was requested) so a desktop shell
// can point its window at the right URL. It runs on its own goroutine.
func Run(onReady func(addr string)) error {
	cfg := config.Load()

	// An instance bound to the configured port is running by definition; starting
	// a second one would fail on the UDP bind a moment later, with a message the
	// user never sees (a double-clicked exe has no console). Detect it here and
	// say so clearly instead.
	if probeExistingServer(cfg.HTTPAddr) {
		return ErrAlreadyRunning
	}

	log.Printf("dcsmanager %s", Version)
	log.Printf("dcsmanager %s", Version)

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

	// ---- Session source: DCS or the test tools ---------------------------
	// The tools speak the same protocol as DCS, so detection happens on the
	// wire. The resulting source is shared by both persistence paths so a test
	// session is tagged no matter which one creates the mission.
	detector := source.NewDetector()
	forcedSource := source.Forced()
	sessionSource := func() string {
		if forcedSource != "" {
			return forcedSource
		}
		if detector.Observed() {
			return db.SourceTest
		}
		return db.SourceLive
	}
	if forcedSource != "" {
		log.Printf("source: forced to %q by DCSMANAGER_SOURCE", forcedSource)
	}

	// ---- UDP: unit positions ---------------------------------------------
	udpConn, err := udp.Listen(cfg.UDPAddr)
	if err != nil {
		log.Fatalf("udp: listen on %s: %v", cfg.UDPAddr, err)
	}
	defer udpConn.Close()
	listener := udp.NewListener(store, classifier)
	listener.SetSourceDetector(func(payload []byte) bool {
		fired := detector.Observe(payload)
		if fired && forcedSource == "" {
			log.Printf("source: simulated telemetry detected, the session will be recorded as %q", db.SourceTest)
		}
		return fired
	})
	go listener.Serve(udpConn)
	log.Printf("udp: listening on %s", cfg.UDPAddr)

	// ---- TCP: events, players, chat (and future commands) ----------------
	tcpLn, err := tcp.Listen(cfg.TCPAddr)
	if err != nil {
		log.Fatalf("tcp: listen on %s: %v", cfg.TCPAddr, err)
	}
	defer tcpLn.Close()
	tcpListener := tcp.NewListener(liveStore)
	// The listener is served only once its callbacks are wired (see below).
	// Starting it here and assigning them later would be a data race: a
	// connection handled in that window reads a nil callback and the message is
	// silently dropped.
	log.Printf("tcp: listening on %s", cfg.TCPAddr)

	// ---- HTTP: API + UI ---------------------------------------------------
	statsService := stats.New(database, classifier)

	// Airfields come from DCS's own terrain files when they can be read, and
	// fall back to the dataset embedded in the binary otherwise. Reading them is
	// possible because the manager runs on the same machine as the simulator.
	dcsInstall := dcsdir.Find(cfg.SavedGames)
	terrainsDir := dcsdir.TerrainsDir(dcsInstall)
	if dcsInstall == "" {
		log.Printf("dcs: installation not found, using the embedded airfield dataset")
	} else if terrainsDir == "" {
		log.Printf("dcs: %s has no Mods/terrains, using the embedded airfield dataset", dcsInstall)
	} else {
		log.Printf("dcs: installation at %s", dcsInstall)
	}

	airfields, terrainReports, err := aerodrome.LoadWithDCS(terrainsDir)
	if err != nil {
		log.Printf("aerodrome: dataset unavailable: %v", err)
		airfields = nil
	} else {
		total := 0
		for _, r := range terrainReports {
			if r.Error != "" {
				log.Printf("aerodrome: %s: %s", r.Theatre, r.Error)
				continue
			}
			log.Printf("aerodrome: %s: %d airfields (%d radio, %d beacons, %d towns)",
				r.Theatre, r.Airfields, r.RadioAirfields, r.BeaconTotal, r.Towns)
			// An airfield DCS gives no position for cannot be placed on the map.
			// Saying so beats letting it vanish silently. Marianas WWII is the
			// case in point: the terrain ships no beacon data at all.
			if missing := r.Airfields - r.WithPosition; missing > 0 {
				log.Printf("aerodrome: %s: %d airfield(s) have no position in DCS data and cannot be mapped",
					r.Theatre, missing)
			}
			for _, d := range r.Dropped {
				log.Printf("aerodrome: dropped %s", d)
			}
			total += r.Airfields
		}
		if len(terrainReports) > 0 {
			log.Printf("aerodrome: %d airfields read from DCS", total)
		} else {
			log.Printf("aerodrome: %d airfields loaded from the embedded dataset", airfields.Count())
		}
	}
	// Aeronautical charts live on disk as scans and are never shipped: they are
	// documents, indexed by name and displayed as-is.
	chartCatalog, err := charts.Load(cfg.ChartsDir)
	if err != nil {
		log.Printf("charts: index unavailable: %v", err)
		chartCatalog = nil
	} else if chartCatalog.Count() > 0 {
		log.Printf("charts: %d charts indexed from %s (%s)",
			chartCatalog.Count(), cfg.ChartsDir, strings.Join(chartCatalog.Theatres(), ", "))
	} else {
		log.Printf("charts: none found in %s (optional)", cfg.ChartsDir)
	}

	srv := api.New(cfg, store, liveStore, database, statsService, airfields, chartCatalog, tcpListener)
	// The mission's options are recorded for the session description.
	tcpListener.OnOptions = srv.ApplyMissionOptions

	// Persist messages as they arrive, and mirror them over SSE.
	writer := ingest.New(database, liveStore)
	writer.SetSourceFunc(sessionSource)
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

	// Every callback is in place: the listener may now accept connections.
	go tcpListener.Serve(tcpLn)

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
		track.SetMissionIDFunc(database.OpenMissionID)
		track.SetMissionSource(sessionSource)
		track.SetEnsureMission(func() int64 {
			id, err := database.EnsureMissionTagged("Session without mission", cfg.Theatre, sessionSource())
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

	// Listen ourselves rather than calling ListenAndServe, so that a port of 0
	// can be resolved to the real one before the window is opened.
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("http: listen on %s: %v", cfg.HTTPAddr, err)
	}
	defer ln.Close()

	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()
	httpAddr := ln.Addr().String()
	log.Printf("http: web ui on http://%s", httpAddr)

	if onReady != nil {
		go onReady(httpAddr)
	}

	// ---- Shutdown ---------------------------------------------------------
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-stopCh:
	}

	log.Println("shutting down...")
	close(trackStop)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	_ = tcpLn.Close()
	_ = udpConn.Close()
	return nil
}

// probeExistingServer reports whether another manager answers on the HTTP
// address. It asks for the health endpoint and checks the service name, so an
// unrelated program that happens to hold the port is never mistaken for us.
//
// Only a concrete host is probed: with a port of 0 (or a wildcard host) there is
// nothing meaningful to ask, and the caller resolves the real address anyway.
func probeExistingServer(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "0" || host == "" || host == "0.0.0.0" || host == "::" {
		return false
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var health struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&health); err != nil {
		return false
	}
	return health.Service == "dcsmanager"
}

// LogFilePath is where a window-mode launch writes its log, since a
// double-clicked executable has no console to print to. It lives next to the
// SQLite database.
func LogFilePath() (string, error) {
	path := config.Load().DBPath
	if path == "" {
		return "", errors.New("no database path configured")
	}
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "dcsmanager.log"), nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// FindLuaDir looks for the dcs-lua directory next to the executable, then in
// the current directory and its parents (so it works from `go run`).
func FindLuaDir() string {
	if exe, err := os.Executable(); err == nil {
		for _, dir := range []string{
			filepath.Join(filepath.Dir(exe), "dcs-lua"),
			filepath.Join(filepath.Dir(exe), "..", "dcs-lua"),
		} {
			if isDir(dir) {
				return dir
			}
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; ; {
		candidate := filepath.Join(dir, "dcs-lua")
		if isDir(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
