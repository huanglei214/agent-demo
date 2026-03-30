package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	arkmodel "github.com/huanglei214/agent-demo/internal/model/ark"
	mockmodel "github.com/huanglei214/agent-demo/internal/model/mock"
	"github.com/huanglei214/agent-demo/internal/planner"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

func TestStartRunCreatesCompletedArtifactsWithMockProvider(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	response, err := services.StartRun(context.Background(), RunRequest{
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

	modelCalls, err := services.StateStore.LoadModelCalls(response.Run.ID)
	if err != nil {
		t.Fatalf("load model calls: %v", err)
	}
	if len(modelCalls) == 0 {
		t.Fatalf("expected persisted model calls, got %#v", modelCalls)
	}
	if modelCalls[0].Request.Input == "" {
		t.Fatalf("expected persisted model request input, got %#v", modelCalls[0])
	}
	if modelCalls[0].Request.Provider != "mock" || modelCalls[0].Request.Model != "mock-model" {
		t.Fatalf("expected provider/model metadata on model call, got %#v", modelCalls[0].Request)
	}
	if len(modelCalls[0].Request.Messages) != 2 {
		t.Fatalf("expected provider-view messages on model call, got %#v", modelCalls[0].Request.Messages)
	}
	if modelCalls[0].Response == nil || modelCalls[0].Response.Text == "" {
		t.Fatalf("expected persisted model response, got %#v", modelCalls[0])
	}
}

func TestGenerateWithModelTimeoutUsesConfiguredDeadline(t *testing.T) {
	t.Parallel()

	services := NewServices(config.Config{
		Model: config.ModelConfig{
			TimeoutSeconds: 135,
		},
	})
	capturingModel := &deadlineCapturingModel{}

	_, err := services.GenerateWithModelTimeout(context.Background(), capturingModel, model.Request{
		SystemPrompt: "system",
		Input:        "hello",
	})
	if err != nil {
		t.Fatalf("generate with timeout: %v", err)
	}
	if capturingModel.deadline.IsZero() {
		t.Fatal("expected model context to have deadline")
	}
	remaining := time.Until(capturingModel.deadline)
	if remaining < 130*time.Second || remaining > 135*time.Second {
		t.Fatalf("expected deadline around 135s, got remaining %s", remaining)
	}
}

func TestStartRunReturnsContextCanceledWhenRequestContextIsCanceled(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return blockingModel{}, nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := services.StartRun(ctx, RunRequest{
		Instruction: "Wait for the request context to cancel",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestStartRunUsesExplicitTodoPlanMode(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	plannerStub := &planModePlanner{}
	runnerStub := &capturingRunner{}
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, _ *agent.ModelServices, agentServices *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		agentServices.Planner = plannerStub
	})
	services.Runner = runnerStub

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "Summarize the repository",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
		PlanMode:    harnessruntime.PlanModeTodo,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if response.Run.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected todo plan mode, got %#v", response.Run)
	}
	if plannerStub.calls != 1 {
		t.Fatalf("expected planner to be called once for explicit todo mode, got %d", plannerStub.calls)
	}
}

func TestStartRunDefaultsToNonePlanModeForSimpleInstruction(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	plannerStub := &planModePlanner{}
	runnerStub := &capturingRunner{}
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, _ *agent.ModelServices, agentServices *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		agentServices.Planner = plannerStub
	})
	services.Runner = runnerStub

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "Summarize the repository",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if response.Run.PlanMode != harnessruntime.PlanModeNone {
		t.Fatalf("expected none plan mode, got %#v", response.Run)
	}
	if plannerStub.calls != 0 {
		t.Fatalf("expected planner not to be called for simple none-mode run, got %d", plannerStub.calls)
	}
}

