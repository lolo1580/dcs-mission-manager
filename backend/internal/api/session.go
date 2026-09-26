package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleGameEvents returns the recent in-memory game events (oldest first).
func (s *Server) handleGameEvents(w http.ResponseWriter, _ *http.Request) {
	if s.live == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}
	events := s.live.Events()
	writeJSON(w, http.StatusOK, map[string]any{
		"count":  len(events),
		"events": events,
	})
}

// handleChat returns recent messages on GET, and (Phase 2+) accepts a message
// to send into DCS on POST. Sending requires the downstream command channel,
// which is not wired yet, so POST answers 501 with a clear message.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Message == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing message"})
			return
		}
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "sending to DCS not yet available (command channel coming soon)",
		})
		return
	}

	if s.live == nil {
		writeJSON(w, http.StatusOK, map[string]any{"chat": []any{}})
		return
	}
	chat := s.live.Chat()
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(chat),
		"chat":  chat,
	})
}

// handlePlayers returns the connected players.
func (s *Server) handlePlayers(w http.ResponseWriter, _ *http.Request) {
	if s.live == nil {
		writeJSON(w, http.StatusOK, map[string]any{"players": []any{}})
		return
	}
	players := s.live.Players()
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(players),
		"players": players,
	})
}

// handleMission returns the current mission, if any.
func (s *Server) handleMission(w http.ResponseWriter, _ *http.Request) {
	if s.live == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	if m, ok := s.live.Mission(); ok {
		writeJSON(w, http.StatusOK, m)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// handleHistoryEvents returns persisted events from the database.
func (s *Server) handleHistoryEvents(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}
	events, err := s.db.RecentEvents(limitParam(r, 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(events), "events": events})
}

// handleHistoryChat returns persisted chat from the database.
func (s *Server) handleHistoryChat(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"chat": []any{}})
		return
	}
	chat, err := s.db.RecentChat(limitParam(r, 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(chat), "chat": chat})
}

// handleHistoryMissions returns past missions.
func (s *Server) handleHistoryMissions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"missions": []any{}})
		return
	}
	missions, err := s.db.Missions(limitParam(r, 50))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(missions), "missions": missions})
}

func limitParam(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
