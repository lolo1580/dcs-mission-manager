package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"dcsmanager/internal/checklist"
)

// handleChecklists serves the checklist collection and one checklist by uid.
//
// Uids are used in URLs rather than integer ids so that an export can be
// imported on another database, and a later synchronisation has a stable key.
func (s *Server) handleChecklists(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "checklists": []any{}})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/checklists"), "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			s.listChecklists(w, r)
		case http.MethodPost:
			s.saveChecklist(w, r, "")
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
		}
		return
	}
	uid := path
	switch r.Method {
	case http.MethodGet:
		s.getChecklist(w, r, uid)
	case http.MethodPut, http.MethodPost:
		s.saveChecklist(w, r, uid)
	case http.MethodDelete:
		s.deleteChecklist(w, r, uid)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET, PUT or DELETE"})
	}
}

func (s *Server) listChecklists(w http.ResponseWriter, r *http.Request) {
	ownerID, admin := s.documentScope(r)
	list, err := s.db.ChecklistsFor(ownerID, admin, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []checklist.Checklist{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    true,
		"checklists": list,
		"count":      len(list),
	})
}

func (s *Server) getChecklist(w http.ResponseWriter, r *http.Request, uid string) {
	ownerID, admin := s.documentScope(r)
	c, err := s.db.ChecklistFor(uid, ownerID, admin)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "checklist not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) saveChecklist(w http.ResponseWriter, r *http.Request, uid string) {
	ownerID, admin := s.documentScope(r)
	var c checklist.Checklist
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed checklist"})
		return
	}
	if uid != "" {
		c.UID = uid
		existing, err := s.db.ChecklistFor(uid, ownerID, admin)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "checklist not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !canEditDocument(ownerID, admin, existing.OwnerID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the owner can edit this checklist"})
			return
		}
	}
	if err := checklist.Normalize(&c, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	saved, err := s.db.SaveChecklistFor(ownerID, c)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteChecklist(w http.ResponseWriter, r *http.Request, uid string) {
	ownerID, admin := s.documentScope(r)
	existing, err := s.db.ChecklistFor(uid, ownerID, admin)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "checklist not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !canEditDocument(ownerID, admin, existing.OwnerID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the owner can delete this checklist"})
		return
	}
	if err := s.db.DeleteChecklistFor(uid, ownerID, admin); errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "checklist not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": uid})
}
