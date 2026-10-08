// Package userinstall prepares persistent per-user data for the Windows setup.
package userinstall

import (
	"dcsmanager/internal/config"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Configure never removes portable files or overwrites an existing data set.
// Portable applications must be closed before calling this (checked by the CLI).
func Configure(root, savedGames, portable, dcsInstall string) (string, error) {
	if savedGames != "" {
		if info, err := os.Stat(savedGames); err != nil || !info.IsDir() {
			return "", errors.New("dossier Saved Games DCS introuvable")
		}
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	if dcsInstall != "" {
		if _, err := os.Stat(filepath.Join(dcsInstall, "bin", "DCS.exe")); err != nil {
			if _, err := os.Stat(filepath.Join(dcsInstall, "bin-mt", "DCS.exe")); err != nil {
				return "", errors.New("installation DCS invalide: DCS.exe absent")
			}
		}
	}
	path := filepath.Join(root, "settings.json")
	settings, err := config.ReadUserSettings(root)
	if err != nil {
		return "", err
	}
	if savedGames != "" {
		settings.SavedGames = savedGames
	}
	if dcsInstall != "" {
		settings.DCSInstall = dcsInstall
	}
	note := "Données conservées dans " + root
	if portable != "" {
		migrated, err := migrate(filepath.Join(portable, "data"), filepath.Join(root, "data"))
		if err != nil {
			return "", err
		}
		if migrated {
			note = "Données portables copiées ; les fichiers d’origine sont conservés."
		} else {
			note = "Données installées déjà présentes : aucune donnée portable écrasée."
		}
		charts := filepath.Join(portable, "maps_dcs")
		if settings.ChartsDir == "" {
			if info, err := os.Stat(charts); err == nil && info.IsDir() {
				settings.ChartsDir = charts
			}
		}
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(root, "settings-*.tmp")
	if err != nil {
		return "", err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, path); err != nil {
		return "", err
	}
	return note, nil
}

func migrate(source, destination string) (bool, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return false, fmt.Errorf("données portables: %w", err)
	}
	if len(entries) == 0 {
		return false, errors.New("dossier de données portables vide")
	}
	existing, err := os.ReadDir(destination)
	if err == nil && len(existing) > 0 {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	root := filepath.Dir(destination)
	stage, err := os.MkdirTemp(root, "migration-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(stage)
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("migration refusée: lien symbolique dans les données")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(stage, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("migration refusée: fichier spécial")
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return false, err
	}
	if len(existing) == 0 && err == nil { // Only an empty destination can be removed.
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	if err := os.Rename(stage, destination); err != nil {
		return false, err
	}
	return true, nil
}
