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
//
// With ?sinceId=N it becomes incremental: only events with an id strictly
// greater than N are returned, oldest first, together with the next cursor in
// nextSinceId. That is what lets an external consumer mirror the whole history
// without gaps or duplicates. Without sinceId it keeps its original meaning (the
// most recent events, newest first).
func (s *Server) handleHistoryEvents(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}
	if since, ok := sinceParam(r); ok {
		events, err := s.db.EventsSince(since, limitParam(r, 1000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":       len(events),
			"events":      events,
			"nextSinceId": maxEventID(events),
		})
		return
	}
	events, err := s.db.RecentEvents(limitParam(r, 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(events), "events": events})
}

// handleHistoryChat returns persisted chat from the database. Like
// handleHistoryEvents, ?sinceId=N turns it into an incremental feed.
func (s *Server) handleHistoryChat(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"chat": []any{}})
		return
	}
	if since, ok := sinceParam(r); ok {
		chat, err := s.db.ChatSince(since, limitParam(r, 1000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":       len(chat),
			"chat":        chat,
			"nextSinceId": maxChatID(chat),
		})
		return
	}
	chat, err := s.db.RecentChat(limitParam(r, 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(chat), "chat": chat})
}

// handleHistoryMissions returns past missions. Like the other history endpoints,
// ?sinceId=N makes it incremental: only missions with id > N, oldest first, plus
// nextSinceId.
func (s *Server) handleHistoryMissions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"missions": []any{}})
		return
	}
	if since, ok := sinceParam(r); ok {
		missions, err := s.db.MissionsSince(since, limitParam(r, 1000))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":       len(missions),
			"missions":    missions,
			"nextSinceId": maxMissionID(missions),
		})
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

// sinceParam reads ?sinceId=N. ok is false when the parameter is absent, which
// keeps the "recent" behaviour of the history endpoints; a present but
// unparsable value is treated as 0 (the whole history), never as an error.
func sinceParam(r *http.Request) (since int64, ok bool) {
	v := r.URL.Query().Get("sinceId")
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, true
	}
	return n, true
}

// maxEventID returns the largest event id, for the incremental cursor. Zero when
// the batch is empty, telling the caller to keep its previous cursor.
func maxEventID(events []model.Event) int64 {
	var max int64
	for _, e := range events {
		if e.ID > max {
			max = e.ID
		}
	}
	return max
}

// maxChatID returns the largest chat id, for the incremental cursor.
func maxChatID(chat []model.Chat) int64 {
	var max int64
	for _, c := range chat {
		if c.ID > max {
			max = c.ID
		}
	}
	return max
}

// maxMissionID returns the largest mission id, for the incremental cursor.
func maxMissionID(missions []model.Mission) int64 {
	var max int64
	for _, m := range missions {
		if m.ID > max {
			max = m.ID
		}
	}
	return max
}
