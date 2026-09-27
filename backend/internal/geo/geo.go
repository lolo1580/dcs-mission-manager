// Package geo converts between DCS terrain coordinates (metres, x/z) and
// geographic coordinates (latitude/longitude).
//
// DCS projects each terrain with its own transverse Mercator. The parameters are
// not documented, but DCS ships its own ground truth: every beacon in
// beacons.lua carries both the terrain position and the real
// latitude/longitude, so the projection can be fitted per map from the installed
// data — to a few tens of metres over a whole theatre (see
// docs/terrain-projection.md).
//
// Everything here is pure computation: no file access beyond reading beacons.lua
// to gather the fit samples.
package geo

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dcsmm/internal/lua"
)

// earthRadius is the sphere the transverse Mercator is computed on. The affine
// step absorbs any scale difference from the real ellipsoid.
const earthRadius = 6371000.0

// Sample is one known correspondence: a terrain position and its real position.
type Sample struct {
	X, Z float64
	Lat  float64
	Lng  float64
	Name string
}

// Projection maps terrain coordinates to geographic ones and back.
//
// The forward path is: geographic -> transverse Mercator plane -> affine ->
// terrain. The affine absorbs the real scale, rotation and false origin that
// remain once the shape is right, so only (Lat0, Lng0, K0) have to be found.
type Projection struct {
	Lat0, Lng0, K0 float64
	AX, BX, CX     float64 // terrain x = AX*p + BX*q + CX
	AZ, BZ, CZ     float64 // terrain z = AZ*p + BZ*q + CZ
	// RMS is the fit residual, in metres.
	RMS float64
	// Samples is how many pairs the fit used, after trimming outliers.
	Samples int
}

// tmForward is the spherical transverse Mercator: (lat,lng) -> plane (p,q),
// in metres, relative to the central meridian.
func tmForward(lat, lng, lat0, lng0 float64) (p, q float64) {
	dl := (lng - lng0) * math.Pi / 180
	la := lat * math.Pi / 180
	la0 := lat0 * math.Pi / 180
	b := math.Cos(la) * math.Sin(dl)
	if b >= 1 {
		b = math.Nextafter(1, 0)
	}
	if b <= -1 {
		b = math.Nextafter(-1, 0)
	}
	p = earthRadius * math.Atanh(b)
	q = earthRadius * (math.Atan2(math.Tan(la), math.Cos(dl)) - la0)
	return
}

// tmInverse recovers (lat,lng) from the plane position and the parameters.
func tmInverse(p, q, lat0, lng0 float64) (lat, lng float64) {
	pp := p / earthRadius
	d := q/earthRadius + lat0*math.Pi/180
	sinD, cosD := math.Sin(d), math.Cos(d)
	sinPhi := sinD / math.Cosh(pp)
	if sinPhi > 1 {
		sinPhi = 1
	} else if sinPhi < -1 {
		sinPhi = -1
	}
	lat = math.Asin(sinPhi) * 180 / math.Pi
	lng = lng0 + math.Atan2(math.Sinh(pp), cosD)*180/math.Pi
	return
}

// Forward returns the terrain coordinates of a geographic position.
func (pr Projection) Forward(lat, lng float64) (x, z float64) {
	p, q := tmForward(lat, lng, pr.Lat0, pr.Lng0)
	p *= pr.K0
	q *= pr.K0
	return pr.AX*p + pr.BX*q + pr.CX, pr.AZ*p + pr.BZ*q + pr.CZ
}

// Inverse returns the geographic position of terrain coordinates.
func (pr Projection) Inverse(x, z float64) (lat, lng float64) {
	det := pr.AX*pr.BZ - pr.BX*pr.AZ
	if det == 0 {
		return 0, 0
	}
	dx, dz := x-pr.CX, z-pr.CZ
	p := (pr.BZ*dx - pr.BX*dz) / det
	q := (pr.AX*dz - pr.AZ*dx) / det
	if pr.K0 != 0 {
		p /= pr.K0
		q /= pr.K0
	}
	return tmInverse(p, q, pr.Lat0, pr.Lng0)
}

// affineRMS fits terrain = affine(plane) for the given parameters and returns
// the RMS residual in metres.
func affineRMS(s []Sample, lat0, lng0, k0 float64) float64 {
	n := float64(len(s))
	var mx, mz, mp, mq float64
	P := make([]float64, len(s))
	Q := make([]float64, len(s))
	for i, v := range s {
		P[i], Q[i] = tmForward(v.Lat, v.Lng, lat0, lng0)
		P[i] *= k0
		Q[i] *= k0
		mx += v.X
		mz += v.Z
		mp += P[i]
		mq += Q[i]
	}
	mx, mz, mp, mq = mx/n, mz/n, mp/n, mq/n

	var sPP, sPQ, sQQ, sPx, sQx, sPz, sQz float64
	for i := range s {
		dp, dq := P[i]-mp, Q[i]-mq
		sPP += dp * dp
		sPQ += dp * dq
		sQQ += dq * dq
		sPx += dp * (s[i].X - mx)
		sQx += dq * (s[i].X - mx)
		sPz += dp * (s[i].Z - mz)
		sQz += dq * (s[i].Z - mz)
	}
	det := sPP*sQQ - sPQ*sPQ
	if det == 0 {
		return math.Inf(1)
	}
	ax := (sQQ*sPx - sPQ*sQx) / det
	bx := (sPP*sQx - sPQ*sPx) / det
	az := (sQQ*sPz - sPQ*sQz) / det
	bz := (sPP*sQz - sPQ*sPz) / det
	cx := mx - ax*mp - bx*mq
	cz := mz - az*mp - bz*mq

	var acc float64
	for i := range s {
		px := ax*P[i] + bx*Q[i] + cx
		pz := az*P[i] + bz*Q[i] + cz
		acc += (px-s[i].X)*(px-s[i].X) + (pz-s[i].Z)*(pz-s[i].Z)
	}
	return math.Sqrt(acc / n)
}