func TestStartRunAutoUpgradesComplexInstructionToTodoPlanMode(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	plannerStub := &planModePlanner{}
	runnerStub := &capturingRunner{}
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, _ *agent.ModelServices, agentServices *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		agentServices.Planner = plannerStub
	})
	services.Runner = runnerStub

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "先阅读 README.md，再检查 docs/，最后总结当前项目状态",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if response.Run.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected auto-upgraded todo plan mode, got %#v", response.Run)
	}
	if plannerStub.calls != 1 {
		t.Fatalf("expected planner to be called once for complex todo-mode run, got %d", plannerStub.calls)
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

	first, err := services.StartRun(context.Background(), RunRequest{
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

	second, err := services.StartRun(context.Background(), RunRequest{
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

	response, err := services.StartRun(context.Background(), RunRequest{
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

	recalled, err := memory.NewManager(store.NewPaths(cfg.Runtime.Root)).Recall(memory.RecallQuery{
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
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return staticActionModel{
				response: model.Action{
					Action: "tool",
					Calls: []model.ToolCall{{
						Tool: "fs.write_file",
						Input: map[string]any{
							"path":      "user_info.txt",
							"content":   "我已记住你的名字：黄磊",
							"overwrite": true,
						},
					}},
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
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

	response, err := services.ResumeRun(context.Background(), run.ID)
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

func TestExecuteRunMarksFailedRunAndPlanStepOnModelError(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return failingModel{
				err: &arkmodel.Error{
					Kind:    arkmodel.ErrorKindTimeout,
					Message: "request timed out",
				},
			}, nil
		}
	})

	now := time.Now()
	task := harnessruntime.Task{
		ID:          "task_failure",
		Instruction: "请总结当前状态",
		Workspace:   workspace,
		CreatedAt:   now,
	}
	session := harnessruntime.Session{
		ID:        "session_failure",
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        "run_failure",
		TaskID:    task.ID,
		SessionID: session.ID,
		Status:    harnessruntime.RunPending,
		Provider:  "ark",
		Model:     "ark-test",
		MaxTurns:  5,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := harnessruntime.Plan{
		ID:      "plan_failure",
		RunID:   run.ID,
		Goal:    task.Instruction,
		Version: 1,
		Steps: []harnessruntime.PlanStep{
			{
				ID:          "step_failure",
				Title:       "Answer the request",
				Description: task.Instruction,
				Status:      harnessruntime.StepPending,
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

	if _, err := services.ExecuteRun(context.Background(), task, session, run, plan, state, true, nil); err == nil {
		t.Fatal("expected executeRun to fail")
	}

	inspection, err := services.InspectRun(run.ID)
	if err != nil {
		t.Fatalf("inspect failed run: %v", err)
	}
	if inspection.Run.Status != harnessruntime.RunFailed {
		t.Fatalf("expected failed run, got %#v", inspection.Run)
	}
	if inspection.Plan.Steps[0].Status != harnessruntime.StepFailed {
		t.Fatalf("expected failed step, got %#v", inspection.Plan.Steps[0])
	}
	if inspection.ModelCallCount == 0 {
		t.Fatalf("expected inspect to include model call summaries, got %#v", inspection)
	}
	if len(inspection.ModelCalls) == 0 {
		t.Fatalf("expected inspect to include model calls, got %#v", inspection)
	}

	events, err := services.ReplayRun(run.ID)
	if err != nil {
		t.Fatalf("replay failed run: %v", err)
	}
	assertEventPresent(t, events, "run.failed")

	var failedEvent *harnessruntime.Event
	for i := range events {
		if events[i].Type == "run.failed" {
			failedEvent = &events[i]
			break
		}
	}
	if failedEvent == nil {
		t.Fatal("expected run.failed event")
	}
	if failedEvent.Payload["failure_kind"] != "ark_timeout" {
		t.Fatalf("expected ark_timeout failure kind, got %#v", failedEvent.Payload)
	}
	if failedEvent.Payload["retryable"] != true {
		t.Fatalf("expected retryable failure, got %#v", failedEvent.Payload)
	}
}

func TestExecuteRunRecordsStructuredToolFailureDetails(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return staticActionModel{
				response: model.Action{
					Action: "tool",
					Calls: []model.ToolCall{{
						Tool: "bash.exec",
						Input: map[string]any{
							"command":         "sleep 2",
							"workdir":         ".",
							"timeout_seconds": 1,
						},
					}},
				},
			}, nil
		}
	})

	now := time.Now()
	task, session, run, plan, state := seedStoredRun(t, services, workspace, now, harnessruntime.RunPending, "run_tool_timeout")
	task.Instruction = "Run a command that times out"
	plan.Goal = task.Instruction
	plan.Steps[0].Description = task.Instruction
	if err := services.StateStore.SaveTask(task); err != nil {
		t.Fatalf("resave task: %v", err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatalf("resave plan: %v", err)
	}

	if _, err := services.ExecuteRun(context.Background(), task, session, run, plan, state, true, nil); err == nil {
		t.Fatal("expected timeout run to fail")
	}

	replayed, err := services.ReplayRun(run.ID)
	if err != nil {
		t.Fatalf("replay failed run: %v", err)
	}

	var failedEvent *harnessruntime.Event
	for i := range replayed {
		if replayed[i].Type == "tool.failed" {
			failedEvent = &replayed[i]
			break
		}
	}
	if failedEvent == nil {
		t.Fatalf("expected tool.failed event in %#v", replayed)
	}
	details, ok := failedEvent.Payload["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured details, got %#v", failedEvent.Payload)
	}
	if details["timed_out"] != true {
		t.Fatalf("expected timed_out details, got %#v", details)
	}
}

func TestStartRunStreamObserverReceivesLifecycleEvents(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	cfg.Workspace = workspace
	cfg.Runtime.Root = filepath.Join(workspace, ".runtime")
	cfg.Model.Provider = "mock"
	cfg.Model.Model = "mock-model"

	services := NewServices(cfg)
	observer := &captureRunObserver{}

	response, err := services.StartRunStream(context.Background(), RunRequest{
		Instruction: "Summarize the repository in one short paragraph",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
	}, observer)
	if err != nil {
		t.Fatalf("expected run to succeed, got %v", err)
	}
	if response.Result == nil {
		t.Fatalf("expected run result, got %#v", response)
	}
	if !observer.hasEvent("run.started") {
		t.Fatalf("expected observer to receive run.started, got %#v", observer.types())
	}
	if !observer.hasEvent("assistant.message") {
		t.Fatalf("expected observer to receive assistant.message, got %#v", observer.types())
	}
	if !observer.hasEvent("run.completed") {
		t.Fatalf("expected observer to receive run.completed, got %#v", observer.types())
	}
}

func TestStreamedFinalAnswerPreservesOrderedDeltas(t *testing.T) {
	t.Parallel()

	provider := mockmodel.New()
	sink := &captureStreamingSink{}

	if err := provider.GenerateStream(context.Background(), model.Request{Input: "Hello, world"}, sink); err != nil {
		t.Fatalf("generate stream: %v", err)
	}

	want := []string{"Hello", ", ", "world"}
	if len(sink.deltas) != len(want) {
		t.Fatalf("expected %d streamed deltas, got %#v", len(want), sink.deltas)
	}
	for i := range want {
		if sink.deltas[i] != want[i] {
			t.Fatalf("expected delta %d to be %q, got %q (all deltas=%#v)", i, want[i], sink.deltas[i], sink.deltas)
		}
	}
	if got := sink.answer; got != "Hello, world" {
		t.Fatalf("expected final persisted answer %q, got %q", "Hello, world", got)
	}
}

type captureStreamingSink struct {
	started   int
	completed int
	failed    int
	deltas    []string
	answer    string
}

func (s *captureStreamingSink) Start() error {
	s.started++
	return nil
}

func (s *captureStreamingSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.answer += text
	return nil
}

func (s *captureStreamingSink) Complete() error {
	s.completed++
	return nil
}

func (s *captureStreamingSink) Fail(err error) error {
	_ = err
	s.failed++
	return nil
}

func TestStartRunAllowsFollowUpToolCallBeforeFinalAnswer(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "weather.txt"), []byte("Wuhan weather: cloudy 22C"), 0o644); err != nil {
		t.Fatalf("seed weather file: %v", err)
	}

	cfg := config.Load(workspace)
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &actionSequenceModel{
				responses: []model.Action{
					{
						Action: "tool",
						Calls: []model.ToolCall{{
							Tool: "fs.search",
							Input: map[string]any{
								"query": "weather.txt",
							},
						}},
					},
					{
						Action: "tool",
						Calls: []model.ToolCall{{
							Tool: "fs.read_file",
							Input: map[string]any{
								"path": "weather.txt",
							},
						}},
					},
					{
						Action: "final",
						Answer: "Wuhan weather is cloudy and 22C.",
					},
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "武汉天气怎么样",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if response.Result == nil || response.Result.Output != "Wuhan weather is cloudy and 22C." {
		t.Fatalf("unexpected result: %#v", response.Result)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	if got := countEventType(events, "tool.called"); got != 2 {
		t.Fatalf("expected two tool calls, got %#v", events)
	}
}

func TestStartRunActivatesExplicitSkillAndNarrowsPromptTools(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	seedWeatherSkill(t, workspace)

	cfg := config.Load(workspace)
	seen := &capturedModelRequest{}
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{
				captured: seen,
				response: model.Action{
					Action: "final",
					Answer: "天气查询技能已激活。",
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请帮我查武汉天气",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		Skill:       "weather-lookup",
	})
	if err != nil {
		t.Fatalf("start run with explicit skill: %v", err)
	}
	if response.Result == nil || !strings.Contains(response.Result.Output, "技能已激活") {
		t.Fatalf("unexpected result: %#v", response.Result)
	}
	if seen.SystemPrompt == "" {
		t.Fatal("expected model request to be captured")
	}
	if !strings.Contains(seen.SystemPrompt, "Active skill: weather-lookup") {
		t.Fatalf("expected skill layer in system prompt, got:\n%s", seen.SystemPrompt)
	}
	if strings.Contains(seen.SystemPrompt, "fs.read_file") {
		t.Fatalf("expected prompt tool list to be narrowed by skill, got:\n%s", seen.SystemPrompt)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "skill.activated")
}

func TestStartRunInjectsTodoPromptContextForTodoMode(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	captured := &capturedModelRequest{}
	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{
				captured: captured,
				response: model.Action{Action: "final", Answer: "done"},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "先阅读 README.md，再总结项目状态",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		PlanMode:    harnessruntime.PlanModeTodo,
	})
	if err != nil {
		t.Fatalf("start todo-mode run: %v", err)
	}
	if response.Run.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected todo plan mode, got %#v", response.Run)
	}
	for _, fragment := range []string{"Todo snapshot:", "Todo rules:", "Use the `todo` action for complex tasks when helpful."} {
		if !strings.Contains(captured.Input, fragment) {
			t.Fatalf("expected captured prompt input to contain %q, got:\n%s", fragment, captured.Input)
		}
	}
	if captured.Metadata["plan_mode"] != string(harnessruntime.PlanModeTodo) {
		t.Fatalf("expected todo plan_mode metadata, got %#v", captured.Metadata)
	}
}

func TestStartRunDoesNotInjectTodoPromptContextForNoneMode(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	captured := &capturedModelRequest{}
	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{
				captured: captured,
				response: model.Action{Action: "final", Answer: "done"},
			}, nil
		}
	})

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "Summarize the repository",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		PlanMode:    harnessruntime.PlanModeNone,
	})
	if err != nil {
		t.Fatalf("start none-mode run: %v", err)
	}
	if strings.Contains(captured.Input, "Todo snapshot:") || strings.Contains(captured.Input, "Todo rules:") {
		t.Fatalf("expected none-mode prompt to omit todo context, got:\n%s", captured.Input)
	}
}

func TestStartRunEmitsTodoUpdatedEvent(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &actionSequenceModel{
				responses: []model.Action{
					{Action: "todo", Todo: &model.TodoAction{Operation: "set", Items: []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending, Priority: 1}}}},
					{Action: "final", Answer: "done"},
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "先阅读 README.md，再总结项目状态",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		PlanMode:    harnessruntime.PlanModeTodo,
	})
	if err != nil {
		t.Fatalf("start todo-event run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	count := 0
	for _, event := range events {
		if event.Type != "todo.updated" {
			continue
		}
		count++
		if event.Payload["operation"] != "set" {
			t.Fatalf("expected todo.updated operation=set, got %#v", event.Payload)
		}
		if got, ok := event.Payload["count"].(float64); !ok || got != 1 {
			t.Fatalf("expected todo.updated count=1, got %#v", event.Payload)
		}
	}
	if count != 1 {
		t.Fatalf("expected one todo.updated event, got %d in %#v", count, events)
	}
}

func TestStartRunAutoMatchesWeatherSkill(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	seedWeatherSkill(t, workspace)

	cfg := config.Load(workspace)
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{
				response: model.Action{
					Action: "final",
					Answer: "根据天气技能进行了查询准备。",
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "武汉天气怎么样",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
	})
	if err != nil {
		t.Fatalf("start weather run: %v", err)
	}
	if response.Result == nil {
		t.Fatalf("expected result, got %#v", response)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "skill.activated")
}

func TestStartRunRejectsToolOutsideActiveSkillAllowlist(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	seedWeatherSkill(t, workspace)

	cfg := config.Load(workspace)
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return staticActionModel{
				response: model.Action{
					Action: "tool",
					Calls: []model.ToolCall{{
						Tool: "fs.read_file",
						Input: map[string]any{
							"path": "README.md",
						},
					}},
				},
			}, nil
		}
	})

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "查一下武汉天气",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		Skill:       "weather-lookup",
	})
	if err == nil {
		t.Fatal("expected run to fail when model requests a disallowed tool")
	}
	if !strings.Contains(err.Error(), "not allowed by active skill") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartRunForcesFinalAfterRepeatedWebRetrieval(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	seedWeatherSkill(t, workspace)

	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	runtimeServices := newRuntimeServices(paths)
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths)
	toolServices := newToolServices(cfg.Workspace)
	registry := toolruntime.NewRegistry()
	registry.Register(staticTool{name: "web.search", access: toolruntime.AccessReadOnly, content: map[string]any{
		"results": []map[string]any{{"title": "Weather", "url": "https://example.com/weather"}},
	}})
	registry.Register(staticTool{name: "web.fetch", access: toolruntime.AccessReadOnly, content: map[string]any{
		"title":            "Weather",
		"content":          "Today is cloudy and 22C.",
		"__echo_input_url": true,
	}})
	toolServices.ToolRegistry = registry
	toolServices.ToolExecutor = toolruntime.NewExecutor(registry)
	delegationServices := newDelegationServices(cfg, paths, toolServices)
	modelServices.ModelFactory = func() (model.Model, error) {
		return &actionSequenceModel{
			responses: []model.Action{
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/weather"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气 详细"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/weather-detail"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气 最新"}}}},
				{Action: "final", Answer: "武汉今天多云，22C。来源：https://example.com/weather"},
			},
		}, nil
	}
	services := NewServicesFromParts(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "武汉天气怎么样",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    6,
		Skill:       "weather-lookup",
	})
	if err != nil {
		t.Fatalf("expected forced final to complete run, got %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	modelCalls, err := services.StateStore.LoadModelCalls(response.Run.ID)
	if err != nil {
		t.Fatalf("load model calls: %v", err)
	}
	foundForcedFinal := false
	for _, call := range modelCalls {
		if call.Phase == "forced_final" {
			foundForcedFinal = true
			break
		}
	}
	if !foundForcedFinal {
		t.Fatalf("expected forced_final model call, got %#v", modelCalls)
	}
}

func TestStartRunFailsWhenForcedFinalStillRequestsTools(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	seedWeatherSkill(t, workspace)

	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	runtimeServices := newRuntimeServices(paths)
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths)
	toolServices := newToolServices(cfg.Workspace)
	registry := toolruntime.NewRegistry()
	registry.Register(staticTool{name: "web.search", access: toolruntime.AccessReadOnly, content: map[string]any{
		"query":   "武汉天气",
		"results": []map[string]any{{"title": "Weather", "url": "https://example.com/weather"}},
	}})
	registry.Register(staticTool{name: "web.fetch", access: toolruntime.AccessReadOnly, content: map[string]any{
		"title":            "Weather",
		"content":          "Today is cloudy and 22C.",
		"__echo_input_url": true,
	}})
	toolServices.ToolRegistry = registry
	toolServices.ToolExecutor = toolruntime.NewExecutor(registry)
	delegationServices := newDelegationServices(cfg, paths, toolServices)
	modelServices.ModelFactory = func() (model.Model, error) {
		return &actionSequenceModel{
			responses: []model.Action{
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/weather"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气 详细"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/weather-detail"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "武汉天气 最新"}}}},
				{Action: "tool", Calls: []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/weather-third"}}}},
			},
		}, nil
	}
	services := NewServicesFromParts(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "武汉天气怎么样",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    6,
		Skill:       "weather-lookup",
	})
	if err == nil {
		t.Fatal("expected run to fail when forced-final call still requests tools")
	}
	if !strings.Contains(err.Error(), "forced-final model response did not produce a final answer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartRunExecutesBatchedToolCallsInOrder(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &batchWriteReadModel{}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "先写文件再读取确认内容",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if response.Result == nil || !strings.Contains(response.Result.Output, "verified") {
		t.Fatalf("expected final verification result, got %#v", response.Result)
	}
}

