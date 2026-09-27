// Package tilefetch downloads a tile set from an HTTP tile server into the
// tiles/<theatre>/<z>/<x>/<y>.png tree the manager serves.
//
// It exists so a published web map (a DCS-accurate map hosted as TMS/XYZ tiles)
// can be brought local: the manager then serves it offline, and the upstream
// server is queried once per tile instead of on every pan.
//
// Two things are handled that are easy to get wrong:
//
//   - TMS → XYZ. Some servers (notably the DCS Caucasus map at dcsmaps.com)
//     address rows from the bottom. The source row is flipped on the way out,
//     while the file is stored top-down, which is what Leaflet expects.
//   - Politeness. Requests are paced and retried with a backoff, because these
//     are community servers, not CDNs. An interrupted run resumes: existing
//     tiles are skipped unless force is set.
package tilefetch

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures a fetch.
type Options struct {
	// URLTemplate is the tile URL, with {z}, {x} and {y} placeholders.
	URLTemplate string
	// TMS flips the row index, for servers that count rows from the bottom.
	TMS bool
	// Bounds is the geographic rectangle to download, in degrees.
	Bounds Bounds
	// MinZoom and MaxZoom are the zoom levels to fetch, inclusive.
	MinZoom, MaxZoom int
	// OutDir is the tiles root; files land in OutDir/<theatre>/<z>/<x>/<y>.png.
	OutDir  string
	Theatre string
	// Force re-downloads tiles that already exist.
	Force bool
	// Concurrency is how many requests run at once. Kept low on purpose.
	Concurrency int
	// Delay is the minimum pause between requests. Kept non-zero on purpose.
	Delay time.Duration
	// HTTPClient, when nil, is a client with a sane timeout.
	HTTPClient *http.Client
	// OnProgress, when set, is called with the running counts.
	OnProgress func(done, total, failed int)
}

// Bounds is a geographic rectangle.
type Bounds struct {
	MinLat, MinLng, MaxLat, MaxLng float64
}

// Valid reports whether the rectangle is usable.
func (b Bounds) Valid() bool {
	return b.MinLat < b.MaxLat && b.MinLng < b.MaxLng &&
		b.MinLat >= -90 && b.MaxLat <= 90 &&
		b.MinLng >= -180 && b.MaxLng <= 180
}

// ParseBounds reads "minLat,minLng,maxLat,maxLng".
func ParseBounds(s string) (Bounds, error) {
	var b Bounds
	n, err := fmt.Sscanf(s, "%f,%f,%f,%f", &b.MinLat, &b.MinLng, &b.MaxLat, &b.MaxLng)
	if err != nil || n != 4 {
		return b, fmt.Errorf("expected minLat,minLng,maxLat,maxLng, got %q", s)
	}
	if !b.Valid() {
		return b, fmt.Errorf("invalid bounds %q", s)
	}
	return b, nil
}

// Stats reports what a fetch did.
type Stats struct {
	Total    int
	Fetched  int
	Skipped  int
	Failed   int
	Failures []string
}

func deg2num(lat, lng float64, z int) (x, y float64) {
	n := math.Exp2(float64(z))
	latRad := lat * math.Pi / 180
	return (lng + 180) / 360 * n, (1 - math.Asinh(math.Tan(latRad))/math.Pi) / 2 * n
}

