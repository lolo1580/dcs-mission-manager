package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dcsmanager/internal/backup"
)

// backupDir is where the manager keeps profile archives: beside the database,
// under backups/, so the manager's state stays in one folder. It mirrors the
// CLI's own location.
func (s *Server) backupDir() string {
	dir := filepath.Dir(s.cfg.DBPath)
	if dir == "" || dir == "." {
		return filepath.Join("data", "backups")
	}
	return filepath.Join(dir, "backups")
}

// handleBackupCategories lists the categories a backup can include.
//
//	GET /api/backup/categories
func (s *Server) handleBackupCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": backup.Categories,
		"default":    backup.DefaultCategories(),
		"savedGames": s.cfg.SavedGames,
		"directory":  s.backupDir(),
	})
}

// handleBackup is the list/create endpoint.
//
//	GET  /api/backup   the stored archives
//	POST /api/backup   create one ({categories: [...]})
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := backup.List(s.backupDir())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"backups":    list,
			"directory":  s.backupDir(),
			"savedGames": s.cfg.SavedGames,
		})
	case http.MethodPost:
		s.createBackup(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

type createBackupRequest struct {
	Categories []string `json:"categories"`
}

func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	var req createBackupRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	if s.cfg.SavedGames == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "DCS Saved Games folder not found; set DCSMANAGER_SAVED_GAMES",
		})
		return
	}

	a, err := backup.Create(s.cfg.SavedGames, s.backupDir(), req.Categories, "")
	if err != nil {
		// An unknown category is the caller's mistake, not a server fault.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup": a})
}

type restoreBackupRequest struct {
	Name   string `json:"name"`
	DryRun bool   `json:"dryRun"`
}

// handleBackupRestore restores a stored archive back into Saved Games.
//
//	POST /api/backup/restore   {name, dryRun}
//
// The name comes from the client, so it is resolved under the backup directory
// and anything that escapes it is refused: a "../" must not let a caller restore
// an arbitrary file on the machine.
func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	var req restoreBackupRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	path, err := safeJoin(s.backupDir(), req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.cfg.SavedGames == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "DCS Saved Games folder not found; set DCSMANAGER_SAVED_GAMES",
		})
		return
	}

	res, err := backup.Restore(path, s.cfg.SavedGames, req.DryRun)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": res})
}

// handleBackupDownload streams one stored archive as a file.
//
//	GET /api/backup/download/{name}
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/backup/download/")
	path, err := safeJoin(s.backupDir(), name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Lstat, not Stat: a symlink planted in the backup directory must not be
	// followed and served.
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "archive not found"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

// safeJoin resolves name under dir and refuses a result that escapes it.
func safeJoin(dir, name string) (string, error) {
	if name == "" {
		return "", errBackupName
	}
	clean := filepath.Clean(filepath.Join(dir, filepath.FromSlash(name)))
	base := filepath.Clean(dir)
	if clean == base || !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		return "", errBackupName
	}
	return clean, nil
}

// decodeJSONBody decodes a small JSON body, capping its size.
func decodeJSONBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(dst)
}

var errBackupName = errors.New("invalid archive name")
