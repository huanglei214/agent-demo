package app

import (
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
