package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The setup's marker opts an executable into installed mode. A portable copy
// without this marker keeps its existing relative data paths.
const InstalledMarker = "installed-mode"

type UserSettings struct {
	SavedGames string `json:"savedGames,omitempty"`
	ChartsDir  string `json:"chartsDir,omitempty"`
	DCSInstall string `json:"dcsInstall,omitempty"`
}

func UserDataRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "DCS Manager"), nil
}

func installedDefaults(executable, root string) (bool, UserSettings) {
	if !isInstalled(executable) {
		return false, UserSettings{}
	}
	settings, _ := ReadUserSettings(root)
	return true, settings
}

func isInstalled(executable string) bool {
	info, err := os.Stat(filepath.Join(filepath.Dir(executable), InstalledMarker))
	return err == nil && info.Mode().IsRegular()
}

// ReadUserSettings is shared by the installed application and setup wizard.
// Missing settings are normal on first installation; malformed settings are not.
func ReadUserSettings(root string) (UserSettings, error) {
	var settings UserSettings
	data, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return UserSettings{}, fmt.Errorf("configuration existante invalide: %w", err)
	}
	return settings, nil
}

// ApplyUserPaths keeps environment overrides ahead of paths chosen in setup.
func ApplyUserPaths(cfg *Config, settings UserSettings) {
	if os.Getenv("DCSMANAGER_SAVED_GAMES") == "" && settings.SavedGames != "" {
		cfg.SavedGames = settings.SavedGames
	}
	if os.Getenv("DCSMANAGER_DCS_INSTALL") == "" {
		cfg.DCSInstall = settings.DCSInstall
	}
}

func applyInstalledDefaults(cfg *Config, executable, root string) {
	installed, settings := installedDefaults(executable, root)
	if !installed {
		return
	}
	if os.Getenv("DCSMANAGER_DB_PATH") == "" {
		cfg.DBPath = filepath.Join(root, "data", "dcsmanager.db")
	}
	if os.Getenv("DCSMANAGER_CHARTS_DIR") == "" {
		cfg.ChartsDir = settings.ChartsDir
		if cfg.ChartsDir == "" {
			cfg.ChartsDir = filepath.Join(root, "maps_dcs")
		}
	}
	if os.Getenv("DCSMANAGER_CATEGORIES") == "" {
		cfg.CategoriesFile = filepath.Join(root, "categories.json")
	}
	ApplyUserPaths(cfg, settings)
}

func WebViewDataPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	root, err := UserDataRoot()
	if err != nil {
		return ""
	}
	if isInstalled(exe) {
		return filepath.Join(root, "webview")
	}
	return ""
}
