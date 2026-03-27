package app

import (
	"os"
	"sort"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func (s Services) CreateSession(workspace string) (harnessruntime.Session, error) {
	now := time.Now()
	session := harnessruntime.Session{
		ID:        harnessruntime.NewID("session"),
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.StateStore.SaveSession(session); err != nil {
		return harnessruntime.Session{}, err
	}
	return session, nil
}

func (s Services) LoadSession(sessionID string) (harnessruntime.Session, error) {
	return s.StateStore.LoadSession(sessionID)
}

type SessionRunSummary struct {
	RunID         string                   `json:"run_id"`
	Status        harnessruntime.RunStatus `json:"status"`
	CurrentStepID string                   `json:"current_step_id,omitempty"`
	ParentRunID   string                   `json:"parent_run_id,omitempty"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

type SessionInspectResponse struct {
	Session  harnessruntime.Session          `json:"session"`
	Messages []harnessruntime.SessionMessage `json:"messages"`
	Runs     []SessionRunSummary             `json:"runs"`
}

func (s Services) InspectSession(sessionID string, recentLimit int) (SessionInspectResponse, error) {
	session, err := s.LoadSession(sessionID)
	if err != nil {
		return SessionInspectResponse{}, err
	}
	messages, err := s.StateStore.LoadRecentSessionMessages(sessionID, recentLimit)
	if err != nil && !os.IsNotExist(err) {
		return SessionInspectResponse{}, err
	}
	runs, err := s.listSessionRuns(sessionID)
	if err != nil {
		return SessionInspectResponse{}, err
	}
	return SessionInspectResponse{
		Session:  session,
		Messages: ensureSessionMessages(messages),
		Runs:     ensureSessionRunSummaries(runs),
	}, nil
}

func (s Services) listSessionRuns(sessionID string) ([]SessionRunSummary, error) {
	entries, err := os.ReadDir(s.Paths.RunsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionRunSummary{}, nil
		}
		return nil, err
	}

	runs := make([]SessionRunSummary, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		run, err := s.StateStore.LoadRun(entry.Name())
		if err != nil {
			return nil, err
		}
		if run.SessionID != sessionID {
			continue
		}
		runs = append(runs, SessionRunSummary{
			RunID:         run.ID,
			Status:        run.Status,
			CurrentStepID: run.CurrentStepID,
			ParentRunID:   run.ParentRunID,
			CreatedAt:     run.CreatedAt,
			UpdatedAt:     run.UpdatedAt,
		})
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	return runs, nil
}

func ensureSessionMessages(messages []harnessruntime.SessionMessage) []harnessruntime.SessionMessage {
	if messages == nil {
		return []harnessruntime.SessionMessage{}
	}
	return messages
}

func ensureSessionRunSummaries(runs []SessionRunSummary) []SessionRunSummary {
	if runs == nil {
		return []SessionRunSummary{}
	}
	return runs
}