func TestStartRunExecutesReadOnlyToolBatchInParallel(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	registry := toolruntime.NewRegistry()
	stats := &parallelToolStats{}
	toolA := &delayedStaticTool{name: "web.search", access: toolruntime.AccessReadOnly, delay: 120 * time.Millisecond, content: map[string]any{"query": "武汉天气"}, stats: stats}
	toolB := &delayedStaticTool{name: "web.fetch", access: toolruntime.AccessReadOnly, delay: 120 * time.Millisecond, content: map[string]any{"title": "武汉天气", "content": "多云"}, stats: stats}
	registry.Register(toolA)
	registry.Register(toolB)
	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, toolServices *agent.ToolServices, delegationServices *agent.DelegationServices) {
		toolServices.ToolRegistry = registry
		toolServices.ToolExecutor = toolruntime.NewExecutor(registry)
		*delegationServices = newDelegationServices(config.Load(workspace), store.NewPaths(config.Load(workspace).Runtime.Root), *toolServices)
		modelServices.ModelFactory = func() (model.Model, error) {
			return &actionSequenceModel{
				responses: []model.Action{
					{Action: "tool", Calls: []model.ToolCall{
						{Tool: "web.search", Input: map[string]any{"query": "武汉天气"}},
						{Tool: "web.fetch", Input: map[string]any{"url": "https://example.com/wuhan"}},
					}},
					{Action: "final", Answer: "done"},
				},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "查一下武汉天气",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if peak := stats.peak.Load(); peak < 2 {
		t.Fatalf("expected batched tools to overlap, peak concurrency was %d", peak)
	}
}

func TestResumeRunContinuesAfterInterruptedModelCall(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	now := time.Now()

	task, session, run, plan, state := seedStoredRun(t, services, workspace, now, harnessruntime.RunRunning, "run_running")
	run.CurrentStepID = plan.Steps[0].ID
	plan.Steps[0].Status = harnessruntime.StepRunning
	state.CurrentStepID = plan.Steps[0].ID
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := services.EventStore.Append(harnessruntime.Event{
		ID:        "evt_run_running_5",
		RunID:     run.ID,
		SessionID: session.ID,
		TaskID:    task.ID,
		Sequence:  5,
		Type:      "model.called",
		Timestamp: now,
		Actor:     "runtime",
		Payload: map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
		},
	}); err != nil {
		t.Fatalf("append model.called event: %v", err)
	}
	_ = task
	_ = session

	response, err := services.ResumeRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("resume running run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed resumed run, got %#v", response.Run)
	}

	events, err := services.ReplayRun(run.ID)
	if err != nil {
		t.Fatalf("replay resumed run: %v", err)
	}
	if got := countEventType(events, "model.called"); got < 2 {
		t.Fatalf("expected resumed run to retry model call, got %#v", events)
	}
}

