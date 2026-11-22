package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"dcsmanager/internal/content"
)

// handleContentStatus reports the local content roots and the DCS process state.
func (s *Server) handleContentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	status, err := s.content.Status()
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// handleContentMods lists enabled and disabled mods.
func (s *Server) handleContentMods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	scan, err := s.content.Mods()
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

// handleContentLiveries lists user liveries and liveries bundled in mods.
func (s *Server) handleContentLiveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	scan, err := s.content.Liveries()
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

// handleContentQuarantine lists removed mods and liveries.
func (s *Server) handleContentQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	items, err := s.content.Quarantine()
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// handleContentModAction enables, disables, quarantines, restores or reveals a mod.
func (s *Server) handleContentModAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	var req content.ModActionRequest
	if err := decodeContentBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corps invalide: " + err.Error()})
		return
	}
	result, err := s.content.ActionMod(req)
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleContentLiveryAction quarantines, restores, classifies or reveals a livery.
func (s *Server) handleContentLiveryAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	var req content.LiveryActionRequest
	if err := decodeContentBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corps invalide: " + err.Error()})
		return
	}
	result, err := s.content.ActionLivery(req)
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleContentImportInspect stores an uploaded archive and returns its preview.
func (s *Server) handleContentImportInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<30)
	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "envoi multipart attendu"})
		return
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeContentError(w, err)
			return
		}
		if part.FormName() != "file" && part.FileName() == "" {
			_ = part.Close()
			continue
		}
		preview, err := s.content.InspectUpload(part, part.FileName())
		_ = part.Close()
		if err != nil {
			writeContentError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aucun fichier envoyé"})
}

// handleContentImportCommit installs the selected preview items.
func (s *Server) handleContentImportCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	var req struct {
		ID       string                    `json:"id"`
		Selected []content.ImportSelection `json:"selected"`
		Replace  bool                      `json:"replace"`
	}
	if err := decodeContentBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corps invalide: " + err.Error()})
		return
	}
	result, err := s.content.CommitImport(req.ID, req.Selected, req.Replace)
	if err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleContentImportCancel removes an inspected upload.
func (s *Server) handleContentImportCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/content/import/")
	id = path.Base(strings.TrimSpace(id))
	if id == "" || id == "." || id == "/" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant manquant"})
		return
	}
	if err := s.content.CancelImport(id); err != nil {
		writeContentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleContentThumbnail serves a livery preview image when one exists.
func (s *Server) handleContentThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	if s.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gestionnaire de contenu indisponible"})
		return
	}
	path, err := s.content.ThumbnailPath(r.URL.Query().Get("id"))
	if err != nil {
		writeContentError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, path)
}

func decodeContentBody(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(dst)
}

func writeContentError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, content.ErrDCSRunning), errors.Is(err, content.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, content.ErrNoSavedGames):
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
