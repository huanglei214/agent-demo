package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	"github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestNewServicesExposeToolAccessAndReadOnlyDelegationPolicy(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	services := NewServices(config.Load(workspace))

	tools := services.ListTools()
	accessByName := map[string]string{}
	for _, descriptor := range tools {
		accessByName[descriptor.Name] = string(descriptor.Access)
	}

	if accessByName["fs.read_file"] != "read_only" {
		t.Fatalf("expected fs.read_file to be read_only, got %#v", tools)
	}
	if accessByName["fs.write_file"] != "write" {
		t.Fatalf("expected fs.write_file to be write, got %#v", tools)
	}
	if accessByName["fs.str_replace"] != "write" {
		t.Fatalf("expected fs.str_replace to be write, got %#v", tools)
	}
	if accessByName["web.search"] != "read_only" {
		t.Fatalf("expected web.search to be read_only, got %#v", tools)
	}
	if accessByName["web.fetch"] != "read_only" {
		t.Fatalf("expected web.fetch to be read_only, got %#v", tools)
	}
	if accessByName["bash.exec"] != "exec" {
		t.Fatalf("expected bash.exec to be exec, got %#v", tools)
	}
	if _, ok := accessByName["fs.stat"]; ok {
		t.Fatalf("expected fs.stat to be removed from tool registry, got %#v", tools)
	}

	task := services.DelegationManager.BuildTask(
		harnessruntime.Run{ID: "run_parent", SessionID: "session_1"},
		harnessruntime.Plan{RunID: "run_parent", Goal: "整理仓库"},
		harnessruntime.PlanStep{
			ID:          "step_1",
			Title:       "Delegate a bounded child task",
			Description: "analyze the repository",
			Delegatable: true,
		},
		"分析仓库",
		nil,
		nil,
	)

	if len(task.AllowedTools) == 0 {
		t.Fatal("expected read-only child tools to be present")
	}
	for _, allowed := range task.AllowedTools {
		if allowed == "fs.write_file" {
			t.Fatalf("expected write tool to be excluded from child delegation policy, got %#v", task.AllowedTools)
		}
		if allowed == "fs.str_replace" {
			t.Fatalf("expected str_replace tool to be excluded from child delegation policy, got %#v", task.AllowedTools)
		}
	}
}

func TestNewServicesUnsupportedProviderUsesSentinelError(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	cfg.Model.Provider = "unknown"

	_, err := newModelServices(cfg).ModelFactory()
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
	if !errors.Is(err, harnessruntime.ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
	}
}

func TestNewServicesFromPartsAcceptsInterfaceImplementations(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	paths := store.NewPaths(cfg.Runtime.Root)
	runtimeServices := newRuntimeServices(paths)
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths)
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths, toolServices)

	memoryService := &recordingMemoryService{}
	contextService := &recordingContextService{}
	promptService := &recordingPromptService{}
	agentServices.MemoryManager = memoryService
	agentServices.ContextManager = contextService
	modelServices.PromptBuilder = promptService
	modelServices.ModelFactory = func() (model.Model, error) {
		return staticActionModel{
			response: model.Action{
				Action: "final",
				Answer: "done",
			},
		}, nil
	}

	services := NewServicesFromParts(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)
	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "Summarize the workspace briefly",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if memoryService.recallCalls == 0 {
		t.Fatal("expected custom memory service to be used during run execution")
	}
	if contextService.buildCalls == 0 {
		t.Fatal("expected custom context service to be used during run execution")
	}
	if promptService.buildRunPromptCalls == 0 {
		t.Fatal("expected custom prompt service to be used during run execution")
	}
}

