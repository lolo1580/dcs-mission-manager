// Command geoexplore reports the terrain projection fitted for each map, from
// the ground truth in beacons.lua. It is the CLI over internal/geo, useful to
// inspect a map or check the fit after a DCS update.
//
// usage: go run ./cmd/geoexplore <terrainsDir> <Theatre> [Theatre...]
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"dcsmm/internal/geo"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: geoexplore <terrainsDir> <Theatre> [Theatre...]")
		return
	}
	dir := os.Args[1]
	bad := false
	for _, th := range os.Args[2:] {
		samples, err := geo.LoadBeaconSamples(filepath.Join(dir, th))
		if err != nil {
			fmt.Printf("%-20s %v\n", th, err)
			bad = true
			continue
		}
		pr, err := geo.Fit(samples)
		if err != nil {
			// A map with no beacon data (Marianas WWII ships an empty
			// beacons.lua) is a normal case, not a failure.
			fmt.Printf("%-20s %3d pairs  (no projection possible)\n", th, len(samples))
			continue
		}
		// Report the fit error on the false-origin point, as a sanity check that
		// the forward and inverse paths agree.
		lat, lng := pr.Inverse(pr.CX, pr.CZ)
		x, z := pr.Forward(lat, lng)
		fmt.Printf("%-20s %3d pairs  lat0=%.4f lng0=%.4f  RMS=%.0f m  k0=%.3f  roundtrip=%.2f m\n",
			th, pr.Samples, pr.Lat0, pr.Lng0, pr.RMS, pr.K0,
			math.Hypot(x-pr.CX, z-pr.CZ))
	}
	if bad {
		os.Exit(1)
	}
}
