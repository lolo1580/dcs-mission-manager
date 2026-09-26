// Package tracker turns the live unit feed into a persisted history: periodic
// position samples (for trails and heatmaps) and loss events (units that vanish,
// presumed destroyed or despawned).
//
// It deliberately works from the lat/lng positions already received by the live
// map, so it needs no per-theatre projection.
package tracker

import (
	"log"
	"math"
	"sync"
	"time"

	"dcsmm/internal/db"
	"dcsmm/internal/model"
	"dcsmm/internal/state"
)

// Tracker periodically samples the unit store and detects losses.
type Tracker struct {
	db    *db.DB
	store *state.Store

	mu          sync.Mutex
	missionID   int64
	seen        map[string]model.Sample // last known state per unit id
	sampleEvery time.Duration
	// grace is how long a unit must be absent before it is considered lost.
	grace time.Duration

	// lostIDs holds units already reported as lost, so they are reported once.
	lostIDs map[string]bool

	// missionIDFn, when set, is consulted when no mission has been set
	// explicitly. It lets the tracker follow mission transitions announced by
	// the hooks without a hard dependency on the ingest package.
	missionIDFn func() int64
	// ensureMissionFn returns an open mission, creating one when needed. It is
	// used when positions arrive with no mission ever announced.
	ensureMissionFn func() int64
	// missionSourceFn, when set, reports the source of the running session
	// ("live" or "test"). It lets the tracker tag a mission it creates itself
	// with the same source the rest of the pipeline uses, so simulated
	// positions never land in a mission counted as real.
	missionSourceFn func() string
}

// Options configures a Tracker.
type Options struct {
	// SampleEvery is the interval between position samples.
	SampleEvery time.Duration
	// Grace is how long a unit must be missing before being counted as a loss.
	Grace time.Duration
}

// New creates a tracker. A nil database disables persistence (the tracker then
// only maintains its in-memory view, which is harmless).
func New(database *db.DB, store *state.Store, opts Options) *Tracker {
	if opts.SampleEvery <= 0 {
		opts.SampleEvery = 3 * time.Second
	}
	return &Tracker{
		db:          database,
		store:       store,
		seen:        make(map[string]model.Sample),
		lostIDs:     make(map[string]bool),
		sampleEvery: opts.SampleEvery,
		grace:       opts.Grace,
	}
}

// SetMissionID sets the mission the samples belong to.
func (t *Tracker) SetMissionID(id int64) {
	t.mu.Lock()
	t.missionID = id
	t.mu.Unlock()
}

// SetMissionIDFunc sets a fallback used when no mission has been set explicitly,
// so the tracker follows mission transitions automatically. When it returns 0
// the tracker has no mission to attach samples to.
func (t *Tracker) SetMissionIDFunc(fn func() int64) {
	t.mu.Lock()
	t.missionIDFn = fn
	t.mu.Unlock()
}

// SetEnsureMission sets a function consulted when the tracker has to open a
// mission on its own. It is only a notification hook: the tracker creates the
// mission itself, through its own mutex, so that the get-or-create can never
// race with another component doing the same.
func (t *Tracker) SetEnsureMission(fn func() int64) {
	t.mu.Lock()
	t.ensureMissionFn = fn
	t.mu.Unlock()
}

// SetMissionSource sets a function reporting the current session source
// ("live" or "test"). It is consulted when the tracker has to create a mission
// on its own, so the created mission carries the right tag.
func (t *Tracker) SetMissionSource(fn func() string) {
	t.mu.Lock()
	t.missionSourceFn = fn
	t.mu.Unlock()
}

// currentSource returns the source of the running session, defaulting to live.
func (t *Tracker) currentSource() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sourceLocked()
}

// currentMissionID returns the mission to attach samples to.
func (t *Tracker) currentMissionID() int64 {
	t.mu.Lock()
	id := t.missionID
	fn := t.missionIDFn
	t.mu.Unlock()

	if id != 0 {
		return id
	}
	if fn != nil {
		if got := fn(); got != 0 {
			return got
		}
	}
	// Nothing announced: fall back to a default mission carrying the running
	// session's source (live or test).
	return t.EnsureMission()
}

