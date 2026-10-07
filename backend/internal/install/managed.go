package install

import (
	"errors"
	"os"
	"path/filepath"
)

// InstallPreservingConfig updates the bundled scripts without resetting user
// network/sampling settings. A missing configuration is created normally.
func (in *Installer) InstallPreservingConfig() ([]Result, error) {
	targets := DefaultTargets()
	configRel := filepath.Join("Config", "dcsmanager.cfg")
	info, err := os.Stat(filepath.Join(in.SavedGames, configRel))
	if errors.Is(err, os.ErrNotExist) {
		return in.Install(targets)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("configuration DCS Manager invalide")
	}
	filtered := make([]Target, 0, len(targets)-1)
	for _, target := range targets {
		if target.DestRel != configRel {
			filtered = append(filtered, target)
		}
	}
	results, err := in.Install(filtered)
	results = append(results, Result{DestRel: configRel, Action: "unchanged", Note: "configuration utilisateur conservée"})
	return results, err
}
