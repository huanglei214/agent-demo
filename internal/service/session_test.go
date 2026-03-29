package service

import (
	"os"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestInspectSessionUsesSessionRunIndexWhenRunArtifactIsMissing(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	services := NewServices(config.Load(workspace))
	now := time.Now().UTC()
	session := harnessruntime.Session{
		ID:        "session_indexed",
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_indexed",
		SessionID: session.ID,
		TaskID:    "task_indexed",
		Status:    harnessruntime.RunCompleted,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := services.StateStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := os.RemoveAll(services.Paths.RunDir(run.ID)); err != nil {
		t.Fatalf("remove top-level run dir: %v", err)
	}

	response, err := services.InspectSession(session.ID, 10)
	if err != nil {
		t.Fatalf("inspect session: %v", err)
	}
	if len(response.Runs) != 1 {
		t.Fatalf("expected one session run, got %#v", response.Runs)
	}
	if response.Runs[0].RunID != run.ID || response.Runs[0].Status != harnessruntime.RunCompleted {
		t.Fatalf("unexpected session run summary: %#v", response.Runs[0])
	}
}
