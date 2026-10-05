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
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"dcsmanager/internal/aerodrome"
	"dcsmanager/internal/api"
	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/category"
	"dcsmanager/internal/charts"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/db/postgres"
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/dcsdata"
	"dcsmanager/internal/dcsdir"
	"dcsmanager/internal/debriefstore"
	"dcsmanager/internal/ingest"
	"dcsmanager/internal/install"
	"dcsmanager/internal/live"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/model"
	"dcsmanager/internal/panelservice"
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

	store := state.New(cfg.UnitTTL, cfg.MaxUnits)
	classifier := category.New(cfg.CategoriesFile)
	liveStore := live.New(1000, 500)

	// ---- Optional persistence --------------------------------------------
	// database is a db.Store (an interface), not *db.DB, so that "persistence
	// disabled" stays a true nil interface. Assigning a typed nil *db.DB to a
	// db.Store would produce a non-nil interface holding a nil pointer, which
	// would defeat every downstream `== nil` guard (the API, the stats service
	// and the tracker all rely on it).
	var database db.Store
	if cfg.DBEnabled {
		opened, err := openStore(cfg)
		if err != nil {
			log.Printf("db: disabled, could not open (%s): %v", cfg.DBDriver, err)
		} else {
			database = opened
			defer database.Close()
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
	// The module inventory comes from DCS's own list, so the UI can show what is
	// installed and owned rather than a hand-maintained catalogue. "Owned" is the
	// store's have="1" (bought); whether a module is actually on disk is answered
	// by the installation itself, autoupdate.cfg at the game root.
	if cfg.SavedGames != "" {
		if inv, err := dcsdata.LoadModules(cfg.SavedGames, dcsInstall); err != nil {
			log.Printf("modules: inventory unavailable: %v", err)
		} else if inv.Total > 0 {
			log.Printf("modules: %d entries, %d owned, %d installed on disk (%s)",
				inv.Total, inv.Owned, inv.Installed, dcsdata.ModuleInventoryPath(cfg.SavedGames))
			srv.SetModules(inv)
		} else {
			log.Printf("modules: none found in %s (optional)", dcsdata.ModuleInventoryPath(cfg.SavedGames))
		}

		// The logbook is the player's own career record, kept by DCS.
		if lb, err := dcsdata.LoadLogbook(cfg.SavedGames); err != nil {
			log.Printf("career: logbook unavailable: %v", err)
		} else if len(lb.Players) > 0 {
			log.Printf("career: %d profile(s) from %s", len(lb.Players), dcsdata.LogbookPath(cfg.SavedGames))
			srv.SetLogbook(lb)
		} else {
			log.Printf("career: no logbook in %s (optional)", dcsdata.LogbookPath(cfg.SavedGames))
		}

		// The mods installed under Mods/.
		if mods, err := dcsdata.InstalledMods(cfg.SavedGames); err != nil {
			log.Printf("mods: listing failed: %v", err)
		} else if len(mods) > 0 {
			log.Printf("mods: %d installed in %s", len(mods), dcsdata.ModsDir(cfg.SavedGames))
			srv.SetMods(mods)
		} else {
			log.Printf("mods: none in %s (optional)", dcsdata.ModsDir(cfg.SavedGames))
		}

		// The DCS-side script status: our own files via the installer's status,
		// plus other tools' hooks.
		ins := install.New(FindLuaDir(), cfg.SavedGames)
		managed := ins.Status(install.DefaultTargets())
		states := make([]dcsdata.ScriptState, 0, len(managed))
		for _, r := range managed {
			states = append(states, dcsdata.ScriptState{
				DestRel: filepath.ToSlash(r.DestRel),
				State:   r.Action,
				Note:    r.Note,
			})
		}
		scripts := dcsdata.InspectScripts(cfg.SavedGames, states)
		srv.SetScripts(scripts)
		log.Printf("scripts: %d managed, %d third-party hook(s)",
			len(scripts.Managed), len(scripts.ThirdParty))
	}
	// The mission's options are recorded for the session description.
	tcpListener.OnOptions = srv.ApplyMissionOptions

	// Persist messages as they arrive, and mirror them over SSE.
	writer := ingest.New(database, liveStore)
	writer.SetSourceFunc(sessionSource)
	debriefs := debriefstore.New(database)
	// A corrupt assembled debrief is written beside the database so it can be
	// examined, rather than dropped with only its first log line.
	debriefs.DumpDir = filepath.Join(filepath.Dir(cfg.DBPath), "rejected")
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

	// ---- Panels and DCS-BIOS: cockpit hardware ----------------------------
	// The manager drives Logitech panels directly and speaks DCS-BIOS' protocol,
	// so a cockpit with either can be watched. Both are best-effort: a machine
	// without panels or without DCS-BIOS simply sees nothing here.
	bios := dcsbios.New(dcsbios.DefaultOptions(), func(st dcsbios.State) {
		srv.BroadcastMessage(map[string]any{
			"type": "dcsbios",
			"state": map[string]any{
				"connected": st.Connected,
				"aircraft":  st.Aircraft,
				"frames":    st.Frames,
			},
		})
	})
	if err := bios.Start(); err != nil {
		log.Printf("dcsbios: listener unavailable: %v", err)
	} else {
		defer bios.Stop()
		log.Printf("dcsbios: listening on %s:%d", dcsbios.DefaultMulticast, dcsbios.DefaultReceivePort)
	}

	// The binding store sends DCS-BIOS commands when a panel control moves. It
	// starts disabled on every run: sending into a live cockpit is opt-in.
	mappings := mapping.NewStore(mappingPath(cfg), func(line string) error {
		identifier, value := parseCommandLine(line)
		return bios.SendCommand(identifier, value)
	})

	panelSvc := panelservice.New(panelservice.DefaultOptions(), func(e panelservice.Event) {
		srv.BroadcastMessage(map[string]any{"type": "panel", "panel": api.PanelEventJSON(e)})
		// A panel input drives the bound command, when the aircraft is known and
		// sending is enabled.
		if e.Kind == panelservice.KindInput {
			if aircraft := bios.State().Aircraft; aircraft != "" {
				if cat, err := biosmeta.LoadModule(cfg.SavedGames, aircraft); err == nil {
					for _, line := range mappings.Apply(aircraft, e.Input, cat) {
						srv.BroadcastMessage(map[string]any{"type": "command", "command": line})
					}
				}
			}
		}
	})
	panelSvc.Start()
	defer panelSvc.Stop()

	srv.SetPanels(panelSvc, bios)
	srv.SetMappings(mappings)

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

// openStore opens the configured persistence engine. SQLite stays the default;
// PostgreSQL is opt-in through DCSMANAGER_DB_DRIVER=postgres and a DSN.
func openStore(cfg config.Config) (db.Store, error) {
	switch cfg.DBDriver {
	case "postgres", "postgresql", "pg":
		if cfg.DBDSN == "" {
			return nil, errors.New("DCSMANAGER_DB_DRIVER=postgres requires DCSMANAGER_DB_DSN")
		}
		store, err := postgres.Open(cfg.DBDSN)
		if err != nil {
			return nil, err
		}
		log.Printf("db: using postgres (dsn %s)", redactDSN(cfg.DBDSN))
		return store, nil
	default:
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return nil, err
		}
		log.Printf("db: using sqlite %s", cfg.DBPath)
		return store, nil
	}
}

// redactDSN hides the password in a PostgreSQL connection string before it is
// written to the log.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		return dsn
	}
	if _, hasPw := u.User.Password(); hasPw {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	return u.String()
}

// mappingPath is where the panel-to-command bindings live: beside the database, so
// the whole manager's state sits in one folder.
func mappingPath(cfg config.Config) string {
	dir := filepath.Dir(cfg.DBPath)
	if dir == "" || dir == "." {
		return "mappings.json"
	}
	return filepath.Join(dir, "mappings.json")
}

// parseCommandLine splits the "IDENTIFIER value\n" line the mapping store builds
// back into the parts SendCommand wants. A line it cannot parse yields an empty
// identifier, which SendCommand refuses rather than sending nonsense.
func parseCommandLine(line string) (string, int) {
	line = strings.TrimSpace(line)
	i := strings.LastIndexByte(line, ' ')
	if i <= 0 {
		return "", 0
	}
	identifier := line[:i]
	value := 0
	for _, r := range line[i+1:] {
		if r == '-' {
			value = -value
			continue
		}
		if r < '0' || r > '9' {
			return identifier, value
		}
		value = value*10 + int(r-'0')
	}
	return identifier, value
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
