// Command dcsmm is the DCS Mission Manager.
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
//	dcsmm                    run the manager in a native window (default)
//	dcsmm serve              run headless (browser UI), as before
//	dcsmm install-lua        install the DCS-side Lua scripts
//	dcsmm uninstall-lua      remove the managed Lua artifacts
//	dcsmm status             report whether the Lua scripts are installed
//	dcsmm version            print the version
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dcsmm/internal/app"
	"dcsmm/internal/config"
	"dcsmm/internal/db"
	"dcsmm/internal/dcsvectors"
	"dcsmm/internal/desktop"
	"dcsmm/internal/imgtiles"
	"dcsmm/internal/install"
	"dcsmm/internal/mbtiles"
	"dcsmm/internal/tilefetch"
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
					fmt.Fprintln(os.Stderr, "dcsmm: another instance is already running (see DCSMM_HTTP_ADDR)")
					os.Exit(3)
				}
				fmt.Fprintf(os.Stderr, "dcsmm: %v\n", err)
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
		case "import-tiles":
			os.Exit(runImportTiles(os.Args[2:]))
		case "import-image":
			os.Exit(runImportImage(os.Args[2:]))
		case "fetch-tiles":
			os.Exit(runFetchTiles(os.Args[2:]))
		case "import-vectors":
			os.Exit(runImportVectors(os.Args[2:]))
		case "version", "--version", "-v":
			fmt.Printf("dcsmm %s\n", Version)
			return
		case "help", "--help", "-h":
			usage()
			return
		}
	}
	desktop.Run()
}