func TestServicesExposeRunScopedQueriesWithoutDirectStoreAccess(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	services := NewServices(cfg)
	now := time.Now()

	task, session, run, _, state := seedStoredRun(t, services, cfg.Workspace, now, harnessruntime.RunCompleted, "run_query")
	message := harnessruntime.SessionMessage{
		ID:        "msg_1",
		SessionID: session.ID,
		RunID:     run.ID,
		Role:      harnessruntime.MessageRoleUser,
		Content:   task.Instruction,
		CreatedAt: now,
	}
	if err := services.StateStore.AppendSessionMessage(message); err != nil {
		t.Fatalf("append session message: %v", err)
	}

	loadedRun, err := services.LoadRun(run.ID)
	if err != nil {
		t.Fatalf("load run via service: %v", err)
	}
	if loadedRun.ID != run.ID {
		t.Fatalf("expected run %q, got %#v", run.ID, loadedRun)
	}

	gotRun, gotState, err := services.LoadRunState(run.ID)
	if err != nil {
		t.Fatalf("load run state via service: %v", err)
	}
	if gotRun.ID != run.ID || gotState.RunID != state.RunID {
		t.Fatalf("expected run/state for %q, got run=%#v state=%#v", run.ID, gotRun, gotState)
	}

	messages, err := services.LoadRecentSessionMessages(session.ID, 5)
	if err != nil {
		t.Fatalf("load recent session messages via service: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("expected one message %#v, got %#v", message, messages)
	}
}

func TestStartRunPassesContextToPlannerAndRunner(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	paths := store.NewPaths(cfg.Runtime.Root)
	runtimeServices := newRuntimeServices(paths)
	modelServices := newModelServices(cfg)
	plannerStub := &recordingPlanner{}
	agentServices := newAgentServices(cfg, paths)
	agentServices.Planner = plannerStub
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths, toolServices)
	services := NewServicesFromParts(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)

	runnerStub := &capturingRunner{}
	services.Runner = runnerStub

	ctxKey := struct{}{}
	ctx := context.WithValue(context.Background(), ctxKey, "planner-runner")
	if _, err := services.StartRun(ctx, RunRequest{
		Instruction: "Summarize the workspace briefly",
		Workspace:   cfg.Workspace,
		Provider:    "test-provider",
		Model:       "test-model",
		MaxTurns:    1,
		PlanMode:    harnessruntime.PlanModeTodo,
	}); err != nil {
		t.Fatalf("start run: %v", err)
	}

	if plannerStub.lastCtx == nil || plannerStub.lastCtx.Value(ctxKey) != "planner-runner" {
		t.Fatalf("expected planner to receive request context, got %#v", plannerStub.lastCtx)
	}
	if runnerStub.executeCtx == nil || runnerStub.executeCtx.Value(ctxKey) != "planner-runner" {
		t.Fatalf("expected runner to receive request context, got %#v", runnerStub.executeCtx)
	}
}

func TestResumeRunPassesContextToRunner(t *testing.T) {
	t.Parallel()

	services := NewServices(config.Load(t.TempDir()))
	runnerStub := &capturingRunner{}
	services.Runner = runnerStub

	ctxKey := struct{}{}
	ctx := context.WithValue(context.Background(), ctxKey, "resume")
	if _, err := services.ResumeRun(ctx, "run_123"); err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if runnerStub.resumeCtx == nil || runnerStub.resumeCtx.Value(ctxKey) != "resume" {
		t.Fatalf("expected resume runner to receive request context, got %#v", runnerStub.resumeCtx)
	}
}

type stubMemoryService struct{}

func (stubMemoryService) Recall(memory.RecallQuery) ([]harnessruntime.MemoryEntry, error) {
	return nil, nil
}

func (stubMemoryService) DetectExplicitRemember(memory.ExplicitRememberInput) ([]harnessruntime.MemoryCandidate, string, bool) {
	return nil, "", false
}

func (stubMemoryService) ExtractCandidates(memory.ExtractInput) []harnessruntime.MemoryCandidate {
	return nil
}

func (stubMemoryService) CommitCandidates(string, []harnessruntime.MemoryCandidate) ([]harnessruntime.MemoryEntry, error) {
	return nil, nil
}

type stubContextService struct{}

func (stubContextService) Build(harnesscontext.BuildInput) harnesscontext.ModelContext {
	return harnesscontext.ModelContext{}
}

func (stubContextService) ShouldCompact(harnesscontext.CompactionCheckInput) (bool, string) {
	return false, ""
}

func (stubContextService) Compact(harnesscontext.CompactInput) (harnessruntime.Summary, error) {
	return harnessruntime.Summary{}, nil
}

type stubPromptService struct{}