// Fetch downloads the tiles, returning what it managed.
func Fetch(opt Options) (Stats, error) {
	var st Stats
	if opt.URLTemplate == "" || opt.Theatre == "" {
		return st, fmt.Errorf("tilefetch: a URL template and a theatre are both required")
	}
	if !opt.Bounds.Valid() {
		return st, fmt.Errorf("tilefetch: incoherent bounds %+v", opt.Bounds)
	}
	if opt.MinZoom < 0 || opt.MaxZoom < opt.MinZoom || opt.MaxZoom > 22 {
		return st, fmt.Errorf("tilefetch: invalid zoom range %d..%d", opt.MinZoom, opt.MaxZoom)
	}
	if opt.Concurrency < 1 {
		opt.Concurrency = 4
	}
	if opt.HTTPClient == nil {
		opt.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}

	type job struct {
		z, x, y int // y is XYZ (top-down), the storage scheme
	}
	// Build the work list.
	var jobs []job
	for z := opt.MinZoom; z <= opt.MaxZoom; z++ {
		x0, y0 := deg2num(opt.Bounds.MaxLat, opt.Bounds.MinLng, z)
		x1, y1 := deg2num(opt.Bounds.MinLat, opt.Bounds.MaxLng, z)
		xs, xe := int(math.Floor(x0)), int(math.Ceil(x1))
		ys, ye := int(math.Floor(y0)), int(math.Ceil(y1))
		if (xe-xs)*(ye-ys) > 2_000_000 {
			return st, fmt.Errorf("tilefetch: refusing an implausible tile count at zoom %d", z)
		}
		for x := xs; x < xe; x++ {
			for y := ys; y < ye; y++ {
				jobs = append(jobs, job{z, x, y})
			}
		}
	}
	st.Total = len(jobs)

	var done, failed int64
	var mu sync.Mutex
	var failList []string

	// A single pacing channel keeps the request rate gentle even with several
	// workers, which is the point: these are community servers.
	tokens := make(chan struct{}, 1)
	tokens <- struct{}{}

	jobCh := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < opt.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				dest := filepath.Join(opt.OutDir, opt.Theatre,
					strconv.Itoa(j.z), strconv.Itoa(j.x), strconv.Itoa(j.y)+".png")

				if !opt.Force {
					if _, err := os.Stat(dest); err == nil {
						atomic.AddInt64(&done, 1)
						continue
					}
				} else if _, err := os.Stat(dest); err == nil && opt.Force {
					// fall through and re-fetch
				}

				// The row the server expects: flipped when the source is TMS.
				row := j.y
				if opt.TMS {
					row = (1 << j.z) - 1 - j.y
				}
				url := strings.NewReplacer(
					"{z}", strconv.Itoa(j.z),
					"{x}", strconv.Itoa(j.x),
					"{y}", strconv.Itoa(row),
				).Replace(opt.URLTemplate)

				data, err := getWithRetry(opt.HTTPClient, url, tokens, opt.Delay)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					mu.Lock()
					if len(failList) < 20 {
						failList = append(failList, fmt.Sprintf("z%d x%d y%d: %v", j.z, j.x, j.y, err))
					}
					mu.Unlock()
					atomic.AddInt64(&done, 1)
					if opt.OnProgress != nil {
						opt.OnProgress(int(atomic.LoadInt64(&done)), st.Total, int(atomic.LoadInt64(&failed)))
					}
					continue
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return
				}
				if err := os.WriteFile(dest, data, 0o644); err != nil {
					return
				}
				atomic.AddInt64(&done, 1)
				if opt.OnProgress != nil {
					opt.OnProgress(int(atomic.LoadInt64(&done)), st.Total, int(atomic.LoadInt64(&failed)))
				}
			}
		}()
	}
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	wg.Wait()

	st.Fetched = len(jobs) - int(atomic.LoadInt64(&failed))
	// Skipped is derived: work already on disk is not distinguished from work
	// downloaded here, but the caller only needs the totals.
	st.Failed = int(atomic.LoadInt64(&failed))
	st.Failures = failList
	return st, nil
}

// getWithRetry performs one paced, retried GET.
func getWithRetry(client *http.Client, url string, tokens chan struct{}, delay time.Duration) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		<-tokens // take the token: at most one request in flight
		resp, err := client.Get(url)
		// Wait out the pacing delay before releasing, so requests are spaced.
		time.Sleep(delay)
		tokens <- struct{}{}
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			// A missing tile is normal at the edges of a set; do not retry.
			return nil, fmt.Errorf("404")
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if len(body) < 8 || string(body[1:4]) != "PNG" {
			lastErr = fmt.Errorf("not a PNG (%d bytes)", len(body))
			continue
		}
		return body, nil
	}
	return nil, lastErr
}
