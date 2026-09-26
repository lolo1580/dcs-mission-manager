// Command dcsmm is the DCS Mission Manager backend.
//
// It listens for telemetry from the DCS Lua scripts (UDP for positions, TCP for
// events and players), keeps an in-memory state, persists history to SQLite, and
// exposes everything to the web UI (HTTP + Server-Sent Events).
//
// Usage:
//
//	dcsmm                    run the manager (default)
//	dcsmm install-lua        install the DCS-side Lua scripts
//	dcsmm uninstall-lua      remove the managed Lua artifacts
//	dcsmm status             report whether the Lua scripts are installed
//	dcsmm version            print the version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dcsmm/internal/aerodrome"
	"dcsmm/internal/api"
	"dcsmm/internal/category"
	"dcsmm/internal/config"
	"dcsmm/internal/db"
	"dcsmm/internal/dcsdir"
	"dcsmm/internal/debriefstore"
	"dcsmm/internal/ingest"
	"dcsmm/internal/install"
	"dcsmm/internal/live"
	"dcsmm/internal/model"
	"dcsmm/internal/source"
	"dcsmm/internal/state"
	"dcsmm/internal/stats"
	"dcsmm/internal/tcp"
	"dcsmm/internal/tracker"
	"dcsmm/internal/udp"
	"dcsmm/internal/visibility"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install-lua":
			os.Exit(runLuaCommand(os.Args[2:], "install"))
		case "uninstall-lua":
			os.Exit(runLuaCommand(os.Args[2:], "uninstall"))
		case "status":
			os.Exit(runLuaCommand(os.Args[2:], "status"))
		case "purge":
			os.Exit(runPurgeCommand(os.Args[2:]))
		case "version", "--version", "-v":
			fmt.Printf("dcsmm %s\n", Version)
			return
		case "help", "--help", "-h":
			usage()
			return
		}
	}
	runServer()
}

func usage() {
	fmt.Print(`DCS Mission Manager

Usage:
  dcsmm                 Run the manager (web UI + DCS ingestion)
  dcsmm install-lua     Install the Lua scripts into Saved Games
  dcsmm uninstall-lua   Remove the installed scripts
  dcsmm status          Report whether the scripts are installed / up to date
  dcsmm purge           Delete recorded sessions (destructive; see options)
  dcsmm version         Print the version

Options for install-lua / uninstall-lua / status:
  --saved-games <dir>   DCS Saved Games directory (auto-detected)
  --lua-dir <dir>       dcs-lua directory of the distribution (auto-detected)
  --dry-run             Show what would be done, without writing anything

Options for purge (exactly one is required):
  --source test         Delete sessions recorded from the test tools
  --source live         Delete sessions recorded from DCS
  --mission-id <n>      Delete one mission and everything linked to it
  --all                 Delete every recorded session (keeps schema and players)

Server configuration: DCSMM_* environment variables (see README).
`)
}

