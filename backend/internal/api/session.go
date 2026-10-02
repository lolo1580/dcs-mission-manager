package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"dcsmanager/internal/model"
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

// handleChat returns recent messages on GET, and accepts a message to send into
// DCS on POST. Sending goes down the same TCP connection the hook uses to
// report, as a {"type":"command","command":"chat",...} line the Lua side
// executes.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Message == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing message"})
			return
		}
		if s.commander == nil || s.commander.Connected() == 0 {
			// Distinguish "DCS is not connected" from a malformed request: the
			// first is a normal state (game not running), not a bug.
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "DCS is not connected (hooks not installed, or game not running)",
			})
			return
		}
		sent := s.commander.SendCommand(model.Command{
			Type:    "command",
			Command: model.CommandChat,
			Message: body.Message,
			From:    "Server",
		})
		if sent == 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "DCS did not accept the message (connection lost)",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sent": sent})
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
