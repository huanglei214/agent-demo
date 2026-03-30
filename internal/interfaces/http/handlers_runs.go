package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/service"
)

type startRunRequest struct {
	Instruction string `json:"instruction"`
	Workspace   string `json:"workspace"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	MaxTurns    int    `json:"max_turns"`
	SessionID   string `json:"session_id"`
	Skill       string `json:"skill"`
}

func (s Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseLimitQuery(w, r)
	if !ok {
		return
	}

	response, err := s.services.ListRuns(limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"runs": response,
	})
}

func (s Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	var req startRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if strings.TrimSpace(req.Instruction) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "instruction is required")
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
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = s.services.Config.Model.Provider
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = s.services.Config.Model.Model
	}
	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 20
	}

	response, err := s.services.StartRun(r.Context(), service.RunRequest{
		Instruction: req.Instruction,
		Workspace:   workspace,
		Provider:    provider,
		Model:       model,
		MaxTurns:    maxTurns,
		SessionID:   strings.TrimSpace(req.SessionID),
		Skill:       strings.TrimSpace(req.Skill),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (s Server) handleResumeRun(w http.ResponseWriter, r *http.Request) {
	response, err := s.services.ResumeRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s Server) handleInspectRun(w http.ResponseWriter, r *http.Request) {
	response, err := s.services.InspectRun(chi.URLParam(r, "runID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s Server) handleReplaySummary(w http.ResponseWriter, r *http.Request) {
	entries, err := s.services.ReplayRunSummary(chi.URLParam(r, "runID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
	})
}

func (s Server) handleReplayEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.services.ReplayRun(chi.URLParam(r, "runID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
	})
}

func (s Server) handleRunStream(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	run, err := s.services.LoadRun(runID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming is not supported by this response writer")
		return
	}

	afterSequence := int64(0)
	if raw := r.URL.Query().Get("after"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "after must be a non-negative integer")
			return
		}
		afterSequence = parsed
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = w.Write([]byte("retry: 1000\n\n"))
	flusher.Flush()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastSequence := afterSequence
	for {
		events, err := s.services.ReplayRunAfter(runID, lastSequence)
		if err != nil {
			_ = writeSSEEvent(w, "error", map[string]any{"message": err.Error()})
			flusher.Flush()
			return
		}
		lastSequence, err = streamNewEvents(w, flusher, events, lastSequence)
		if err != nil {
			return
		}

		run, err = s.services.LoadRun(runID)
		if err != nil {
			_ = writeSSEEvent(w, "error", map[string]any{"message": err.Error()})
			flusher.Flush()
			return
		}
		if isTerminalRun(run.Status) {
			_ = writeSSEEvent(w, "done", map[string]any{
				"run_id":   run.ID,
				"status":   run.Status,
				"sequence": lastSequence,
			})
			flusher.Flush()
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func streamNewEvents(w http.ResponseWriter, flusher http.Flusher, events []harnessruntime.Event, afterSequence int64) (int64, error) {
	lastSequence := afterSequence
	for _, event := range events {
		if event.Sequence <= afterSequence {
			continue
		}
		if err := writeSSEEvent(w, "runtime.event", event); err != nil {
			return lastSequence, err
		}
		flusher.Flush()
		lastSequence = event.Sequence
	}
	return lastSequence, nil
}

func writeSSEEvent(w http.ResponseWriter, eventName string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("event: " + eventName + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return err
	}
	return nil
}

func isTerminalRun(status harnessruntime.RunStatus) bool {
	switch status {
	case harnessruntime.RunCompleted, harnessruntime.RunFailed, harnessruntime.RunCancelled, harnessruntime.RunBlocked:
		return true
	default:
		return false
	}
}
