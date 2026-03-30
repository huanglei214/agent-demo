package service

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestLoadRunStatePreservesPlanModeAndTodos(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	services := NewServices(config.Load(workspace))
	now := time.Now().UTC()
	session := harnessruntime.Session{
		ID:        "session_todo",
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_todo",
		SessionID: session.ID,
		TaskID:    "task_todo",
		Status:    harnessruntime.RunPending,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  1,
		PlanMode:  harnessruntime.PlanModeTodo,
		CreatedAt: now,
		UpdatedAt: now,
	}
	state := harnessruntime.RunState{
		RunID:     run.ID,
		TurnCount: 1,
		Todos: []harnessruntime.TodoItem{
			{
				ID:        "todo_1",
				Content:   "inspect README",
				Status:    harnessruntime.TodoPending,
				Priority:  7,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	if err := services.StateStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loadedRun, err := services.LoadRun(run.ID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if loadedRun.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected plan mode %q, got %#v", harnessruntime.PlanModeTodo, loadedRun)
	}

	gotRun, gotState, err := services.LoadRunState(run.ID)
	if err != nil {
		t.Fatalf("load run state: %v", err)
	}
	if gotRun.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected loaded run plan mode %q, got %#v", harnessruntime.PlanModeTodo, gotRun)
	}
	if len(gotState.Todos) != 1 {
		t.Fatalf("expected one todo item, got %#v", gotState.Todos)
	}
	gotTodo := gotState.Todos[0]
	if gotTodo.ID != "todo_1" || gotTodo.Content != "inspect README" || gotTodo.Status != harnessruntime.TodoPending || gotTodo.Priority != 7 || !gotTodo.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected todo item after reload: %#v", gotTodo)
	}
}

func TestLoadRunStateDefaultsLegacyRunPlanModeAndTodos(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	services := NewServices(config.Load(workspace))
	now := time.Now().UTC()
	session := harnessruntime.Session{
		ID:        "session_legacy",
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_legacy",
		SessionID: session.ID,
		TaskID:    "task_legacy",
		Status:    harnessruntime.RunPending,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	state := harnessruntime.RunState{
		RunID:     run.ID,
		TurnCount: 3,
		UpdatedAt: now,
	}

	if err := services.StateStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	runPath := services.Paths.RunPath(run.ID)
	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runPayload := map[string]any{
		"id":         run.ID,
		"task_id":    run.TaskID,
		"session_id": run.SessionID,
		"status":     run.Status,
		"provider":   run.Provider,
		"model":      run.Model,
		"max_turns":  run.MaxTurns,
		"created_at": run.CreatedAt,
		"updated_at": run.UpdatedAt,
	}
	runData, err := json.MarshalIndent(runPayload, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy run: %v", err)
	}
	if err := os.WriteFile(runPath, append(runData, '\n'), 0o644); err != nil {
		t.Fatalf("write legacy run: %v", err)
	}

	statePath := services.Paths.StatePath(run.ID)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	statePayload := map[string]any{
		"run_id":     state.RunID,
		"turn_count": state.TurnCount,
		"updated_at": state.UpdatedAt,
	}
	stateData, err := json.MarshalIndent(statePayload, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.WriteFile(statePath, append(stateData, '\n'), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	gotRun, gotState, err := services.LoadRunState(run.ID)
	if err != nil {
		t.Fatalf("load legacy run state: %v", err)
	}
	if gotRun.PlanMode != harnessruntime.PlanModeNone {
		t.Fatalf("expected legacy run plan mode %q, got %#v", harnessruntime.PlanModeNone, gotRun)
	}
	if len(gotState.Todos) != 0 {
		t.Fatalf("expected empty todos for legacy state, got %#v", gotState.Todos)
	}
	if gotState.TurnCount != state.TurnCount {
		t.Fatalf("unexpected legacy state after reload: %#v", gotState)
	}
}
