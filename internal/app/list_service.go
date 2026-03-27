package app

import (
	"os"
	"sort"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type SessionListItem struct {
	ID        string    `json:"id"`
	Workspace string    `json:"workspace"`
	RunCount  int       `json:"run_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RunListItem struct {
	ID            string                   `json:"id"`
	SessionID     string                   `json:"session_id"`
	TaskID        string                   `json:"task_id"`
	Status        harnessruntime.RunStatus `json:"status"`
	CurrentStepID string                   `json:"current_step_id,omitempty"`
	Instruction   string                   `json:"instruction,omitempty"`
	Provider      string                   `json:"provider"`
	Model         string                   `json:"model"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

func (s Services) ListSessions(limit int) ([]SessionListItem, error) {
	entries, err := os.ReadDir(s.Paths.SessionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionListItem{}, nil
		}
		return nil, err
	}

	items := make([]SessionListItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		session, err := s.StateStore.LoadSession(entry.Name())
		if err != nil {
			return nil, err
		}
		runs, err := s.listSessionRuns(session.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, SessionListItem{
			ID:        session.ID,
			Workspace: session.Workspace,
			RunCount:  len(runs),
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return clampList(items, limit), nil
}

func (s Services) ListRuns(limit int) ([]RunListItem, error) {
	entries, err := os.ReadDir(s.Paths.RunsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []RunListItem{}, nil
		}
		return nil, err
	}

	items := make([]RunListItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		run, err := s.StateStore.LoadRun(entry.Name())
		if err != nil {
			return nil, err
		}

		instruction := ""
		task, err := s.StateStore.LoadTask(run.TaskID)
		if err == nil {
			instruction = task.Instruction
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		items = append(items, RunListItem{
			ID:            run.ID,
			SessionID:     run.SessionID,
			TaskID:        run.TaskID,
			Status:        run.Status,
			CurrentStepID: run.CurrentStepID,
			Instruction:   instruction,
			Provider:      run.Provider,
			Model:         run.Model,
			CreatedAt:     run.CreatedAt,
			UpdatedAt:     run.UpdatedAt,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return clampList(items, limit), nil
}

func clampList[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