func TestResumeRunRejectsBlockedRun(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	_, _, run, _, _ := seedStoredRun(t, services, workspace, time.Now(), harnessruntime.RunBlocked, "run_blocked")

	if _, err := services.ResumeRun(context.Background(), run.ID); err == nil {
		t.Fatal("expected blocked run to be non-resumable")
	} else if !strings.Contains(err.Error(), "manual intervention") {
		t.Fatalf("unexpected blocked resume error: %v", err)
	}
}

func TestResumeRunContinuesAfterToolExecutionWithoutReplayingTool(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	now := time.Now()

	task, session, run, plan, state := seedStoredRun(t, services, workspace, now, harnessruntime.RunRunning, "run_post_tool")
	run.CurrentStepID = plan.Steps[0].ID
	plan.Steps[0].Status = harnessruntime.StepRunning
	state.CurrentStepID = plan.Steps[0].ID
	state.TurnCount = 1
	state.ResumePhase = "post_tool"
	state.PendingToolName = "fs.list_dir"
	state.PendingToolResult = map[string]any{
		"path":  ".",
		"items": []string{"README.md"},
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
	events := []harnessruntime.Event{
		{
			ID:        "evt_run_post_tool_5",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  5,
			Type:      "tool.called",
			Timestamp: now,
			Actor:     "runtime",
			Payload: map[string]any{
				"tool": "fs.list_dir",
			},
		},
		{
			ID:        "evt_run_post_tool_6",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  6,
			Type:      "tool.succeeded",
			Timestamp: now,
			Actor:     "tool",
			Payload: map[string]any{
				"tool": "fs.list_dir",
			},
		},
	}
	for _, event := range events {
		if err := services.EventStore.Append(event); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	response, err := services.ResumeRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("resume post-tool run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed resumed run, got %#v", response.Run)
	}

	replayed, err := services.ReplayRun(run.ID)
	if err != nil {
		t.Fatalf("replay resumed run: %v", err)
	}
	if got := countEventType(replayed, "tool.called"); got != 1 {
		t.Fatalf("expected resume to continue after existing tool result without replaying tool, got %#v", replayed)
	}
	foundResumePhase := false
	for _, event := range replayed {
		if event.Type == "model.called" && event.Payload["phase"] == "post_tool_resume" {
			foundResumePhase = true
			break
		}
	}
	if !foundResumePhase {
		t.Fatalf("expected post_tool_resume model call in %#v", replayed)
	}
}

func TestResumeRunPreservesTodoSnapshotAcrossResume(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	now := time.Now().UTC()

	task, session, run, plan, state := seedStoredRun(t, services, workspace, now, harnessruntime.RunRunning, "run_resume_todo")
	run.PlanMode = harnessruntime.PlanModeTodo
	run.CurrentStepID = plan.Steps[0].ID
	plan.Steps[0].Status = harnessruntime.StepRunning
	state.CurrentStepID = plan.Steps[0].ID
	state.TurnCount = 1
	state.Todos = []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending, Priority: 1, UpdatedAt: now}}
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	captured := &capturedModelRequest{}
	services = newTestServices(t, cfg, func(runtimeServices *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		runtimeServices.EventStore = services.EventStore
		runtimeServices.StateStore = services.StateStore
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{captured: captured, response: model.Action{Action: "final", Answer: "done"}}, nil
		}
	})

	response, err := services.ResumeRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("resume todo run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed resumed run, got %#v", response.Run)
	}
	if !strings.Contains(captured.Input, "Todo snapshot:") || !strings.Contains(captured.Input, "Read README") {
		t.Fatalf("expected resumed prompt to include todo snapshot, got:\n%s", captured.Input)
	}
	gotRun, gotState, err := services.LoadRunState(run.ID)
	if err != nil {
		t.Fatalf("load resumed run state: %v", err)
	}
	if gotRun.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected todo plan mode after resume, got %#v", gotRun)
	}
	if len(gotState.Todos) != 1 || gotState.Todos[0].Content != "Read README" {
		t.Fatalf("expected preserved todo snapshot after resume, got %#v", gotState.Todos)
	}
	_ = task
	_ = session
}

