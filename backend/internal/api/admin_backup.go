package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// handleAdminBackup streams a zip containing a consistent copy of the server
// database. It is the server's own backup: the desktop backup endpoint is
// hidden in this profile.
func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	dir, err := os.MkdirTemp("", "dcsmanager-backup-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "dcsmanager.db")
	if err := s.auth.BackupTo(dbPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	zipPath := filepath.Join(dir, "backup.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("dcsmanager.db")
	if err == nil {
		var source *os.File
		source, err = os.Open(dbPath)
		if err == nil {
			_, err = io.Copy(entry, source)
			source.Close()
		}
	}
	readme, readmeErr := writer.Create("README.txt")
	if readmeErr == nil {
		_, readmeErr = fmt.Fprintf(readme,
			"DCS Manager server backup\nCreated: %s\nVersion: %s\n\n"+
				"Stop the container, replace data/dcsmanager.db with this file, then start it again.\n",
			time.Now().Format(time.RFC3339), s.version)
	}
	closeErr := writer.Close()
	archive.Close()
	if err != nil || readmeErr != nil || closeErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create the backup archive"})
		return
	}
	_ = s.auth.Audit(user.ID, "server_backup", filepath.Base(zipPath))

	name := fmt.Sprintf("dcsmanager-server-%s.zip", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, zipPath)
}