// EnsureMission returns the current mission, creating a default one when the
// tracker has to persist positions without any mission ever having been
// announced. The mission carries the running session's source, so simulated
// positions are never attributed to a "live" mission.
//
// The get-or-create runs under t.mu, so two callers can never open two missions
// for the same session.
func (t *Tracker) EnsureMission() int64 {
	if t.db == nil {
		return 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Re-read the current mission while holding the lock: another goroutine may
	// have opened one since the caller last looked.
	if id := t.db.OpenMissionID(); id != 0 {
		t.missionID = id
		return id
	}
	// The notification hook may be set, but the tracker still opens the mission
	// itself, under this lock, so the create is serialised.
	if t.ensureMissionFn != nil {
		t.ensureMissionFn()
		if id := t.db.OpenMissionID(); id != 0 {
			t.missionID = id
			return id
		}
	}
	id, err := t.db.EnsureMissionTagged("Session without mission", "", t.sourceLocked())
	if err != nil {
		return 0
	}
	t.missionID = id
	return id
}

// sourceLocked reports the session source. t.mu must be held.
func (t *Tracker) sourceLocked() string {
	if t.missionSourceFn == nil {
		return db.SourceLive
	}
	if s := t.missionSourceFn(); db.ValidSource(s) {
		return s
	}
	return db.SourceLive
}

// hasTracked reports whether any unit has ever been sampled. It keeps loss
// detection running after the last unit disappears: the tracker is the only
// component that can notice a unit vanished, so it must keep ticking.
func (t *Tracker) hasTracked() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen) > 0
}

// Run samples the store every sampleEvery until ctx is done.
func (t *Tracker) Run(stop <-chan struct{}) {
	ticker := time.NewTicker(t.sampleEvery)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.Tick()
		}
	}
}

// PruneLoop periodically deletes tracking data older than retention, keeping the
// database bounded on long-running servers. It returns when stop is closed.
func (t *Tracker) PruneLoop(stop <-chan struct{}, retention time.Duration) {
	if t.db == nil || retention <= 0 {
		return
	}
	// Prune once an hour: retention is measured in days, so precision here is
	// irrelevant and an hourly pass is cheap.
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if n, err := t.db.PruneTracking(retention); err != nil {
				log.Printf("tracker: prune: %v", err)
			} else if n > 0 {
				log.Printf("tracker: pruned %d old tracking rows", n)
			}
		}
	}
}

// Tick performs one sampling + loss-detection pass. Exported for tests.
func (t *Tracker) Tick() {
	units := t.store.Snapshot()
	now := time.Now()
	nowMs := now.UnixMilli()

	// Do not open a mission just because the sampler ticked. Without this, an
	// idle backend would create an empty "mission" at every start, which then
	// shows up in the UI as a phantom session with no data.
	if len(units) == 0 && !t.hasTracked() {
		return
	}

	missionID := t.currentMissionID()

	seenNow := make(map[string]bool, len(units))
	samples := make([]model.Sample, 0, len(units))

	for _, u := range units {
		seenNow[u.ID] = true
		t.mu.Lock()
		t.seen[u.ID] = model.Sample{
			UnitID:    u.ID,
			Name:      u.Label,
			Type:      u.Type,
			Category:  u.Category,
			Coalition: u.Coalition,
			Lat:       u.Lat,
			Lng:       u.Lng,
			Alt:       u.Alt,
			Heading:   u.Heading,
			Ownship:   u.Ownship,
			RealTS:    nowMs,
		}
		// A unit that reappears is no longer considered lost.
		delete(t.lostIDs, u.ID)
		t.mu.Unlock()

		samples = append(samples, model.Sample{
			MissionID: missionID,
			UnitID:    u.ID,
			Name:      u.Label,
			Type:      u.Type,
			Category:  u.Category,
			Coalition: u.Coalition,
			Lat:       u.Lat,
			Lng:       u.Lng,
			Alt:       u.Alt,
			Heading:   u.Heading,
			Speed:     u.Speed,
			G:         u.G,
			Ownship:   u.Ownship,
			RealTS:    nowMs,
		})
	}

	if t.db != nil && missionID > 0 {
		// If the session has since been recognised as simulated, promote the
		// mission. The tracker may have opened it as "live" at an earlier tick,
		// before the test packet arrived.
		if source := t.currentSource(); source != "" {
			if err := t.db.UpgradeMissionSource(missionID, source); err != nil {
				log.Printf("tracker: upgrade mission source: %v", err)
			}
		}
		if err := t.db.SaveSamples(missionID, samples); err != nil {
			log.Printf("tracker: save samples: %v", err)
		}
	}

	t.detectLosses(seenNow, missionID, nowMs)
}