// Fit finds the projection parameters that best explain the samples.
//
// The search is over the central meridian and reference latitude only: the
// affine step absorbs the scale, so K0 stays at 1. Outliers are trimmed
// iteratively, because a few beacons far from the map carry coordinates that do
// not follow the same projection and would otherwise dominate the fit.
func Fit(samples []Sample) (Projection, error) {
	if len(samples) < 4 {
		return Projection{}, fmt.Errorf("geo: need at least 4 samples, got %d", len(samples))
	}

	var minLat, maxLat, minLng, maxLng float64
	for i, s := range samples {
		if i == 0 {
			minLat, maxLat, minLng, maxLng = s.Lat, s.Lat, s.Lng, s.Lng
		}
		minLat, maxLat = math.Min(minLat, s.Lat), math.Max(maxLat, s.Lat)
		minLng, maxLng = math.Min(minLng, s.Lng), math.Max(maxLng, s.Lng)
	}

	// Trimmed residual: fit, drop the worst 10 %, repeat.
	trimmed := func(sub []Sample, lat0, lng0 float64) float64 {
		rms := affineRMS(sub, lat0, lng0, 1)
		return rms
	}
	robust := func(lat0, lng0 float64) float64 {
		cur := samples
		for round := 0; round < 3 && len(cur) > 8; round++ {
			// Rank by residual under the current parameters.
			n := float64(len(cur))
			var mx, mz, mp, mq float64
			P := make([]float64, len(cur))
			Q := make([]float64, len(cur))
			for i, v := range cur {
				P[i], Q[i] = tmForward(v.Lat, v.Lng, lat0, lng0)
				mx += v.X
				mz += v.Z
				mp += P[i]
				mq += Q[i]
			}
			mx, mz, mp, mq = mx/n, mz/n, mp/n, mq/n
			var sPP, sPQ, sQQ, sPx, sQx, sPz, sQz float64
			for i := range cur {
				dp, dq := P[i]-mp, Q[i]-mq
				sPP += dp * dp
				sPQ += dp * dq
				sQQ += dq * dq
				sPx += dp * (cur[i].X - mx)
				sQx += dq * (cur[i].X - mx)
				sPz += dp * (cur[i].Z - mz)
				sQz += dq * (cur[i].Z - mz)
			}
			det := sPP*sQQ - sPQ*sPQ
			if det == 0 {
				break
			}
			ax := (sQQ*sPx - sPQ*sQx) / det
			bx := (sPP*sQx - sPQ*sPx) / det
			az := (sQQ*sPz - sPQ*sQz) / det
			bz := (sPP*sQz - sPQ*sPz) / det
			cx := mx - ax*mp - bx*mq
			cz := mz - az*mp - bz*mq
			idx := make([]int, len(cur))
			res := make([]float64, len(cur))
			for i := range cur {
				px := ax*P[i] + bx*Q[i] + cx
				pz := az*P[i] + bz*Q[i] + cz
				res[i] = math.Hypot(px-cur[i].X, pz-cur[i].Z)
				idx[i] = i
			}
			sort.Slice(idx, func(a, b int) bool { return res[idx[a]] < res[idx[b]] })
			keep := int(float64(len(cur)) * 0.9)
			if keep < 4 {
				break
			}
			next := make([]Sample, 0, keep)
			for _, i := range idx[:keep] {
				next = append(next, cur[i])
			}
			cur = next
		}
		return trimmed(cur, lat0, lng0)
	}

	best := Projection{RMS: math.Inf(1)}
	cLat, cLng := (minLat+maxLat)/2, (minLng+maxLng)/2
	// lat0 and lng0 are only defined up to what the affine can absorb, so many
	// parameter sets fit the data equally well. Prefer the one closest to the
	// map's own centre: it keeps Forward/Inverse well-behaved outside the sample
	// area instead of relying on a freak cancellation.
	type cand struct {
		lat0, lng0, rms, dev float64
	}
	var cands []cand
	for _, dLat := range []float64{0, -3, 3, -6, 6} {
		for _, dLng := range []float64{0, -6, -3, 3, 6, -12, 12} {
			lat0, lng0 := cLat+dLat, cLng+dLng
			r := robust(lat0, lng0)
			cands = append(cands, cand{lat0, lng0, r, math.Hypot(dLat, dLng)})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].rms < cands[j].rms })
	floor := cands[0].rms * 1.01
	chosen := cands[0]
	for _, c := range cands {
		if c.rms <= floor && c.dev < chosen.dev {
			chosen = c
		}
	}
	best = Projection{Lat0: chosen.lat0, Lng0: chosen.lng0, K0: 1, RMS: chosen.rms}

	// Refine locally around the best candidate.
	for _, step := range []float64{2, 0.5, 0.1, 0.02} {
		improved := true
		for improved {
			improved = false
			for _, dLat := range []float64{-step, 0, step} {
				for _, dLng := range []float64{-step, 0, step} {
					lat0, lng0 := best.Lat0+dLat, best.Lng0+dLng
					if r := robust(lat0, lng0); r < best.RMS {
						best.Lat0, best.Lng0, best.RMS = lat0, lng0, r
						improved = true
					}
				}
			}
		}
	}

	// Final affine on the trimmed set, kept for Forward/Inverse.
	fit := affine(samples, best.Lat0, best.Lng0, best.K0)
	fit.Lat0, fit.Lng0, fit.K0 = best.Lat0, best.Lng0, best.K0
	fit.RMS = best.RMS
	return fit, nil
}

