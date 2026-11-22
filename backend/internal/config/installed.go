package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The setup's marker opts an executable into installed mode. A portable copy
// without this marker keeps its existing relative data paths.
const InstalledMarker = "installed-mode"

// AppName is the product name used for the per-user data folder and the UI.
const AppName = "DCS Manager"

// LegacyAppName is the short-lived pre-release folder used while the project
// was briefly called Flight Sim Manager. Its data is copied once into the
// current folder and the original is left untouched.
const LegacyAppName = "Flight Sim Manager"

// DatabaseFile is the SQLite file name inside the data folder.
const DatabaseFile = "dcsmanager.db"

// LegacyDatabaseFile is the name written by the pre-release Flight Sim
// Manager build.
const LegacyDatabaseFile = "flightsimmanager.db"

type UserSettings struct {
	SavedGames string `json:"savedGames,omitempty"`
	ChartsDir  string `json:"chartsDir,omitempty"`
	DCSInstall string `json:"dcsInstall,omitempty"`
	// ServerURL, SyncToken and ClientID configure the optional upload to a
	// DCS Manager server. The token is a personal API token created on that
	// server; it is kept in this file, which is readable only by the user.
	ServerURL  string `json:"serverUrl,omitempty"`
	SyncToken  string `json:"syncToken,omitempty"`
	ClientID   string `json:"clientId,omitempty"`
	LastSync   int64  `json:"lastSync,omitempty"`
	LastResult string `json:"lastResult,omitempty"`
	SyncCursor int64  `json:"syncCursor,omitempty"`
}

func UserDataRoot() (string, error) {
	if override := os.Getenv("DCSMANAGER_DATA_ROOT"); override != "" {
		return filepath.Abs(override)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName), nil
}

// LegacyUserDataRoot is where versions up to 1.0 stored their data.
func LegacyUserDataRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, LegacyAppName), nil
}

// PrepareUserData copies a pre-release Flight Sim Manager data set into the
// current per-user folder. It runs before the configuration is loaded, so the
// database path is ready on first launch. It is a no-op for portable builds and
// when a data set already exists (the normal case for users coming from 1.0).
//
// The old folder is never modified: if anything goes wrong the user can still
// open it, and the copy is staged before being moved into place.
func PrepareUserData() error {
	executable, err := os.Executable()
	if err != nil {
		return nil
	}
	if !isInstalled(executable) {
		return nil
	}
	root, err := UserDataRoot()
	if err != nil {
		return err
	}
	legacy, err := LegacyUserDataRoot()
	if err != nil || filepath.Clean(legacy) == filepath.Clean(root) {
		return nil
	}
	return migrateLegacyData(legacy, root)
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

// WriteUserSettings replaces settings.json atomically, so a crash cannot leave a
// truncated file that would refuse to load on the next start.
func WriteUserSettings(root string, settings UserSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(root, "settings-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(root, "settings.json"))
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
		cfg.DBPath = filepath.Join(root, "data", DatabaseFile)
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

// migrateLegacyData copies the old data folder and settings into root. Any
// non-empty data folder already present means a previous run (or a fresh setup)
// owns the destination, and the migration is skipped.
func migrateLegacyData(legacy, root string) error {
	legacyData := filepath.Join(legacy, "data")
	if info, err := os.Stat(legacyData); err != nil || !info.IsDir() {
		return nil
	}
	if entries, err := os.ReadDir(filepath.Join(root, "data")); err == nil && hasDatabase(entries) {
		return nil
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	stage, err := os.MkdirTemp(root, "migration-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	stagedData := filepath.Join(stage, "data")
	if err := copyTree(legacyData, stagedData); err != nil {
		return fmt.Errorf("copie des données Flight Sim Manager: %w", err)
	}
	if err := renameDatabaseFiles(stagedData); err != nil {
		return err
	}

	// Settings and categories are merged first; moving the data folder is the
	// commit marker, so a failed run is simply retried on the next launch.
	legacySettings, err := ReadUserSettings(legacy)
	if err != nil {
		return err
	}
	current, err := ReadUserSettings(root)
	if err != nil {
		return err
	}
	merged := current
	if merged.SavedGames == "" {
		merged.SavedGames = legacySettings.SavedGames
	}
	if merged.DCSInstall == "" {
		merged.DCSInstall = legacySettings.DCSInstall
	}
	if merged.ChartsDir == "" {
		merged.ChartsDir = legacySettings.ChartsDir
	}
	if merged != (UserSettings{}) {
		if err := WriteUserSettings(root, merged); err != nil {
			return err
		}
	}
	if err := copyFileIfMissing(filepath.Join(legacy, "categories.json"), filepath.Join(root, "categories.json")); err != nil {
		return err
	}

	// Merge into the data folder rather than renaming the whole staged tree: a
	// log file written before the first launch may already be there, and it must
	// neither block the migration nor be overwritten.
	destination := filepath.Join(root, "data")
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	if err := mergeTree(stagedData, destination); err != nil {
		return err
	}

	note := fmt.Sprintf("Données copiées depuis %s le %s.\nLe dossier d'origine est conservé.\n",
		legacy, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(filepath.Join(root, "migrated-from-flight-sim-manager.txt"), []byte(note), 0644)
	return nil
}

// hasDatabase reports whether a data folder already holds a SQLite database.
// The log file may exist before the first launch writes the database, so its
// presence alone must not suppress the migration.
func hasDatabase(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") {
			return true
		}
	}
	return false
}

// renameDatabaseFiles gives the copied SQLite database its new name. The write
// ahead log and shared-memory file must be renamed with it: leaving them under
// the old name would detach uncommitted pages from the database.
func renameDatabaseFiles(dir string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		old := filepath.Join(dir, LegacyDatabaseFile+suffix)
		if _, err := os.Stat(old); err != nil {
			continue
		}
		target := filepath.Join(dir, DatabaseFile+suffix)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if err := os.Rename(old, target); err != nil {
			return fmt.Errorf("renommage de la base: %w", err)
		}
	}
	return nil
}

// mergeTree moves the staged files into an existing data folder, keeping any
// file that is already there. Nothing is overwritten: the migration only fills
// gaps, so a log written before the first launch survives.
func mergeTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		return os.Rename(path, target)
	})
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		// Panel-era artifacts are deliberately not carried over: the feature was
		// removed in 2.0 and these files would only be dead weight.
		if entry.IsDir() && relative == "profiles" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && (entry.Name() == "mappings.json" || entry.Name() == "panel-settings.json") {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("migration refusée: lien symbolique dans les données")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("migration refusée: fichier spécial")
		}
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()
		targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(targetFile, sourceFile)
		closeErr := targetFile.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func copyFileIfMissing(source, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return nil
	}
	data, err := os.ReadFile(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0644)
}
