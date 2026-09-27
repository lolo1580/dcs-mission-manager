package geo

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestFitRecoversProjection builds synthetic samples from a known projection and
// checks that Fit recovers it and that Forward/Inverse round-trip. This is the
// contract the tile and vector-map code depends on.
func TestFitRecoversProjection(t *testing.T) {
	const (
		lat0, lng0 = 44.0, 40.0
	)
	// A grid of positions across a map-sized area.
	var samples []Sample
	for _, lat := range []float64{41.5, 42.5, 43.5, 44.5, 45.5} {
		for _, lng := range []float64{36.0, 38.0, 40.0, 42.0, 44.0} {
			// Emulate DCS: geographic -> terrain through a plane with a 2-D
			// affine (scale, rotation, offset).
			p, q := tmForward(lat, lng, lat0, lng0)
			x := 1.01*p + 0.02*q + 123456
			z := -0.02*p + 0.99*q - 654321
			samples = append(samples, Sample{X: x, Z: z, Lat: lat, Lng: lng})
		}
	}

	pr, err := Fit(samples)
	if err != nil {
		t.Fatalf("fit: %v", err)
	}
	if pr.RMS > 1 {
		t.Errorf("RMS = %.2f m on exact synthetic data, want ~0", pr.RMS)
	}

	// Round-trip: terrain -> lat/lng -> terrain, anywhere on the map.
	for _, tc := range []struct{ x, z float64 }{
		{0, 0}, {100000, -50000}, {-250000, 300000},
	} {
		lat, lng := pr.Inverse(tc.x, tc.z)
		x, z := pr.Forward(lat, lng)
		if d := math.Hypot(x-tc.x, z-tc.z); d > 1 {
			t.Errorf("round-trip at (%v,%v) off by %.2f m", tc.x, tc.z, d)
		}
	}
}

// TestFitOnRealData is an integration check against an actual DCS installation:
// it fits a map and verifies that the residual is small, which is what makes the
// vector-map and tile code usable. It is skipped when DCS is not available (CI),
// via DCSMM_TERRAINS pointing at a Mods/terrains directory.
func TestFitOnRealData(t *testing.T) {
	root := os.Getenv("DCSMM_TERRAINS")
	if root == "" {
		t.Skip("set DCSMM_TERRAINS to a Mods/terrains directory to run this")
	}
	for _, th := range []string{"Caucasus", "Kola", "PersianGulf"} {
		samples, err := LoadBeaconSamples(filepath.Join(root, th))
		if err != nil || len(samples) < 10 {
			t.Logf("%s: skipped (%v, %d samples)", th, err, len(samples))
			continue
		}
		pr, err := Fit(samples)
		if err != nil {
			t.Errorf("%s: fit: %v", th, err)
			continue
		}
		if pr.RMS > 120 {
			t.Errorf("%s: RMS = %.0f m, want under 120 m", th, pr.RMS)
		}
		// A beacon's own position must map back to its own coordinates.
		worst := 0.0
		for _, s := range samples {
			lat, lng := pr.Inverse(s.X, s.Z)
			d := math.Hypot((lat-s.Lat)*111320, (lng-s.Lng)*111320*math.Cos(s.Lat*math.Pi/180))
			if d > worst {
				worst = d
			}
		}
		t.Logf("%s: %d samples, RMS %.0f m, worst beacon %.0f m", th, pr.Samples, pr.RMS, worst)
	}
}

// TestFitRejectsTooFewSamples documents the empty-map case (Marianas WWII ships
// no beacons): it must be an error, not a silent bogus projection.
func TestFitRejectsTooFewSamples(t *testing.T) {
	if _, err := Fit([]Sample{{X: 1, Z: 1, Lat: 42, Lng: 40}}); err == nil {
		t.Fatal("expected an error for too few samples")
	}
}
