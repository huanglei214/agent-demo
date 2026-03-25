package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestStartRunCreatesCompletedArtifactsWithMockProvider(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	response, err := services.StartRun(RunRequest{
		Instruction: "请读取 README.md 并总结当前项目状态",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if response.Result == nil || strings.TrimSpace(response.Result.Output) == "" {
		t.Fatalf("expected final result, got %#v", response.Result)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "prompt.built")
	assertEventPresent(t, events, "context.built")
	assertEventPresent(t, events, "user.message")
	assertEventPresent(t, events, "assistant.message")
	assertEventPresent(t, events, "memory.candidate_extracted")
	assertEventPresent(t, events, "memory.committed")

	memories, err := services.StateStore.LoadRunMemories(response.Run.ID)
	if err != nil {
		t.Fatalf("load run memories: %v", err)
	}
	if len(memories.Candidates) == 0 || len(memories.Committed) == 0 {
		t.Fatalf("expected persisted run memories, got %#v", memories)
	}

	messages, err := services.StateStore.LoadSessionMessages(response.Run.SessionID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two session messages, got %#v", messages)
	}
}

func TestStartRunReusesExistingSession(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	session, err := services.CreateSession(workspace)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := services.StartRun(RunRequest{
		Instruction: "第一轮问题",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   session.ID,
	})
	if err != nil {
		t.Fatalf("start first session run: %v", err)
	}

	second, err := services.StartRun(RunRequest{
		Instruction: "第二轮追问",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   session.ID,
	})
	if err != nil {
		t.Fatalf("start second session run: %v", err)
	}

	if first.Run.SessionID != session.ID || second.Run.SessionID != session.ID {
		t.Fatalf("expected both runs to reuse session %s, got %s and %s", session.ID, first.Run.SessionID, second.Run.SessionID)
	}

	messages, err := services.StateStore.LoadSessionMessages(session.ID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected four session messages across two turns, got %#v", messages)
	}
	if messages[2].Content != "第二轮追问" {
		t.Fatalf("expected second user turn to be preserved in order, got %#v", messages)
	}
}

func TestStartRunRoutesRememberRequestsToMemory(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	response, err := services.StartRun(RunRequest{
		Instruction: "我是黄磊，请记住",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if response.Result == nil || !strings.Contains(response.Result.Output, "黄磊") {
		t.Fatalf("expected memory-routed response mentioning 黄磊, got %#v", response.Result)
	}

	if _, err := os.Stat(filepath.Join(workspace, "user_info.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected no user_info.txt file to be created, got err=%v", err)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "memory.routed")
	assertEventPresent(t, events, "memory.committed")
	assertEventAbsent(t, events, "tool.called")
	assertEventAbsent(t, events, "fs.file_created")

	recalled, err := services.MemoryManager.Recall(memory.RecallQuery{
		SessionID: response.Run.SessionID,
		Goal:      "我是谁",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("recall memory: %v", err)
	}
	if len(recalled) == 0 || !strings.Contains(recalled[0].Content, "黄磊") {
		t.Fatalf("expected recalled memory to contain 黄磊, got %#v", recalled)
	}
}

func TestStartRunInterceptsMemoryLikeWriteFileToolInConversationMode(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	services.ModelFactory = func() (model.Model, error) {
		return staticActionModel{
			response: model.Action{
				Action: "tool",
				Tool:   "fs.write_file",
				Input: map[string]any{
					"path":      "user_info.txt",
					"content":   "我已记住你的名字：黄磊",
					"overwrite": true,
				},
			},
		}, nil
	}

	response, err := services.StartRun(RunRequest{
		Instruction: "我是黄磊，请记住",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if response.Result == nil || !strings.Contains(response.Result.Output, "黄磊") {
		t.Fatalf("expected intercepted response mentioning 黄磊, got %#v", response.Result)
	}
	if _, err := os.Stat(filepath.Join(workspace, "user_info.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected intercepted write_file not to create user_info.txt, got err=%v", err)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "memory.routed")
	assertEventAbsent(t, events, "tool.called")
	assertEventAbsent(t, events, "fs.file_created")
}

func TestResumeRunContinuesPendingRun(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	now := time.Now()

	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "请读取 README.md 并总结当前项目状态",
		Workspace:   workspace,
		CreatedAt:   now,
	}
	session := harnessruntime.Session{
		ID:        "session_1",
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_1",
		TaskID:    task.ID,
		SessionID: session.ID,
		Status:    harnessruntime.RunPending,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  5,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		RunID:   run.ID,
		Goal:    task.Instruction,
		Version: 1,
		Steps: []harnessruntime.PlanStep{
			{
				ID:              "step_1",
				Title:           "Read relevant workspace files",
				Description:     task.Instruction,
				Status:          harnessruntime.StepPending,
				EstimatedEffort: "small",
				OutputSchema:    "final-answer",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	state := harnessruntime.RunState{
		RunID:     run.ID,
		TurnCount: 0,
		UpdatedAt: now,
	}

	if err := services.StateStore.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := services.StateStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	response, err := services.ResumeRun(run.ID)
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed resumed run, got %#v", response.Run)
	}

	events, err := services.ReplayRun(run.ID)
	if err != nil {
		t.Fatalf("replay resumed run: %v", err)
	}
	assertEventPresent(t, events, "run.started")
	assertEventPresent(t, events, "run.completed")
}

func assertEventPresent(t *testing.T, events []harnessruntime.Event, eventType string) {
	t.Helper()

	for _, event := range events {
		if event.Type == eventType {
			return
		}
	}
	t.Fatalf("expected event %q in %#v", eventType, events)
}

func assertEventAbsent(t *testing.T, events []harnessruntime.Event, eventType string) {
	t.Helper()

	for _, event := range events {
		if event.Type == eventType {
			t.Fatalf("did not expect event %q in %#v", eventType, events)
		}
	}
}

type staticActionModel struct {
	response model.Action
}

func (m staticActionModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	data, _ := json.Marshal(m.response)
	return model.Response{
		Text:         string(data),
		FinishReason: "stop",
	}, nil
}