// detectLosses records units present in the previous sample but absent now,
// after the grace period.
func (t *Tracker) detectLosses(seenNow map[string]bool, missionID, nowMs int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// A grace of zero means "report as soon as a unit is missing".
	cutoff := time.Now().Add(-t.grace).UnixMilli()

	for id, s := range t.seen {
		if seenNow[id] {
			// Still reported: tracking continues, and it is not lost. Tick
			// already clears lostIDs for a unit that reappears.
			continue
		}
		if s.RealTS > cutoff {
			continue // within the grace period
		}

		// Missing beyond the grace: record the loss once, then forget the unit.
		// Without the deletion, both maps grow with every unit ever seen and
		// every tick scans all of them.
		if t.db != nil && missionID > 0 {
			if err := t.db.SaveLoss(missionID, s); err != nil {
				log.Printf("tracker: save loss %s: %v", id, err)
			}
		}
		delete(t.seen, id)
		delete(t.lostIDs, id)
	}
}

// Stats is a sortie analysis for one unit track.
type Stats struct {
	UnitID      string  `json:"unitId"`
	Name        string  `json:"name,omitempty"`
	Type        string  `json:"type,omitempty"`
	Points      int     `json:"points"`
	DistanceKm  float64 `json:"distanceKm"`
	MaxAlt      float64 `json:"maxAlt"`
	MaxSpeed    float64 `json:"maxSpeed"`
	MaxG        float64 `json:"maxG"`
	AvgSpeed    float64 `json:"avgSpeed"`
	DurationSec float64 `json:"durationSec"`
}

// Analyse computes per-unit sortie statistics from stored trails.
func Analyse(trails map[string][]db.TrailPoint) []Stats {
	out := make([]Stats, 0, len(trails))
	for unitID, pts := range trails {
		if len(pts) < 2 {
			continue
		}
		s := Stats{UnitID: unitID, Points: len(pts)}
		var totalSpeed float64
		for i, p := range pts {
			if p.Alt > s.MaxAlt {
				s.MaxAlt = p.Alt
			}
			if p.Speed > s.MaxSpeed {
				s.MaxSpeed = p.Speed
			}
			if p.G > s.MaxG {
				s.MaxG = p.G
			}
			totalSpeed += p.Speed
			if i > 0 {
				s.DistanceKm += haversineKm(
					pts[i-1].Lat, pts[i-1].Lng, p.Lat, p.Lng)
			}
		}
		s.AvgSpeed = totalSpeed / float64(len(pts))
		s.DurationSec = float64(pts[len(pts)-1].RealTS-pts[0].RealTS) / 1000.0
		out = append(out, s)
	}
	sortStats(out)
	return out
}

// haversineKm returns the great-circle distance between two points, in km.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	sdLat := math.Sin(dLat / 2)
	sdLng := math.Sin(dLng / 2)
	a := sdLat*sdLat + math.Cos(rad(lat1))*math.Cos(rad(lat2))*sdLng*sdLng
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	return 2 * r * math.Asin(math.Sqrt(a))
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }
