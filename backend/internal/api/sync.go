package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dcsmanager/internal/auth"
	"dcsmanager/internal/checklist"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/flightplan"
	"dcsmanager/internal/syncer"
)

// syncUpload is the payload a desktop sends to a server. It is deliberately a
// plain JSON document: an operator can inspect or replay it, and the server
// upserts each mission by (owner, client, external id).
type syncUpload struct {
	ClientID string             `json:"clientId"`
	Missions []db.MissionExport `json:"missions"`
}

func (s *Server) handleSyncUpload(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	var payload syncUpload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20))
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed upload"})
		return
	}
	payload.ClientID = strings.TrimSpace(payload.ClientID)
	if payload.ClientID == "" || len(payload.ClientID) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "clientId is required (100 characters maximum)"})
		return
	}
	if len(payload.Missions) > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many missions in one upload (200 maximum)"})
		return
	}

	imported := 0
	for i, mission := range payload.Missions {
		if mission.ExternalID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("mission %d has no externalId", i+1)})
			return
		}
		if _, err := s.auth.ImportMission(user.ID, payload.ClientID, mission); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("mission %d: %v", i+1, err)})
			return
		}
		imported++
	}
	_ = s.auth.Audit(user.ID, "sync_upload", fmt.Sprintf("%s: %d mission(s)", payload.ClientID, imported))
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported})
}

// ---- Desktop side of the optional upload ---------------------------------