// runLuaCommand implements install-lua / uninstall-lua / status.
func runLuaCommand(args []string, mode string) int {
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	savedGames := fs.String("saved-games", "", "DCS Saved Games directory")
	luaDir := fs.String("lua-dir", "", "dcs-lua directory of the distribution")
	dryRun := fs.Bool("dry-run", false, "write nothing, show the actions")
	_ = fs.Parse(args)

	// Resolve the Lua source directory. It is optional: when none is found, the
	// installer falls back to the scripts embedded in the binary, so a lone
	// dcsmm.exe works. An explicit --lua-dir still wins.
	resolvedLua := *luaDir
	if resolvedLua == "" {
		resolvedLua = findLuaDir()
	}

	// Resolve Saved Games: explicit flag or auto-detection.
	resolvedSG := *savedGames
	if resolvedSG == "" {
		var err error
		resolvedSG, err = install.FindSavedGames()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}

	ins := install.New(resolvedLua, resolvedSG)
	ins.DryRun = *dryRun

	if resolvedLua == "" {
		fmt.Println("Scripts  : embedded in the binary")
	} else {
		fmt.Printf("Scripts  : %s\n", resolvedLua)
	}
	fmt.Printf("DCS      : %s\n", resolvedSG)
	if *dryRun {
		fmt.Println("Mode      : dry run (no writes)")
	}
	fmt.Println()

	var (
		results []install.Result
		err     error
	)
	switch mode {
	case "install":
		results, err = ins.Install(install.DefaultTargets())
	case "uninstall":
		results, err = ins.Uninstall()
	case "status":
		results = ins.Status(install.DefaultTargets())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printResults(results)
	if mode == "install" && !*dryRun {
		fmt.Println()
		fmt.Println("Remember to restart DCS so the scripts are reloaded.")
		fmt.Println("Backend address: see Saved Games\\DCS\\Config\\dcsmm.cfg")
	}
	return 0
}

// runPurgeCommand implements the destructive `purge` subcommand. Exactly one
// scope must be given, so a mistyped command can never delete more than asked.
func runPurgeCommand(args []string) int {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	source := fs.String("source", "", "delete missions of this source: live|test")
	missionID := fs.Int64("mission-id", 0, "delete this mission and its data")
	all := fs.Bool("all", false, "delete every recorded session")
	dbPath := fs.String("db", "", "database path (defaults to DCSMM_DB_PATH)")
	dryRun := fs.Bool("dry-run", false, "report what would be deleted, delete nothing")
	_ = fs.Parse(args)

	chosen := 0
	if *source != "" {
		chosen++
	}
	if *missionID != 0 {
		chosen++
	}
	if *all {
		chosen++
	}
	if chosen != 1 {
		fmt.Fprintln(os.Stderr, "purge: pass exactly one of --source <live|test>, --mission-id <n> or --all")
		fmt.Fprintln(os.Stderr, "       run `dcsmm help` for the full usage")
		return 2
	}
	if *source != "" && !db.ValidSource(*source) {
		fmt.Fprintf(os.Stderr, "purge: --source must be live or test, got %q\n", *source)
		return 2
	}

	path := *dbPath
	if path == "" {
		path = config.Load().DBPath
	}
	if path == "" {
		path = "./data/dcsmm.db"
	}

	database, err := db.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "purge: open %s: %v\n", path, err)
		return 1
	}
	defer database.Close()

	fmt.Printf("Database : %s\n", path)

	liveCount, err := database.CountMissions(db.SourceLive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "purge: %v\n", err)
		return 1
	}
	testCount, err := database.CountMissions(db.SourceTest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "purge: %v\n", err)
		return 1
	}
	fmt.Printf("Sessions : %d live, %d test\n", liveCount, testCount)

	if *dryRun {
		fmt.Println("Mode     : dry run (nothing is deleted)")
		switch {
		case *all:
			fmt.Println("Would delete every recorded session and all tracking data.")
		case *source != "":
			fmt.Printf("Would delete the %d %s session(s) and their data.\n", countFor(*source, liveCount, testCount), *source)
		default:
			fmt.Printf("Would delete mission #%d and its data.\n", *missionID)
		}
		return 0
	}

	var res db.PurgeResult
	switch {
	case *all:
		res, err = database.PurgeAll()
	case *source != "":
		res, err = database.PurgeSource(*source)
	default:
		res, err = database.PurgeMission(*missionID)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "purge: %v\n", err)
		return 1
	}

	fmt.Println()
	fmt.Printf("  removed  missions        %d\n", res.Missions)
	for _, table := range []string{"track_positions", "losses", "events", "chat", "player_stats", "debriefs"} {
		if n, ok := res.Deleted[table]; ok {
			fmt.Printf("  removed  %-15s %d\n", table, n)
		}
	}
	fmt.Printf("\nDone: %d row(s) deleted.\n", res.Total())
	return 0
}

func countFor(source string, live, test int) int {
	if source == db.SourceTest {
		return test
	}
	return live
}

func printResults(results []install.Result) {
	if len(results) == 0 {
		fmt.Println("Nothing to do.")
		return
	}
	for _, r := range results {
		note := ""
		if r.Note != "" {
			note = " (" + r.Note + ")"
		}
		backup := ""
		if r.Backup != "" {
			backup = "  [backup: " + r.Backup + "]"
		}
		fmt.Printf("  %-14s %-36s%s%s\n", r.Action, r.DestRel, note, backup)
	}
}

// findLuaDir looks for the dcs-lua directory next to the executable, then in the
// current directory and its parents (so it works from `go run`).
func findLuaDir() string {
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

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// runServer runs the manager.
func runServer() {
	cfg := config.Load()
	log.Printf("dcsmm %s", Version)

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
		log.Printf("source: forced to %q by DCSMM_SOURCE", forcedSource)
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
	go tcpListener.Serve(tcpLn)
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
			total += r.Airfields
		}
		if len(terrainReports) > 0 {
			log.Printf("aerodrome: %d airfields read from DCS", total)
		} else {
			log.Printf("aerodrome: %d airfields loaded from the embedded dataset", airfields.Count())
		}
	}
	visibilityPolicy := visibility.New(cfg.RevealAllUnits)
	if cfg.RevealAllUnits {
		log.Printf("visibility: filtering disabled (DCSMM_REVEAL_ALL_UNITS=true)")
	} else {
		log.Printf("visibility: fog of war enforced (restrictive)")
	}
	srv := api.New(cfg, store, liveStore, database, statsService, airfields, visibilityPolicy)
	// The mission's F10 view options drive fog-of-war filtering.
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
