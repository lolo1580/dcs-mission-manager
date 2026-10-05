// Command dcsmanager is the DCS Manager.
//
// It listens for telemetry from the DCS Lua scripts (UDP for positions, TCP for
// events and players), keeps an in-memory state, persists history to SQLite, and
// serves the embedded web UI (HTTP + Server-Sent Events).
//
// The default mode opens that UI in a native window, so the manager behaves like
// a desktop application rather than a local web server. The `serve` subcommand
// (and any non-interactive environment) keeps the previous headless behaviour:
// the server runs and the UI is reached from a browser.
//
// Usage:
//
//	dcsmanager                    run the manager in a native window (default)
//	dcsmanager serve              run headless (browser UI), as before
//	dcsmanager install-lua        install the DCS-side Lua scripts
//	dcsmanager uninstall-lua      remove the managed Lua artifacts
//	dcsmanager status             report whether the Lua scripts are installed
//	dcsmanager version            print the version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"

	"dcsmanager/internal/app"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/desktop"
	"dcsmanager/internal/install"
	"dcsmanager/internal/migrate"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func init() {
	// The manager reports its version from one place, whichever entry point is
	// used; app.Run logs the same value.
	app.Version = Version
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			if err := app.Run(nil); err != nil {
				if errors.Is(err, app.ErrAlreadyRunning) {
					fmt.Fprintln(os.Stderr, "dcsmanager: another instance is already running (see DCSMANAGER_HTTP_ADDR)")
					os.Exit(3)
				}
				fmt.Fprintf(os.Stderr, "dcsmanager: %v\n", err)
				os.Exit(1)
			}
			return
		case "install-lua":
			os.Exit(runLuaCommand(os.Args[2:], "install"))
		case "uninstall-lua":
			os.Exit(runLuaCommand(os.Args[2:], "uninstall"))
		case "status":
			os.Exit(runLuaCommand(os.Args[2:], "status"))
		case "purge":
			os.Exit(runPurgeCommand(os.Args[2:]))
		case "migrate-db":
			os.Exit(runMigrateCommand(os.Args[2:]))
		case "version", "--version", "-v":
			fmt.Printf("dcsmanager %s\n", Version)
			return
		case "help", "--help", "-h":
			usage()
			return
		}
	}
	desktop.Run()
}

func usage() {
	fmt.Print(`DCS Manager

Usage:
  dcsmanager                 Open the manager in a native window (web UI + DCS ingestion)
  dcsmanager serve           Run headless; open http://localhost:8080 in a browser
  dcsmanager install-lua     Install the Lua scripts into Saved Games
  dcsmanager uninstall-lua   Remove the installed scripts
  dcsmanager status          Report whether the scripts are installed / up to date
  dcsmanager purge           Delete recorded sessions (destructive; see options)
  dcsmanager migrate-db      Copy a SQLite database into PostgreSQL (one-shot)
  dcsmanager version         Print the version

Options for install-lua / uninstall-lua / status:
  --saved-games <dir>   DCS Saved Games directory (auto-detected)
  --lua-dir <dir>       dcs-lua directory of the distribution (auto-detected)
  --dry-run             Show what would be done, without writing anything

Options for migrate-db:
  --from <path>   SQLite database to copy (default: DCSMANAGER_DB_PATH)
  --to <dsn>      PostgreSQL DSN to copy into (default: DCSMANAGER_DB_DSN)

Options for purge (exactly one is required):
  --source test         Delete sessions recorded from the test tools
  --source live         Delete sessions recorded from DCS
  --mission-id <n>      Delete one mission and everything linked to it
  --all                 Delete every recorded session (keeps schema and players)

Server configuration: DCSMANAGER_* environment variables (see README).
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
	// dcsmanager.exe works. An explicit --lua-dir still wins.
	resolvedLua := *luaDir
	if resolvedLua == "" {
		resolvedLua = app.FindLuaDir()
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
		fmt.Println("Backend address: see Saved Games\\DCS\\Config\\dcsmanager.cfg")
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
	dbPath := fs.String("db", "", "database path (defaults to DCSMANAGER_DB_PATH)")
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
		fmt.Fprintln(os.Stderr, "       run `dcsmanager help` for the full usage")
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
		path = "./data/dcsmanager.db"
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

// runMigrateCommand implements `dcsmanager migrate-db`: it copies an existing
// SQLite database into PostgreSQL, preserving ids. It is a one-shot maintenance
// operation rather than a daemon: the PostgreSQL backend has no in-place upgrade
// path, so a user switching engines (or feeding the statistics plugin) needs
// their history carried over.
func runMigrateCommand(args []string) int {
	fs := flag.NewFlagSet("migrate-db", flag.ExitOnError)
	from := fs.String("from", "", "SQLite database to copy (default: DCSMANAGER_DB_PATH)")
	to := fs.String("to", "", "PostgreSQL DSN to copy into (default: DCSMANAGER_DB_DSN)")
	_ = fs.Parse(args)

	cfg := config.Load()

	sqlitePath := *from
	if sqlitePath == "" {
		sqlitePath = cfg.DBPath
	}
	if sqlitePath == "" {
		sqlitePath = "./data/dcsmanager.db"
	}

	dsn := *to
	if dsn == "" {
		dsn = cfg.DBDSN
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "migrate-db: no destination; pass --to <dsn> or set DCSMANAGER_DB_DSN")
		return 2
	}

	fmt.Printf("From     : %s (SQLite)\n", sqlitePath)
	fmt.Printf("To       : %s\n\n", redactDSN(dsn))

	res, err := migrate.Run(context.Background(), sqlitePath, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate-db: %v\n", err)
		fmt.Fprintln(os.Stderr, "Is the destination schema created? Start the manager once with DCSMANAGER_DB_DRIVER=postgres.")
		return 1
	}

	for _, t := range res.Tables {
		fmt.Printf("  copied  %-16s %d\n", t.Table, t.Rows)
	}
	fmt.Printf("\nDone: %d row(s) copied.\n", res.Total)
	return 0
}

// redactDSN hides the password in a PostgreSQL connection string for display.
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