func (s *Server) handleSyncSettings(w http.ResponseWriter, r *http.Request) {
	if s.Mode() == "server" {
		http.NotFound(w, r)
		return
	}
	root, err := config.UserDataRoot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	switch r.Method {
	case http.MethodGet:
		_, settings, err := syncer.EnsureSettings(root)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPost, http.MethodPut:
		var req struct {
			ServerURL string `json:"serverUrl"`
			Token     string `json:"token"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		settings, err := syncer.Save(root, req.ServerURL, req.Token)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET or POST"})
	}
}

func (s *Server) handleSyncTest(w http.ResponseWriter, r *http.Request) {
	if s.Mode() == "server" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	root, err := config.UserDataRoot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	user, settings, err := syncer.EnsureSettings(root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user.ServerURL == "" || user.SyncToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sync is not configured"})
		return
	}
	client := syncer.NewClient(user.ServerURL, user.SyncToken, settings.ClientID, nil)
	health, err := client.Ping(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleSyncRun(w http.ResponseWriter, r *http.Request) {
	if s.Mode() == "server" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	database, ok := s.db.(*db.DB)
	if !ok || database == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "persistence is disabled"})
		return
	}
	root, err := config.UserDataRoot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	user, settings, err := syncer.EnsureSettings(root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user.ServerURL == "" || user.SyncToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sync is not configured"})
		return
	}
	client := syncer.NewClient(user.ServerURL, user.SyncToken, settings.ClientID, database)
	started := time.Now().UnixMilli()
	result, err := client.Upload(r.Context())
	if err != nil {
		_ = syncer.RecordError(root, "error: "+err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	documents, cursor, err := client.SyncDocuments(r.Context(), database, settings.SyncCursor)
	if err != nil {
		_ = syncer.RecordError(root, "documents error: "+err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = syncer.SaveCursor(root, cursor)
	detail := fmt.Sprintf("%d mission(s), %d plan(s), %d checklist(s)",
		result.Imported, documents.Plans, documents.Checklists)
	if documents.Conflicts > 0 {
		detail += fmt.Sprintf(", %d conflict(s)", documents.Conflicts)
	}
	_ = syncer.RecordSync(root, result.Imported, detail, started)
	writeJSON(w, http.StatusOK, map[string]any{
		"imported":   result.Imported,
		"plans":      documents.Plans,
		"checklists": documents.Checklists,
		"conflicts":  documents.Conflicts,
	})
}

// ---- Bidirectional plans and checklists ----------------------------------

type documentPush struct {
	ClientID   string                `json:"clientId"`
	Plans      []flightplan.Plan     `json:"plans"`
	Checklists []checklist.Checklist `json:"checklists"`
}

func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	admin := user.Role == auth.RoleAdmin

	var payload documentPush
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed document push"})
		return
	}
	if len(payload.Plans) > 500 || len(payload.Checklists) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many documents (500 maximum each)"})
		return
	}

	plans := make([]flightplan.Plan, 0, len(payload.Plans))
	conflicts := 0
	for i := range payload.Plans {
		plan := payload.Plans[i]
		if err := validateFlightPlan(&plan, true); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("plan %d: %v", i+1, err)})
			return
		}
		saved, applied, err := s.auth.UpsertFlightPlanFromSync(user.ID, admin, plan)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("plan %d: %v", i+1, err)})
			return
		}
		if !applied && (saved.UpdatedAt > plan.UpdatedAt || (saved.OwnerID != 0 && saved.OwnerID != user.ID && !admin)) {
			conflicts++
		}
		plans = append(plans, saved)
	}

	checklists := make([]checklist.Checklist, 0, len(payload.Checklists))
	for i := range payload.Checklists {
		c := payload.Checklists[i]
		if err := checklist.Normalize(&c, true); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("checklist %d: %v", i+1, err)})
			return
		}
		saved, applied, err := s.auth.UpsertChecklistFromSync(user.ID, admin, c)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("checklist %d: %v", i+1, err)})
			return
		}
		if !applied && (saved.UpdatedAt > c.UpdatedAt || (saved.OwnerID != 0 && saved.OwnerID != user.ID && !admin)) {
			conflicts++
		}
		checklists = append(checklists, saved)
	}

	_ = s.auth.Audit(user.ID, "sync_documents", fmt.Sprintf("%s: %d plan(s), %d checklist(s), %d conflict(s)",
		payload.ClientID, len(plans), len(checklists), conflicts))
	cursor, _ := s.auth.SyncCursor()
	writeJSON(w, http.StatusOK, map[string]any{
		"plans":      plans,
		"checklists": checklists,
		"conflicts":  conflicts,
		"cursor":     cursor,
	})
}

func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	admin := user.Role == auth.RoleAdmin
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	plans, err := s.auth.SyncFlightPlans(since, user.ID, admin, 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	checklists, err := s.auth.SyncChecklists(since, user.ID, admin, 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cursor, err := s.auth.SyncCursor()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if plans == nil {
		plans = []flightplan.Plan{}
	}
	if checklists == nil {
		checklists = []checklist.Checklist{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plans":      plans,
		"checklists": checklists,
		"cursor":     cursor,
	})
}

func (s *Server) handleSyncConflicts(w http.ResponseWriter, r *http.Request) {
	if s.Mode() != "server" || s.auth == nil {
		http.NotFound(w, r)
		return
	}
	user, ok := currentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	admin := user.Role == auth.RoleAdmin
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sync/conflicts"), "/")
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "preview" {
			if _, ok := s.requireAdmin(w, r); !ok {
				return
			}
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
				return
			}
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || id <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid conflict id"})
				return
			}
			conflict, err := s.auth.Conflict(id)
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "conflict not found"})
				return
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			switch conflict.Kind {
			case "flightplan":
				winner, err := s.auth.FlightPlanByUID(conflict.UID)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "current plan not found"})
					return
				}
				var loser flightplan.Plan
				if err := json.Unmarshal([]byte(conflict.Payload), &loser); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plan payload"})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"kind": "flightplan", "uid": conflict.UID, "winner": winner, "loser": loser,
					"fields": planMergeFields,
				})
			case "checklist":
				winner, err := s.auth.ChecklistByUID(conflict.UID)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "current checklist not found"})
					return
				}
				var loser checklist.Checklist
				if err := json.Unmarshal([]byte(conflict.Payload), &loser); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid checklist payload"})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"kind": "checklist", "uid": conflict.UID, "winner": winner, "loser": loser,
					"fields": checklistMergeFields,
				})
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown conflict kind"})
			}
			return
		}
		if len(parts) == 2 && parts[1] == "merge" {
			if _, ok := s.requireAdmin(w, r); !ok {
				return
			}
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
				return
			}
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || id <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid conflict id"})
				return
			}
			conflict, err := s.auth.Conflict(id)
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "conflict not found"})
				return
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			var req struct {
				Choices map[string]string `json:"choices"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed merge request"})
				return
			}
			now := time.Now().UnixMilli()
			switch conflict.Kind {
			case "flightplan":
				winner, err := s.auth.FlightPlanByUID(conflict.UID)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "current plan not found"})
					return
				}
				var loser flightplan.Plan
				if err := json.Unmarshal([]byte(conflict.Payload), &loser); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plan payload"})
					return
				}
				merged, err := mergePlan(winner, loser, req.Choices)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				merged.UpdatedAt = now
				if err := validateFlightPlan(&merged, true); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				saved, _, err := s.auth.UpsertFlightPlanFromSync(user.ID, true, merged)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				_ = s.auth.DeleteConflict(id)
				_ = s.auth.Audit(user.ID, "conflict_merged", conflict.Kind+" "+conflict.UID)
				writeJSON(w, http.StatusOK, map[string]any{"merged": saved})
				return
			case "checklist":
				winner, err := s.auth.ChecklistByUID(conflict.UID)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "current checklist not found"})
					return
				}
				var loser checklist.Checklist
				if err := json.Unmarshal([]byte(conflict.Payload), &loser); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid checklist payload"})
					return
				}
				merged, err := mergeChecklist(winner, loser, req.Choices)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				merged.UpdatedAt = now
				if err := checklist.Normalize(&merged, true); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				saved, _, err := s.auth.UpsertChecklistFromSync(user.ID, true, merged)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				_ = s.auth.DeleteConflict(id)
				_ = s.auth.Audit(user.ID, "conflict_merged", conflict.Kind+" "+conflict.UID)
				writeJSON(w, http.StatusOK, map[string]any{"merged": saved})
				return
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown conflict kind"})
				return
			}
		}
		if len(parts) == 2 && parts[1] == "restore" {
			if _, ok := s.requireAdmin(w, r); !ok {
				return
			}
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
				return
			}
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || id <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid conflict id"})
				return
			}
			conflict, err := s.auth.Conflict(id)
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "conflict not found"})
				return
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			now := time.Now().UnixMilli()
			switch conflict.Kind {
			case "flightplan":
				var plan flightplan.Plan
				if err := json.Unmarshal([]byte(conflict.Payload), &plan); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plan payload"})
					return
				}
				plan.ID, plan.Revision, plan.SyncRevision, plan.DeletedAt = 0, 0, 0, 0
				plan.UpdatedAt = now
				saved, _, err := s.auth.UpsertFlightPlanFromSync(user.ID, true, plan)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				_ = s.auth.DeleteConflict(id)
				_ = s.auth.Audit(user.ID, "conflict_restored", conflict.Kind+" "+conflict.UID)
				writeJSON(w, http.StatusOK, map[string]any{"restored": saved})
				return
			case "checklist":
				var c checklist.Checklist
				if err := json.Unmarshal([]byte(conflict.Payload), &c); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid checklist payload"})
					return
				}
				c.ID, c.Revision, c.SyncRevision, c.DeletedAt = 0, 0, 0, 0
				c.UpdatedAt = now
				if err := checklist.Normalize(&c, true); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				saved, _, err := s.auth.UpsertChecklistFromSync(user.ID, true, c)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				_ = s.auth.DeleteConflict(id)
				_ = s.auth.Audit(user.ID, "conflict_restored", conflict.Kind+" "+conflict.UID)
				writeJSON(w, http.StatusOK, map[string]any{"restored": saved})
				return
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown conflict kind"})
				return
			}
		}
		if len(parts) != 1 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown conflict resource"})
			return
		}
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use DELETE"})
			return
		}
		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid conflict id"})
			return
		}
		if err := s.auth.DeleteConflict(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "conflict not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use GET"})
		return
	}
	list, err := s.auth.Conflicts(user.ID, admin, 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []db.ConflictCopy{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": list})
}

var planMergeFields = []string{
	"name", "aircraft", "departure", "arrival", "cruiseAltFt", "cruiseTasKt",
	"fuelBurnPph", "reserveMin", "windFromDeg", "windKt", "flightRules",
	"visibility", "notes", "route",
}

var checklistMergeFields = []string{
	"name", "aircraft", "phase", "description", "visibility", "items",
}

// mergePlan copies the chosen fields from the losing version into the winning
// one. Only the fields in planMergeFields are eligible; identity and
// synchronisation metadata always come from the winner.
func mergePlan(winner, loser flightplan.Plan, choices map[string]string) (flightplan.Plan, error) {
	winnerMap, err := documentMap(winner)
	if err != nil {
		return winner, err
	}
	loserMap, err := documentMap(loser)
	if err != nil {
		return winner, err
	}
	for _, field := range planMergeFields {
		if choices[field] == "loser" {
			winnerMap[field] = loserMap[field]
		}
	}
	data, err := json.Marshal(winnerMap)
	if err != nil {
		return winner, err
	}
	var merged flightplan.Plan
	if err := json.Unmarshal(data, &merged); err != nil {
		return winner, err
	}
	merged.ID, merged.Revision, merged.SyncRevision, merged.DeletedAt = 0, 0, 0, 0
	return merged, nil
}

// mergeChecklist is the checklist counterpart.
func mergeChecklist(winner, loser checklist.Checklist, choices map[string]string) (checklist.Checklist, error) {
	winnerMap, err := documentMap(winner)
	if err != nil {
		return winner, err
	}
	loserMap, err := documentMap(loser)
	if err != nil {
		return winner, err
	}
	for _, field := range checklistMergeFields {
		if choices[field] == "loser" {
			winnerMap[field] = loserMap[field]
		}
	}
	data, err := json.Marshal(winnerMap)
	if err != nil {
		return winner, err
	}
	var merged checklist.Checklist
	if err := json.Unmarshal(data, &merged); err != nil {
		return winner, err
	}
	merged.ID, merged.Revision, merged.SyncRevision, merged.DeletedAt = 0, 0, 0, 0
	return merged, nil
}

func documentMap(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
