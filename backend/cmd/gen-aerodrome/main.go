// Command gen-aerodrome produces an embedded aerodrome dataset for a DCS
// theatre, from the terrain files of an installed DCS.
//
// It is a development tool, not part of the manager's runtime. It exists so the
// committed JSON under internal/aerodrome/data/ can be regenerated and traced
// back to the simulator's own files, instead of being typed by hand.
//
// Usage:
//
//	go run ./cmd/gen-aerodrome <Mods/terrains> <TheatreID> <out.json>
//
// Example, for Cold War Germany:
//
//	go run ./cmd/gen-aerodrome "E:\Games\DCS World\Mods\terrains" GermanyCW internal/aerodrome/data/germanycw.json
//
// DCS exposes an airfield's radio frequency in radio.lua and its position in
// beacons.lua. An airfield with no navigation aid has a frequency but no
// position; it is kept, because the manager still lists it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dcsmanager/internal/aerodrome"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: gen-aerodrome <Mods/terrains> <TheatreID> <out.json>")
		os.Exit(2)
	}
	terrainsDir, theatreID, outPath := os.Args[1], os.Args[2], os.Args[3]

	cat, _, err := aerodrome.LoadWithDCS(terrainsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	list := cat.ByTheatre(theatreID)
	if len(list) == 0 {
		fmt.Fprintf(os.Stderr, "no airfield for theatre %q\n", theatreID)
		os.Exit(1)
	}

	seen := map[string]bool{}
	out := make([]aerodrome.Aerodrome, 0, len(list))
	for _, a := range list {
		// DCS radio codes double as the per-terrain identifier, but a few hold
		// spaces ("Mainz Finthen"), which make poor ids: compact them. Collisions
		// are avoided with a numeric suffix rather than by dropping an entry.
		id := compact(a.ID)
		if id == "" {
			id = compact(a.Name)
		}
		base := id
		for n := 2; seen[id]; n++ {
			id = fmt.Sprintf("%s%d", base, n)
		}
		seen[id] = true

		// Strip the fields that are runtime-only or theatre-wide noise: the
		// loader recomputes Source, the API fills DistanceKm, and ICAOCode here
		// is just the radio code again.
		a.ID = id
		a.ICAOCode = ""
		a.Source = ""
		a.DistanceKm = 0
		out = append(out, a)
	}

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}

	withPos, withTower := 0, 0
	for _, a := range out {
		if a.Lat != 0 || a.Lng != 0 {
			withPos++
		}
		if a.Tower != 0 {
			withTower++
		}
	}
	fmt.Printf("%s: wrote %d airfields to %s (%d with a position, %d with a tower)\n",
		theatreID, len(out), outPath, withPos, withTower)
}

// compact keeps only letters and digits and uppercases, so an id is a single
// path-safe token.
func compact(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
