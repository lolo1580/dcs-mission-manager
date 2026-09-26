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
	"dcsmm/internal/debriefstore"
	"dcsmm/internal/ingest"
	"dcsmm/internal/install"
	"dcsmm/internal/live"
	"dcsmm/internal/model"
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
  dcsmm version         Print the version

Options for install-lua / uninstall-lua / status:
  --saved-games <dir>   DCS Saved Games directory (auto-detected)
  --lua-dir <dir>       dcs-lua directory of the distribution (auto-detected)
  --dry-run             Show what would be done, without writing anything

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

	// Resolve the Lua source directory: explicit flag, next to the executable,
	// or the current working directory (for `go run`).
	resolvedLua := *luaDir
	if resolvedLua == "" {
		resolvedLua = findLuaDir()
	}
	if resolvedLua == "" {
		fmt.Fprintln(os.Stderr, "dcs-lua directory not found. Use --lua-dir <path>")
		return 1
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

	fmt.Printf("Scripts  : %s\n", resolvedLua)
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
	airfields, err := aerodrome.Load()
	if err != nil {
		log.Printf("aerodrome: dataset unavailable: %v", err)
		airfields = nil
	} else {
		log.Printf("aerodrome: %d airfields loaded", airfields.Count())
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
		track.SetEnsureMission(func() int64 {
			id, err := database.EnsureMission("Session without mission", cfg.Theatre)
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