func TestResumeRunPreservesNoneModeWithoutTodoState(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	cfg := config.Load(workspace)
	services := NewServices(cfg)
	now := time.Now().UTC()

	_, _, run, plan, state := seedStoredRun(t, services, workspace, now, harnessruntime.RunRunning, "run_resume_none")
	run.PlanMode = harnessruntime.PlanModeNone
	run.CurrentStepID = plan.Steps[0].ID
	plan.Steps[0].Status = harnessruntime.StepRunning
	state.CurrentStepID = plan.Steps[0].ID
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	response, err := services.ResumeRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("resume none-mode run: %v", err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed resumed run, got %#v", response.Run)
	}
	gotRun, gotState, err := services.LoadRunState(run.ID)
	if err != nil {
		t.Fatalf("load resumed run state: %v", err)
	}
	if gotRun.PlanMode != harnessruntime.PlanModeNone {
		t.Fatalf("expected none plan mode after resume, got %#v", gotRun)
	}
	if len(gotState.Todos) != 0 {
		t.Fatalf("expected no todos for none mode after resume, got %#v", gotState.Todos)
	}
}

func TestResumeRunRejectsRunWithPersistedResult(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	cfg := config.Load(workspace)
	services := NewServices(cfg)

	_, _, run, _, _ := seedStoredRun(t, services, workspace, time.Now(), harnessruntime.RunRunning, "run_with_result")
	if err := services.StateStore.SaveResult(harnessruntime.RunResult{
		RunID:       run.ID,
		Status:      harnessruntime.RunCompleted,
		Output:      "done",
		CompletedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	if _, err := services.ResumeRun(context.Background(), run.ID); err == nil {
		t.Fatal("expected run with persisted result to be non-resumable")
	} else if !strings.Contains(err.Error(), "persisted result") {
		t.Fatalf("unexpected persisted result error: %v", err)
	}
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

func countEventType(events []harnessruntime.Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

type captureRunObserver struct {
	events []harnessruntime.Event
}

func (o *captureRunObserver) OnAnswerStreamEvent(agent.AnswerStreamEvent) {}

func (o *captureRunObserver) OnRuntimeEvent(event harnessruntime.Event) {
	o.events = append(o.events, event)
}

func (o *captureRunObserver) hasEvent(eventType string) bool {
	for _, event := range o.events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func (o *captureRunObserver) types() []string {
	result := make([]string, 0, len(o.events))
	for _, event := range o.events {
		result = append(result, event.Type)
	}
	return result
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

type capturedModelRequest struct {
	SystemPrompt string
	Input        string
	Metadata     map[string]any
}

type inspectingModel struct {
	captured *capturedModelRequest
	response model.Action
}

func (m *inspectingModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	if m.captured != nil {
		m.captured.SystemPrompt = req.SystemPrompt
		m.captured.Input = req.Input
		m.captured.Metadata = req.Metadata
	}
	data, _ := json.Marshal(m.response)
	return model.Response{
		Text:         string(data),
		FinishReason: "stop",
	}, nil
}

type failingModel struct {
	err error
}

type planModePlanner struct {
	calls int
}

func (p *planModePlanner) CreatePlan(ctx context.Context, input planner.PlanInput) (harnessruntime.Plan, error) {
	p.calls++
	return planner.New().CreatePlan(ctx, input)
}

func (p *planModePlanner) Replan(ctx context.Context, input planner.ReplanInput) (harnessruntime.Plan, error) {
	return planner.New().Replan(ctx, input)
}

func (blockingModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = req
	<-ctx.Done()
	return model.Response{}, ctx.Err()
}

func (m failingModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	return model.Response{}, m.err
}

func seedStoredRun(t *testing.T, services Services, workspace string, now time.Time, status harnessruntime.RunStatus, runID string) (harnessruntime.Task, harnessruntime.Session, harnessruntime.Run, harnessruntime.Plan, harnessruntime.RunState) {
	t.Helper()

	task := harnessruntime.Task{
		ID:          "task_" + runID,
		Instruction: "请读取 README.md 并总结当前项目状态",
		Workspace:   workspace,
		CreatedAt:   now,
	}
	session := harnessruntime.Session{
		ID:        "session_" + runID,
		Workspace: workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        runID,
		TaskID:    task.ID,
		SessionID: session.ID,
		Status:    status,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  5,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := harnessruntime.Plan{
		ID:      "plan_" + runID,
		RunID:   run.ID,
		Goal:    task.Instruction,
		Version: 1,
		Steps: []harnessruntime.PlanStep{
			{
				ID:              "step_" + runID,
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
	events := []harnessruntime.Event{
		{
			ID:        "evt_" + runID + "_1",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  1,
			Type:      "task.created",
			Timestamp: now,
			Actor:     "system",
		},
		{
			ID:        "evt_" + runID + "_2",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  2,
			Type:      "run.created",
			Timestamp: now,
			Actor:     "system",
		},
		{
			ID:        "evt_" + runID + "_3",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  3,
			Type:      "plan.created",
			Timestamp: now,
			Actor:     "planner",
		},
	}
	if status != harnessruntime.RunPending {
		events = append(events, harnessruntime.Event{
			ID:        "evt_" + runID + "_4",
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  4,
			Type:      "run.status_changed",
			Timestamp: now,
			Actor:     "runtime",
			Payload: map[string]any{
				"from": harnessruntime.RunPending,
				"to":   status,
			},
		})
	}
	for _, event := range events {
		if err := services.EventStore.Append(event); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	return task, session, run, plan, state
}

func seedWeatherSkill(t *testing.T, workspace string) {
	t.Helper()
	path := filepath.Join(workspace, "skills", "weather-lookup", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir weather skill: %v", err)
	}
	content := `---
name: weather-lookup
description: 查询城市实时天气并给出来源
allowed-tools:
  - web.search
  - web.fetch
tags:
  - 天气
  - 温度
---
先搜索再读取页面，不要只返回链接。`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write weather skill: %v", err)
	}
}

type actionSequenceModel struct {
	responses []model.Action
	index     int
}

type deadlineCapturingModel struct {
	deadline time.Time
}

type blockingModel struct{}

type batchWriteReadModel struct {
	index int
}

type staticTool struct {
	name    string
	access  toolruntime.AccessMode
	content map[string]any
}

type delayedStaticTool struct {
	name    string
	access  toolruntime.AccessMode
	delay   time.Duration
	content map[string]any
	stats   *parallelToolStats
}

type parallelToolStats struct {
	current atomic.Int32
	peak    atomic.Int32
}

func (t staticTool) Name() string                       { return t.name }
func (t staticTool) Description() string                { return "static test tool" }
func (t staticTool) AccessMode() toolruntime.AccessMode { return t.access }
func (t staticTool) Execute(ctx context.Context, input json.RawMessage) (toolruntime.Result, error) {
	_ = ctx
	content := map[string]any{}
	for key, value := range t.content {
		content[key] = value
	}
	if echoURL, ok := content["__echo_input_url"].(bool); ok && echoURL {
		delete(content, "__echo_input_url")
		var payload map[string]any
		if err := json.Unmarshal(input, &payload); err == nil {
			if url, ok := payload["url"].(string); ok && strings.TrimSpace(url) != "" {
				content["final_url"] = url
			}
		}
	}
	return toolruntime.Result{Content: content}, nil
}

func (t *delayedStaticTool) Name() string                       { return t.name }
func (t *delayedStaticTool) Description() string                { return "delayed static test tool" }
func (t *delayedStaticTool) AccessMode() toolruntime.AccessMode { return t.access }
func (t *delayedStaticTool) Execute(ctx context.Context, input json.RawMessage) (toolruntime.Result, error) {
	_ = input
	current := t.stats.current.Add(1)
	for {
		peak := t.stats.peak.Load()
		if current <= peak || t.stats.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	defer t.stats.current.Add(-1)
	select {
	case <-ctx.Done():
		return toolruntime.Result{}, ctx.Err()
	case <-time.After(t.delay):
	}
	content := make(map[string]any, len(t.content))
	for key, value := range t.content {
		content[key] = value
	}
	return toolruntime.Result{Content: content}, nil
}

func (m *actionSequenceModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	if m.index >= len(m.responses) {
		return model.Response{}, errors.New("scripted model exhausted")
	}
	data, _ := json.Marshal(m.responses[m.index])
	m.index++
	return model.Response{
		Text:         string(data),
		FinishReason: "stop",
	}, nil
}

func (m *deadlineCapturingModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = req
	deadline, _ := ctx.Deadline()
	m.deadline = deadline
	return model.Response{
		Text:         `{"action":"final","answer":"ok"}`,
		FinishReason: "stop",
	}, nil
}

func (m *batchWriteReadModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	switch m.index {
	case 0:
		m.index++
		return model.Response{
			Text:         `{"action":"tool","calls":[{"tool":"fs.write_file","input":{"path":"notes.txt","content":"hello from batch tools"}},{"tool":"fs.read_file","input":{"path":"notes.txt"}}]}`,
			FinishReason: "stop",
		}, nil
	case 1:
		m.index++
		if !strings.Contains(req.Input, "hello from batch tools") {
			return model.Response{}, errors.New("follow-up prompt did not include content from ordered read")
		}
		return model.Response{
			Text:         `{"action":"final","answer":"verified batched tools preserve order"}`,
			FinishReason: "stop",
		}, nil
	default:
		return model.Response{}, errors.New("batchWriteReadModel exhausted")
	}
}
