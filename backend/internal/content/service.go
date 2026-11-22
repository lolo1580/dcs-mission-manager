// Package content manages the user's DCS mods and liveries: scanning them,
// enabling/disabling mods, quarantining and restoring files, and importing
// archives. It never writes outside the DCS Saved Games folder.
package content

import (
	"errors"
	"os"
	"sync"
	"time"

	"dcsmanager/internal/dcsdata"
	"dcsmanager/internal/install"
)

const cacheTTL = 15 * time.Second

// Status describes the local content roots and whether DCS is running.
type Status struct {
	SavedGames       string `json:"savedGames,omitempty"`
	ModsDir          string `json:"modsDir,omitempty"`
	DisabledDir      string `json:"disabledDir,omitempty"`
	LiveriesDir      string `json:"liveriesDir,omitempty"`
	ModsQuarantine   string `json:"modsQuarantineDir,omitempty"`
	LiveryQuarantine string `json:"liveriesQuarantineDir,omitempty"`
	DCSRunning       bool   `json:"dcsRunning"`
	DCSKnown         bool   `json:"dcsKnown"`
}

// Service owns the read caches and serialises every content mutation.
type Service struct {
	savedGames string
	dataDir    string

	mu         sync.Mutex
	dcsRunning func() (bool, error)
	now        func() time.Time

	modsCache     []dcsdata.InstalledMod
	modsCacheAt   time.Time
	liveryCache   dcsdata.LiveryScan
	liveryCacheAt time.Time
}

// New creates a content service. dataDir is where import staging lives; it is
// normally the manager's data folder beside the SQLite database.
func New(savedGames, dataDir string) *Service {
	if dataDir == "" {
		dataDir = "data"
	}
	return &Service{
		savedGames: savedGames,
		dataDir:    dataDir,
		dcsRunning: install.DCSRunning,
		now:        time.Now,
	}
}

// SetDCSRunning overrides the process check for tests.
func (s *Service) SetDCSRunning(fn func() (bool, error)) {
	if fn != nil {
		s.dcsRunning = fn
	}
}

// SavedGames returns the root this service manages.
func (s *Service) SavedGames() string { return s.savedGames }

// Invalidate drops both scan caches. It must be called after any mutation.
func (s *Service) Invalidate() {
	s.modsCacheAt = time.Time{}
	s.liveryCacheAt = time.Time{}
}

// Status reports the roots and the DCS process state.
func (s *Service) Status() (Status, error) {
	st := Status{
		SavedGames:       s.savedGames,
		ModsDir:          dcsdata.ModsDir(s.savedGames),
		DisabledDir:      dcsdata.DisabledModsDir(s.savedGames),
		LiveriesDir:      dcsdata.LiveriesDir(s.savedGames),
		ModsQuarantine:   dcsdata.ModsQuarantineDir(s.savedGames),
		LiveryQuarantine: dcsdata.LiveriesQuarantineDir(s.savedGames),
	}
	if s.savedGames == "" {
		return st, nil
	}
	if s.dcsRunning != nil {
		running, err := s.dcsRunning()
		st.DCSKnown = true
		st.DCSRunning = running
		if err != nil {
			return st, err
		}
	}
	return st, nil
}

// Mods returns the cached mod scan.
func (s *Service) Mods() (dcsdata.ModsScan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.modsLocked()
}

func (s *Service) modsLocked() (dcsdata.ModsScan, error) {
	if len(s.modsCache) > 0 && s.now().Sub(s.modsCacheAt) < cacheTTL {
		return scanFromMods(s.modsCache, s.savedGames), nil
	}
	scan, err := dcsdata.ScanMods(s.savedGames)
	if err != nil {
		return scan, err
	}
	s.modsCache = scan.Mods
	s.modsCacheAt = s.now()
	return scan, nil
}

// Liveries returns the cached livery scan.
func (s *Service) Liveries() (dcsdata.LiveryScan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.liveriesLocked()
}

func (s *Service) liveriesLocked() (dcsdata.LiveryScan, error) {
	if len(s.liveryCache.Liveries) > 0 && s.now().Sub(s.liveryCacheAt) < cacheTTL {
		return s.liveryCache, nil
	}
	scan, err := dcsdata.ScanLiveries(s.savedGames)
	if err != nil {
		return scan, err
	}
	s.liveryCache = scan
	s.liveryCacheAt = s.now()
	return scan, nil
}

func scanFromMods(mods []dcsdata.InstalledMod, savedGames string) dcsdata.ModsScan {
	scan := dcsdata.ModsScan{
		SavedGames:  savedGames,
		ModsDir:     dcsdata.ModsDir(savedGames),
		DisabledDir: dcsdata.DisabledModsDir(savedGames),
		Mods:        mods,
	}
	for _, mod := range mods {
		scan.TotalBytes += mod.SizeBytes
		if mod.Enabled {
			scan.Active++
		} else {
			scan.Disabled++
		}
	}
	return scan
}

// Livery returns one livery by its opaque ID, using the cached scan.
func (s *Service) Livery(id string) (dcsdata.Livery, bool, error) {
	scan, err := s.Liveries()
	if err != nil {
		return dcsdata.Livery{}, false, err
	}
	for _, livery := range scan.Liveries {
		if livery.ID == id {
			return livery, true, nil
		}
	}
	return dcsdata.Livery{}, false, nil
}

// requireDCSClosed refuses a mutation while DCS has the files open.
func (s *Service) requireDCSClosed() error {
	if s.dcsRunning == nil {
		return nil
	}
	running, err := s.dcsRunning()
	if err != nil {
		return err
	}
	if running {
		return ErrDCSRunning
	}
	return nil
}

var (
	// ErrDCSRunning is returned when a mutation is attempted while DCS is open.
	ErrDCSRunning = errors.New("DCS est en cours d’exécution")
	// ErrNoSavedGames is returned when the DCS Saved Games folder is missing.
	ErrNoSavedGames = errors.New("dossier Saved Games DCS introuvable")
	// ErrConflict is returned when an import target already exists.
	ErrConflict = errors.New("conflit")
)

// ensureSavedGames checks the root before any write.
func (s *Service) ensureSavedGames() error {
	if s.savedGames == "" {
		return ErrNoSavedGames
	}
	info, err := os.Stat(s.savedGames)
	if err != nil || !info.IsDir() {
		return ErrNoSavedGames
	}
	return nil
}