func (stubPromptService) BuildRunPrompt(harnessruntime.RunRole, harnessruntime.Task, harnessruntime.Plan, *harnessruntime.PlanStep, harnesscontext.ModelContext, []map[string]string, *skill.Definition) prompt.Prompt {
	return prompt.Prompt{}
}

func (stubPromptService) BuildFollowUpPrompt(harnessruntime.RunRole, harnessruntime.Task, []harnessruntime.ToolCallResult, map[string]any, []map[string]string, *skill.Definition) prompt.Prompt {
	return prompt.Prompt{}
}

func (stubPromptService) BuildForcedFinalPrompt(harnessruntime.RunRole, harnessruntime.Task, string, map[string]any, []map[string]string, *skill.Definition) prompt.Prompt {
	return prompt.Prompt{}
}

type recordingMemoryService struct {
	recallCalls int
}

func (s *recordingMemoryService) Recall(memory.RecallQuery) ([]harnessruntime.MemoryEntry, error) {
	s.recallCalls++
	return nil, nil
}

func (*recordingMemoryService) DetectExplicitRemember(memory.ExplicitRememberInput) ([]harnessruntime.MemoryCandidate, string, bool) {
	return nil, "", false
}

func (*recordingMemoryService) ExtractCandidates(memory.ExtractInput) []harnessruntime.MemoryCandidate {
	return nil
}

func (*recordingMemoryService) CommitCandidates(string, []harnessruntime.MemoryCandidate) ([]harnessruntime.MemoryEntry, error) {
	return nil, nil
}

type recordingContextService struct {
	buildCalls int
}

func (s *recordingContextService) Build(harnesscontext.BuildInput) harnesscontext.ModelContext {
	s.buildCalls++
	return harnesscontext.ModelContext{}
}

func (*recordingContextService) ShouldCompact(harnesscontext.CompactionCheckInput) (bool, string) {
	return false, ""
}

func (*recordingContextService) Compact(harnesscontext.CompactInput) (harnessruntime.Summary, error) {
	return harnessruntime.Summary{}, nil
}

type recordingPromptService struct {
	buildRunPromptCalls int
}

func (s *recordingPromptService) BuildRunPrompt(harnessruntime.RunRole, harnessruntime.Task, harnessruntime.Plan, *harnessruntime.PlanStep, harnesscontext.ModelContext, []map[string]string, *skill.Definition) prompt.Prompt {
	s.buildRunPromptCalls++
	return prompt.Prompt{
		System: "system",
		Input:  "user",
	}
}

func (*recordingPromptService) BuildFollowUpPrompt(harnessruntime.RunRole, harnessruntime.Task, []harnessruntime.ToolCallResult, map[string]any, []map[string]string, *skill.Definition) prompt.Prompt {
	return prompt.Prompt{}
}

func (*recordingPromptService) BuildForcedFinalPrompt(harnessruntime.RunRole, harnessruntime.Task, string, map[string]any, []map[string]string, *skill.Definition) prompt.Prompt {
	return prompt.Prompt{}
}

type recordingPlanner struct {
	lastCtx context.Context
}

func (p *recordingPlanner) CreatePlan(ctx context.Context, input planner.PlanInput) (harnessruntime.Plan, error) {
	p.lastCtx = ctx
	return planner.New().CreatePlan(ctx, input)
}

func (p *recordingPlanner) Replan(ctx context.Context, input planner.ReplanInput) (harnessruntime.Plan, error) {
	return planner.New().Replan(ctx, input)
}

type capturingRunner struct {
	executeCtx context.Context
	resumeCtx  context.Context
}

func (r *capturingRunner) ExecuteRun(ctx context.Context, task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer agent.RunObserver) (agent.ExecutionResponse, error) {
	r.executeCtx = ctx
	return agent.ExecutionResponse{
		Task: task,
		Run:  run,
	}, nil
}

func (r *capturingRunner) ResumeRun(ctx context.Context, runID string, observer agent.RunObserver) (agent.ExecutionResponse, error) {
	r.resumeCtx = ctx
	return agent.ExecutionResponse{
		Run: harnessruntime.Run{ID: runID},
	}, nil
}
