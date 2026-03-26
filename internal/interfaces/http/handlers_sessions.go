package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type createSessionRequest struct {
	Workspace string `json:"workspace"`
}

func (s Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimitQuery(w, r)
	if !ok {
		return
	}

	response, err := s.services.ListSessions(limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": response,
	})
}

func (s Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	workspace := strings.TrimSpace(req.Workspace)
	if workspace == "" {
		workspace = s.services.Config.Workspace
	}
	if workspace != s.services.Config.Workspace {
		writeError(w, http.StatusBadRequest, "invalid_request", "workspace must match the server workspace")
		return
	}

	session, err := s.services.CreateSession(workspace)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"session": session,
	})
}

func (s Server) handleInspectSession(w http.ResponseWriter, r *http.Request) {
	recentLimit := 20
	if raw := r.URL.Query().Get("recent"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "recent must be a non-negative integer")
			return
		}
		recentLimit = parsed
	}

	response, err := s.services.InspectSession(chi.URLParam(r, "sessionID"), recentLimit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func parseLimitQuery(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 8
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be a non-negative integer")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