// affine solves terrain = affine(plane) on every sample, returning the
// coefficients and the count used.
func affine(s []Sample, lat0, lng0, k0 float64) Projection {
	pr := Projection{Lat0: lat0, Lng0: lng0, K0: k0}
	n := float64(len(s))
	var mx, mz, mp, mq float64
	P := make([]float64, len(s))
	Q := make([]float64, len(s))
	for i, v := range s {
		P[i], Q[i] = tmForward(v.Lat, v.Lng, lat0, lng0)
		P[i] *= k0
		Q[i] *= k0
		mx += v.X
		mz += v.Z
		mp += P[i]
		mq += Q[i]
	}
	mx, mz, mp, mq = mx/n, mz/n, mp/n, mq/n
	var sPP, sPQ, sQQ, sPx, sQx, sPz, sQz float64
	for i := range s {
		dp, dq := P[i]-mp, Q[i]-mq
		sPP += dp * dp
		sPQ += dp * dq
		sQQ += dq * dq
		sPx += dp * (s[i].X - mx)
		sQx += dq * (s[i].X - mx)
		sPz += dp * (s[i].Z - mz)
		sQz += dq * (s[i].Z - mz)
	}
	det := sPP*sQQ - sPQ*sPQ
	if det == 0 {
		return pr
	}
	pr.AX = (sQQ*sPx - sPQ*sQx) / det
	pr.BX = (sPP*sQx - sPQ*sPx) / det
	pr.AZ = (sQQ*sPz - sPQ*sQz) / det
	pr.BZ = (sPP*sQz - sPQ*sPz) / det
	pr.CX = mx - pr.AX*mp - pr.BX*mq
	pr.CZ = mz - pr.AZ*mp - pr.BZ*mq
	pr.Samples = len(s)
	return pr
}

// LoadBeaconSamples reads the (terrain position, geographic position) pairs from
// a terrain's beacons.lua.
func LoadBeaconSamples(terrainDir string) ([]Sample, error) {
	raw, err := os.ReadFile(filepath.Join(terrainDir, "beacons.lua"))
	if err != nil {
		return nil, err
	}
	root, err := lua.Parse(stripPreamble(raw))
	if err != nil {
		return nil, fmt.Errorf("geo: parse beacons.lua: %w", err)
	}
	var out []Sample
	for _, v := range tableEntries(root["beacons"]) {
		e, ok := v.(map[string]any)
		if !ok {
			continue
		}
		pos, _ := e["position"].(map[string]any)
		ge, _ := e["positionGeo"].(map[string]any)
		if pos == nil || ge == nil {
			continue
		}
		s := Sample{}
		s.Name, _ = e["display_name"].(string)
		s.X, _ = pos["1"].(float64)
		s.Z, _ = pos["3"].(float64)
		s.Lat, _ = ge["latitude"].(float64)
		s.Lng, _ = ge["longitude"].(float64)
		if s.Lat == 0 && s.Lng == 0 {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func stripPreamble(raw []byte) []byte {
	var out []byte
	for _, line := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "dofile(") || strings.HasPrefix(t, "local ") ||
			strings.HasPrefix(t, "require(") || strings.HasPrefix(t, "--") {
			continue
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

// tableEntries returns the positional entries of a parsed Lua table.
func tableEntries(v any) []any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]int, 0, len(t))
		for k := range t {
			var n int
			if _, err := fmt.Sscanf(k, "%d", &n); err == nil {
				keys = append(keys, n)
			}
		}
		sort.Ints(keys)
		out := make([]any, 0, len(keys))
		for _, n := range keys {
			out = append(out, t[fmt.Sprint(n)])
		}
		return out
	case []any:
		return t
	}
	return nil
}