func usage() {
	fmt.Print(`DCS Mission Manager

Usage:
  dcsmm                 Open the manager in a native window (web UI + DCS ingestion)
  dcsmm serve           Run headless; open http://localhost:8080 in a browser
  dcsmm install-lua     Install the Lua scripts into Saved Games
  dcsmm uninstall-lua   Remove the installed scripts
  dcsmm status          Report whether the scripts are installed / up to date
  dcsmm purge           Delete recorded sessions (destructive; see options)
  dcsmm import-tiles    Import an MBTiles pack (DCS F10 map) as map tiles
  dcsmm import-image    Slice a calibrated map image into map tiles
  dcsmm fetch-tiles     Download a published tile set (a DCS-accurate web map)
  dcsmm import-vectors  Convert DCS terrain shapefiles to GeoJSON
  dcsmm version         Print the version

Options for import-tiles:
  --mbtiles <file>      MBTiles archive to import (required)
  --theatre <id>        Theatre id to store it under, e.g. PersianGulf (required)
  --out <dir>           Tiles directory (defaults to DCSMM_TILES_DIR)
  --force               Overwrite tiles that already exist

Options for import-image:
  --image <file>        Map image to slice, JPG or PNG (required)
  --theatre <id>        Theatre id to store it under (required)
  --bounds <b>          minLat,minLng,maxLat,maxLng the image covers (required)
  --min-zoom <n>        First zoom level to generate (default 2)
  --max-zoom <n>        Last zoom level to generate (default 6)
  --out <dir>           Tiles directory (defaults to DCSMM_TILES_DIR)
  --force               Overwrite tiles that already exist

Options for fetch-tiles:
  --url <template>      Tile URL with {z} {x} {y} placeholders (required)
  --theatre <id>        Theatre id to store it under (required)
  --bounds <b>          minLat,minLng,maxLat,maxLng to download (required)
  --min-zoom <n>        First zoom level (default 8)
  --max-zoom <n>        Last zoom level (default 12)
  --tms                 Source counts rows from the bottom (TMS, not XYZ)
  --out <dir>           Tiles directory (defaults to DCSMM_TILES_DIR)
  --force               Re-download tiles that already exist
  --delay <ms>          Minimum pause between requests (default 120)
  --concurrency <n>     Parallel requests (default 4, kept low on purpose)

Options for import-vectors:
  --in <dir>            Directory holding the .shp/.dbf files (required)
  --theatre <id>        Theatre id to store them under (required)
  --out <dir>           GeoJSON output root (defaults to DCSMM_VECTORS_DIR)
  --simplify <deg>      Drop vertices closer than this, in degrees (default 0)
  --drop <names>        Comma-separated attribute columns to omit

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
		fmt.Println("Backend address: see Saved Games\\DCS\\Config\\dcsmm.cfg")
	}
	return 0
}

// runImportTiles implements `import-tiles`: it turns an MBTiles pack (the
// community DCS F10 map releases) into the tiles/<theatre>/<z>/<x>/<y>.png tree
// the manager already serves.
func runImportTiles(args []string) int {
	fs := flag.NewFlagSet("import-tiles", flag.ExitOnError)
	mbtilesPath := fs.String("mbtiles", "", "MBTiles archive to import")
	theatre := fs.String("theatre", "", "theatre id to store it under (e.g. PersianGulf)")
	out := fs.String("out", "", "tiles directory (defaults to DCSMM_TILES_DIR)")
	force := fs.Bool("force", false, "overwrite tiles that already exist")
	_ = fs.Parse(args)

	if *mbtilesPath == "" || *theatre == "" {
		fmt.Fprintln(os.Stderr, "import-tiles: --mbtiles and --theatre are both required")
		return 2
	}
	dir := *out
	if dir == "" {
		dir = config.Load().TilesDir
	}
	if dir == "" {
		dir = "./tiles"
	}

	db, err := mbtiles.Open(*mbtilesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-tiles: %v\n", err)
		return 1
	}
	defer db.Close()

	md, err := db.Metadata()
	if err != nil {
		// Metadata is informative; a pack without it can still be converted.
		fmt.Fprintf(os.Stderr, "import-tiles: warning: %v\n", err)
	}
	fmt.Printf("Archive  : %s\n", *mbtilesPath)
	if md.Name != "" {
		fmt.Printf("Name     : %s\n", md.Name)
	}
	if md.Format != "" {
		fmt.Printf("Format   : %s\n", md.Format)
	}
	if md.MaxZoom > 0 {
		fmt.Printf("Zoom     : %d..%d\n", md.MinZoom, md.MaxZoom)
	}
	fmt.Printf("Theatre  : %s\n", *theatre)
	fmt.Printf("Target   : %s\n\n", filepath.Join(dir, *theatre))

	st, err := db.Convert(dir, *theatre, !*force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-tiles: %v\n", err)
		return 1
	}
	fmt.Printf("Tiles    : %d read (zoom %d..%d)\n", st.Tiles, st.MinZoom, st.MaxZoom)
	fmt.Printf("           %d written, %d kept, %d re-encoded to PNG\n", st.Written, st.Skipped, st.Transcoded)
	if st.Written == 0 && st.Skipped == 0 {
		fmt.Fprintln(os.Stderr, "\nNothing was written: is the archive empty?")
		return 1
	}
	fmt.Printf("\nDone. Restart the manager (or it will pick the tiles up on the next start).\n")
	return 0
}

// runImportImage implements `import-image`: it slices a single calibrated map
// image (the freeware DCS F10 packs and the chart scans in maps_dcs/ are
// delivered as one big JPG) into the tiles/<theatre>/<z>/<x>/<y>.png tree.
func runImportImage(args []string) int {
	fs := flag.NewFlagSet("import-image", flag.ExitOnError)
	imagePath := fs.String("image", "", "map image to slice (JPG or PNG)")
	theatre := fs.String("theatre", "", "theatre id to store it under")
	boundsStr := fs.String("bounds", "", "minLat,minLng,maxLat,maxLng the image covers")
	minZoom := fs.Int("min-zoom", 2, "first zoom level to generate")
	maxZoom := fs.Int("max-zoom", 6, "last zoom level to generate")
	out := fs.String("out", "", "tiles directory (defaults to DCSMM_TILES_DIR)")
	force := fs.Bool("force", false, "overwrite tiles that already exist")
	_ = fs.Parse(args)

	if *imagePath == "" || *theatre == "" || *boundsStr == "" {
		fmt.Fprintln(os.Stderr, "import-image: --image, --theatre and --bounds are all required")
		return 2
	}
	bounds, err := imgtiles.ParseBounds(*boundsStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-image: %v\n", err)
		return 2
	}
	dir := *out
	if dir == "" {
		dir = config.Load().TilesDir
	}
	if dir == "" {
		dir = "./tiles"
	}

	fmt.Printf("Image    : %s\n", *imagePath)
	fmt.Printf("Bounds   : %.4f,%.4f .. %.4f,%.4f\n",
		bounds.MinLat, bounds.MinLng, bounds.MaxLat, bounds.MaxLng)
	fmt.Printf("Zoom     : %d..%d\n", *minZoom, *maxZoom)
	fmt.Printf("Theatre  : %s\n\n", *theatre)

	st, err := imgtiles.Convert(*imagePath, *theatre, bounds, *minZoom, *maxZoom, dir, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-image: %v\n", err)
		return 1
	}
	fmt.Printf("Tiles    : %d written, %d kept (zoom %d..%d)\n", st.Written, st.Skipped, st.MinZoom, st.MaxZoom)
	if st.Written == 0 && st.Skipped == 0 {
		fmt.Fprintln(os.Stderr, "\nNothing was written: do the bounds overlap the image?")
		return 1
	}
	fmt.Printf("\nDone. Restart the manager (or it will pick the tiles up on the next start).\n")
	return 0
}

// runFetchTiles implements `fetch-tiles`: it brings a published tile set (a
// DCS-accurate web map hosted as a tile server) local, so the manager serves it
// offline.
func runFetchTiles(args []string) int {
	fs := flag.NewFlagSet("fetch-tiles", flag.ExitOnError)
	url := fs.String("url", "", "tile URL with {z} {x} {y} placeholders")
	theatre := fs.String("theatre", "", "theatre id to store it under")
	boundsStr := fs.String("bounds", "", "minLat,minLng,maxLat,maxLng to download")
	minZoom := fs.Int("min-zoom", 8, "first zoom level")
	maxZoom := fs.Int("max-zoom", 12, "last zoom level")
	tms := fs.Bool("tms", false, "the source counts rows from the bottom (TMS)")
	out := fs.String("out", "", "tiles directory (defaults to DCSMM_TILES_DIR)")
	force := fs.Bool("force", false, "re-download tiles that already exist")
	delay := fs.Int("delay", 120, "minimum pause between requests, in ms")
	conc := fs.Int("concurrency", 4, "parallel requests")
	_ = fs.Parse(args)

	if *url == "" || *theatre == "" || *boundsStr == "" {
		fmt.Fprintln(os.Stderr, "fetch-tiles: --url, --theatre and --bounds are all required")
		return 2
	}
	bounds, err := tilefetch.ParseBounds(*boundsStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch-tiles: %v\n", err)
		return 2
	}
	dir := *out
	if dir == "" {
		dir = config.Load().TilesDir
	}
	if dir == "" {
		dir = "./tiles"
	}

	fmt.Printf("Source   : %s\n", *url)
	fmt.Printf("Bounds   : %.4f,%.4f .. %.4f,%.4f\n",
		bounds.MinLat, bounds.MinLng, bounds.MaxLat, bounds.MaxLng)
	fmt.Printf("Zoom     : %d..%d   TMS: %v\n", *minZoom, *maxZoom, *tms)
	fmt.Printf("Theatre  : %s\n\n", *theatre)

	start := time.Now()
	st, err := tilefetch.Fetch(tilefetch.Options{
		URLTemplate: *url,
		TMS:         *tms,
		Bounds:      bounds,
		MinZoom:     *minZoom,
		MaxZoom:     *maxZoom,
		OutDir:      dir,
		Theatre:     *theatre,
		Force:       *force,
		Concurrency: *conc,
		Delay:       time.Duration(*delay) * time.Millisecond,
		OnProgress: func(done, total, failed int) {
			if done%200 == 0 || done == total {
				fmt.Printf("\r  %d/%d (%d failed)", done, total, failed)
			}
		},
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch-tiles: %v\n", err)
		return 1
	}
	fmt.Printf("Tiles    : %d downloaded, %d failed, in %s\n",
		st.Fetched, st.Failed, time.Since(start).Round(time.Second))
	for _, f := range st.Failures {
		fmt.Fprintf(os.Stderr, "  %s\n", f)
	}
	if st.Fetched == 0 && st.Total > 0 {
		fmt.Fprintln(os.Stderr, "\nNothing was downloaded: check the URL template and bounds.")
		return 1
	}
	fmt.Printf("\nDone. Restart the manager (or it will pick the tiles up on the next start).\n")
	return 0
}

// runImportVectors implements `import-vectors`: it converts the shapefiles that
// describe DCS's terrain features into GeoJSON, which the map draws directly.
func runImportVectors(args []string) int {
	fs := flag.NewFlagSet("import-vectors", flag.ExitOnError)
	in := fs.String("in", "", "directory holding the .shp/.dbf files")
	theatre := fs.String("theatre", "", "theatre id to store them under")
	out := fs.String("out", "", "GeoJSON output root (defaults to DCSMM_VECTORS_DIR)")
	simplify := fs.Float64("simplify", 0, "drop vertices closer than this, in degrees")
	drop := fs.String("drop", "", "comma-separated attribute columns to omit")
	_ = fs.Parse(args)

	if *in == "" || *theatre == "" {
		fmt.Fprintln(os.Stderr, "import-vectors: --in and --theatre are both required")
		return 2
	}
	dir := *out
	if dir == "" {
		dir = config.Load().VectorsDir
	}
	if dir == "" {
		dir = "./vectors"
	}

	cols, err := dcsvectors.ToGeoJSON(*in, *simplify, dcsvectors.ParseDrop(*drop))
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-vectors: %v\n", err)
		return 1
	}
	if len(cols) == 0 {
		fmt.Fprintln(os.Stderr, "import-vectors: no .shp files found in "+*in)
		return 1
	}

	fmt.Printf("Source   : %s\n", *in)
	fmt.Printf("Theatre  : %s\n", *theatre)
	if *simplify > 0 {
		fmt.Printf("Simplify : %g deg\n", *simplify)
	}
	fmt.Printf("Target   : %s\n\n", filepath.Join(dir, *theatre))

	names := make([]string, 0, len(cols))
	for n := range cols {
		names = append(names, n)
	}
	sort.Strings(names)

	var total int64
	for _, n := range names {
		c := cols[n]
		path := filepath.Join(dir, *theatre, n+".geojson")
		size, err := dcsvectors.WriteTo(path, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %-40s %v\n", n, err)
			continue
		}
		total += size
		fmt.Printf("  %-42s %-10s %d Ko\n", n, c.Describe(), size/1024)
	}
	fmt.Printf("\nDone: %d layer(s), %d Ko total.\n", len(names), total/1024)
	fmt.Println("Restart the manager so it picks the layers up.")
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
