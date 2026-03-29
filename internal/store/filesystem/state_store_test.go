package filesystem

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestStateStoreSaveAndLoadRunArtifacts(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	stateStore := NewStateStore(paths)
	now := time.Now().UTC().Truncate(time.Second)

	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "inspect project",
		Workspace:   "/workspace",
		CreatedAt:   now,
	}
	session := harnessruntime.Session{
		ID:        "session_1",
		Workspace: "/workspace",
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_1",
		TaskID:    task.ID,
		SessionID: session.ID,
		Status:    harnessruntime.RunRunning,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  3,
		TurnCount: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	state := harnessruntime.RunState{
		RunID:         run.ID,
		CurrentStepID: "step_1",
		TurnCount:     1,
		UpdatedAt:     now,
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		RunID:   run.ID,
		Goal:    "inspect project",
		Version: 1,
		Steps: []harnessruntime.PlanStep{
			{
				ID:          "step_1",
				Title:       "Inspect",
				Description: "Inspect workspace",
				Status:      harnessruntime.StepRunning,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	result := harnessruntime.RunResult{
		RunID:       run.ID,
		Status:      harnessruntime.RunCompleted,
		Output:      "done",
		CompletedAt: now,
	}
	summaries := []harnessruntime.Summary{
		{
			ID:        "summary_1",
			RunID:     run.ID,
			Scope:     "run",
			Content:   "summary content",
			CreatedAt: now,
		},
	}
	runMemories := harnessruntime.RunMemories{
		RunID: run.ID,
		Recalled: []harnessruntime.MemoryEntry{
			{
				ID:          "mem_1",
				SessionID:   session.ID,
				Scope:       "session",
				Kind:        "decision",
				Content:     "Use Ark.",
				SourceRunID: run.ID,
				CreatedAt:   now,
			},
		},
		Candidates: []harnessruntime.MemoryCandidate{
			{
				Kind:        "fact",
				Scope:       "workspace",
				Content:     "Successful run goal: inspect project",
				SourceRunID: run.ID,
				CreatedAt:   now,
			},
		},
		UpdatedAt: now,
	}
	modelCalls := []harnessruntime.ModelCall{
		{
			ID:       "modelcall_1",
			RunID:    run.ID,
			Sequence: 10,
			Request: harnessruntime.ModelRequestSnapshot{
				SystemPrompt: "system prompt",
				Input:        "user input",
				Provider:     "mock",
				Model:        "mock-model",
				Messages: []harnessruntime.ModelMessage{
					{Role: "system", Content: "system prompt"},
					{Role: "user", Content: "user input"},
				},
				Metadata: map[string]any{"role": "default-agent"},
			},
			Response: &harnessruntime.ModelResponseSnapshot{
				Text:         `{"action":"final","answer":"done"}`,
				FinishReason: "stop",
			},
			Timestamp: now,
		},
	}
	sessionMessages := []harnessruntime.SessionMessage{
		{
			ID:        "msg_1",
			SessionID: session.ID,
			RunID:     run.ID,
			Role:      harnessruntime.MessageRoleUser,
			Content:   "inspect project",
			CreatedAt: now,
		},
		{
			ID:        "msg_2",
			SessionID: session.ID,
			RunID:     run.ID,
			Role:      harnessruntime.MessageRoleAssistant,
			Content:   "done",
			CreatedAt: now,
		},
	}

	if err := stateStore.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := stateStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := stateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := stateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := stateStore.SavePlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := stateStore.SaveResult(result); err != nil {
		t.Fatalf("save result: %v", err)
	}
	if err := stateStore.SaveSummaries(run.ID, summaries); err != nil {
		t.Fatalf("save summaries: %v", err)
	}
	if err := stateStore.SaveRunMemories(runMemories); err != nil {
		t.Fatalf("save run memories: %v", err)
	}
	for _, call := range modelCalls {
		if err := stateStore.AppendModelCall(call); err != nil {
			t.Fatalf("append model call: %v", err)
		}
	}
	for _, message := range sessionMessages {
		if err := stateStore.AppendSessionMessage(message); err != nil {
			t.Fatalf("append session message: %v", err)
		}
	}

	if _, err := os.Stat(paths.TaskPath(task.ID)); err != nil {
		t.Fatalf("task file missing: %v", err)
	}
	if _, err := os.Stat(paths.SessionPath(session.ID)); err != nil {
		t.Fatalf("session file missing: %v", err)
	}
	if _, err := os.Stat(paths.SessionMessagesPath(session.ID)); err != nil {
		t.Fatalf("session messages file missing: %v", err)
	}
	if _, err := os.Stat(paths.SessionRunPath(session.ID, run.ID)); err != nil {
		t.Fatalf("session run index file missing: %v", err)
	}

	gotRun, err := stateStore.LoadRun(run.ID)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if gotRun.Status != harnessruntime.RunRunning || gotRun.Model != "mock-model" {
		t.Fatalf("unexpected run: %#v", gotRun)
	}

	gotState, err := stateStore.LoadState(run.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if gotState.CurrentStepID != "step_1" || gotState.TurnCount != 1 {
		t.Fatalf("unexpected state: %#v", gotState)
	}

	gotPlan, err := stateStore.LoadPlan(run.ID)
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if gotPlan.Version != 1 || len(gotPlan.Steps) != 1 {
		t.Fatalf("unexpected plan: %#v", gotPlan)
	}

	gotResult, err := stateStore.LoadResult(run.ID)
	if err != nil {
		t.Fatalf("load result: %v", err)
	}
	if gotResult.Status != harnessruntime.RunCompleted || gotResult.Output != "done" {
		t.Fatalf("unexpected result: %#v", gotResult)
	}

	gotSummaries, err := stateStore.LoadSummaries(run.ID)
	if err != nil {
		t.Fatalf("load summaries: %v", err)
	}
	if len(gotSummaries) != 1 || gotSummaries[0].Content != "summary content" {
		t.Fatalf("unexpected summaries: %#v", gotSummaries)
	}

	gotRunMemories, err := stateStore.LoadRunMemories(run.ID)
	if err != nil {
		t.Fatalf("load run memories: %v", err)
	}
	if len(gotRunMemories.Recalled) != 1 || len(gotRunMemories.Candidates) != 1 {
		t.Fatalf("unexpected run memories: %#v", gotRunMemories)
	}

	gotModelCalls, err := stateStore.LoadModelCalls(run.ID)
	if err != nil {
		t.Fatalf("load model calls: %v", err)
	}
	if len(gotModelCalls) != 1 || gotModelCalls[0].Request.Input != "user input" {
		t.Fatalf("unexpected model calls: %#v", gotModelCalls)
	}
	if gotModelCalls[0].Request.Provider != "mock" || gotModelCalls[0].Request.Model != "mock-model" {
		t.Fatalf("unexpected model request identity: %#v", gotModelCalls[0].Request)
	}
	if len(gotModelCalls[0].Request.Messages) != 2 {
		t.Fatalf("expected 2 provider-view messages, got %#v", gotModelCalls[0].Request.Messages)
	}

	gotMessages, err := stateStore.LoadSessionMessages(session.ID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(gotMessages) != 2 || gotMessages[0].Role != harnessruntime.MessageRoleUser {
		t.Fatalf("unexpected session messages: %#v", gotMessages)
	}

	recentMessages, err := stateStore.LoadRecentSessionMessages(session.ID, 1)
	if err != nil {
		t.Fatalf("load recent session messages: %v", err)
	}
	if len(recentMessages) != 1 || recentMessages[0].ID != "msg_2" {
		t.Fatalf("unexpected recent session messages: %#v", recentMessages)
	}
}

func TestStateStoreLoadsLargeJSONLLines(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	stateStore := NewStateStore(paths)
	large := strings.Repeat("x", 80*1024)
	now := time.Now().UTC()
	runID := "run_large"
	sessionID := "session_large"

	if err := stateStore.AppendModelCall(harnessruntime.ModelCall{
		ID:        "modelcall_large",
		RunID:     runID,
		Sequence:  1,
		Request:   harnessruntime.ModelRequestSnapshot{Input: large},
		Response:  &harnessruntime.ModelResponseSnapshot{Text: large},
		Timestamp: now,
	}); err != nil {
		t.Fatalf("append model call: %v", err)
	}
	if err := stateStore.AppendSessionMessage(harnessruntime.SessionMessage{
		ID:        "msg_large",
		SessionID: sessionID,
		RunID:     runID,
		Role:      harnessruntime.MessageRoleAssistant,
		Content:   large,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append session message: %v", err)
	}

	modelCalls, err := stateStore.LoadModelCalls(runID)
	if err != nil {
		t.Fatalf("load model calls: %v", err)
	}
	if len(modelCalls) != 1 || modelCalls[0].Request.Input != large || modelCalls[0].Response == nil || modelCalls[0].Response.Text != large {
		t.Fatalf("unexpected large model call payload: %#v", modelCalls)
	}

	messages, err := stateStore.LoadSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != large {
		t.Fatalf("unexpected large session message payload: %#v", messages)
	}
}

func TestStateStoreLoadRunWrapsNotFoundError(t *testing.T) {
	t.Parallel()

	stateStore := NewStateStore(store.NewPaths(t.TempDir()))
	_, err := stateStore.LoadRun("run_missing")
	if err == nil {
		t.Fatal("expected missing run error")
	}
	if !errors.Is(err, harnessruntime.ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wrapped os.ErrNotExist, got %v", err)
	}
}

func TestStateStoreLoadSessionWrapsNotFoundError(t *testing.T) {
	t.Parallel()

	stateStore := NewStateStore(store.NewPaths(t.TempDir()))
	_, err := stateStore.LoadSession("session_missing")
	if err == nil {
		t.Fatal("expected missing session error")
	}
	if !errors.Is(err, harnessruntime.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wrapped os.ErrNotExist, got %v", err)
	}
}
