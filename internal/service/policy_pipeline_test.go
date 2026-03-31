package service

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
	"github.com/huanglei214/agent-demo/internal/store"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

type countingNoopRuntimePolicy struct {
	beforeRunCount  atomic.Int32
	afterModelCount atomic.Int32
}

type mutatingActionNoopPolicy struct{}
type mutatingAfterActionNoopPolicy struct{}

type mutatingContextNoopPolicy struct {
	afterModelCount        atomic.Int32
	sawIsolatedPlan        atomic.Bool
	sawIsolatedCurrentStep atomic.Bool
	sawIsolatedEvidence    atomic.Bool
}

func (*countingNoopRuntimePolicy) Name() string { return "noop" }

func (p *countingNoopRuntimePolicy) BeforeRun(context.Context, *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	p.beforeRunCount.Add(1)
	return policy.Continue(), nil
}

func (p *countingNoopRuntimePolicy) AfterModel(context.Context, *policy.ExecutionContext, *model.Action) (*policy.PolicyOutcome, error) {
	p.afterModelCount.Add(1)
	return policy.Continue(), nil
}

func (*countingNoopRuntimePolicy) AfterAction(context.Context, *policy.ExecutionContext, model.Action, policy.ActionResult) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (*mutatingActionNoopPolicy) Name() string { return "mutating-action-noop" }

func (*mutatingActionNoopPolicy) BeforeRun(context.Context, *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (*mutatingActionNoopPolicy) AfterModel(_ context.Context, _ *policy.ExecutionContext, action *model.Action) (*policy.PolicyOutcome, error) {
	action.Action = "tool"
	action.Answer = "tampered"
	action.Calls = []model.ToolCall{{Tool: "fs.write_file", Input: map[string]any{"path": "tampered.txt"}}}
	return policy.Continue(), nil
}

func (*mutatingActionNoopPolicy) AfterAction(context.Context, *policy.ExecutionContext, model.Action, policy.ActionResult) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (*mutatingAfterActionNoopPolicy) Name() string { return "mutating-after-action-noop" }

func (*mutatingAfterActionNoopPolicy) BeforeRun(context.Context, *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (*mutatingAfterActionNoopPolicy) AfterModel(context.Context, *policy.ExecutionContext, *model.Action) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (*mutatingAfterActionNoopPolicy) AfterAction(_ context.Context, _ *policy.ExecutionContext, action model.Action, result policy.ActionResult) (*policy.PolicyOutcome, error) {
	if len(action.Calls) > 0 {
		action.Calls[0].Input["path"] = "tampered.txt"
	}
	if len(result.ToolCalls) > 0 {
		result.ToolCalls[0].Input["path"] = "tampered.txt"
	}
	if len(result.ToolResults) > 0 {
		result.ToolResults[0].Input["path"] = "tampered.txt"
		result.ToolResults[0].Result["nested"] = map[string]any{"tampered": true}
	}
	return policy.Continue(), nil
}

func (*mutatingContextNoopPolicy) Name() string { return "mutating-context-noop" }

func (p *mutatingContextNoopPolicy) BeforeRun(_ context.Context, execCtx *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	if execCtx.Plan != nil {
		execCtx.Plan.Goal = "tampered-goal"
		if len(execCtx.Plan.Steps) > 0 {
			execCtx.Plan.Steps[0].Title = "tampered-plan-title"
			execCtx.Plan.Steps[0].Description = "tampered-plan-description"
		}
	}
	if execCtx.CurrentStep != nil {
		execCtx.CurrentStep.Title = "tampered-current-step-title"
		execCtx.CurrentStep.Description = "tampered-current-step-description"
	}
	execCtx.WorkingEvidence = map[string]any{"tampered": true}
	execCtx.Metadata = map[string]any{"tampered": true}
	execCtx.ExplicitCandidates = append(execCtx.ExplicitCandidates, harnessruntime.MemoryCandidate{Content: "tampered"})
	execCtx.Memories = append(execCtx.Memories, harnessruntime.MemoryEntry{Content: "tampered"})
	execCtx.Summaries = append(execCtx.Summaries, harnessruntime.Summary{Content: "tampered"})
	return policy.Continue(), nil
}

func (p *mutatingContextNoopPolicy) AfterModel(_ context.Context, execCtx *policy.ExecutionContext, _ *model.Action) (*policy.PolicyOutcome, error) {
	p.afterModelCount.Add(1)
	if execCtx.Plan != nil && execCtx.Plan.Goal != "tampered-goal" && len(execCtx.Plan.Steps) > 0 &&
		execCtx.Plan.Steps[0].Title != "tampered-plan-title" && execCtx.Plan.Steps[0].Description != "tampered-plan-description" {
		p.sawIsolatedPlan.Store(true)
	}
	if execCtx.CurrentStep != nil && execCtx.CurrentStep.Title != "tampered-current-step-title" &&
		execCtx.CurrentStep.Description != "tampered-current-step-description" {
		p.sawIsolatedCurrentStep.Store(true)
	}
	if len(execCtx.WorkingEvidence) == 0 {
		p.sawIsolatedEvidence.Store(true)
	}
	return policy.Continue(), nil
}

func (*mutatingContextNoopPolicy) AfterAction(context.Context, *policy.ExecutionContext, model.Action, policy.ActionResult) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func TestExecutorWithNoopPolicyPreservesFinalAction(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	baselineCfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "baseline-runtime"),
	})
	baselineServices := newTestServices(t, baselineCfg, nil)
	baselineResponse, err := baselineServices.StartRun(context.Background(), RunRequest{
		Instruction: "读取当前仓库并给出一句摘要",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("baseline start run: %v", err)
	}
	if baselineResponse.Result == nil || strings.TrimSpace(baselineResponse.Result.Output) == "" {
		t.Fatal("expected baseline final output to remain available")
	}

	cfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "policy-runtime"),
	})
	paths := newRuntimeServices(store.NewPaths(cfg.Runtime.Root))
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths.Paths)
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths.Paths, toolServices)
	services := NewServicesFromParts(cfg, paths, modelServices, agentServices, toolServices, delegationServices)
	noopPolicy := &countingNoopRuntimePolicy{}

	executor := agent.NewExecutor(cfg, paths, modelServices, agentServices, toolServices, delegationServices)
	executor.Policies = []policy.RuntimePolicy{noopPolicy}
	services.Runner = executor
	services.ModelCaller = executor

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "读取当前仓库并给出一句摘要",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Result == nil || strings.TrimSpace(response.Result.Output) == "" {
		t.Fatal("expected final output to remain available")
	}
	if normalizePolicyPipelineOutput(response.Result.Output) != normalizePolicyPipelineOutput(baselineResponse.Result.Output) {
		t.Fatalf("expected noop policy to preserve final output, baseline=%q with_policy=%q", baselineResponse.Result.Output, response.Result.Output)
	}
	if noopPolicy.beforeRunCount.Load() == 0 {
		t.Fatal("expected BeforeRun hook to be called")
	}
	if noopPolicy.afterModelCount.Load() == 0 {
		t.Fatal("expected AfterModel hook to be called")
	}
}

func TestExecutorRejectsNoopPolicyThatMutatesAfterModelAction(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	cfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "policy-runtime"),
	})
	paths := newRuntimeServices(store.NewPaths(cfg.Runtime.Root))
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths.Paths)
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths.Paths, toolServices)
	services := NewServicesFromParts(cfg, paths, modelServices, agentServices, toolServices, delegationServices)

	executor := agent.NewExecutor(cfg, paths, modelServices, agentServices, toolServices, delegationServices)
	executor.Policies = []policy.RuntimePolicy{&mutatingActionNoopPolicy{}}
	services.Runner = executor
	services.ModelCaller = executor

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "读取当前仓库并给出一句摘要",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err == nil || !strings.Contains(err.Error(), "mutated action") {
		t.Fatalf("expected action mutation to be rejected, got %v", err)
	}
}

func TestExecutorWithNoopPolicyIsolatesPolicyContextSnapshots(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	baselineCfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "baseline-runtime"),
	})
	baselineServices := newTestServices(t, baselineCfg, nil)
	baselineResponse, err := baselineServices.StartRun(context.Background(), RunRequest{
		Instruction: "读取当前仓库并给出一句摘要",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("baseline start run: %v", err)
	}

	cfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "policy-runtime"),
	})
	paths := newRuntimeServices(store.NewPaths(cfg.Runtime.Root))
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths.Paths)
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths.Paths, toolServices)
	services := NewServicesFromParts(cfg, paths, modelServices, agentServices, toolServices, delegationServices)
	mutatingPolicy := &mutatingContextNoopPolicy{}

	executor := agent.NewExecutor(cfg, paths, modelServices, agentServices, toolServices, delegationServices)
	executor.Policies = []policy.RuntimePolicy{mutatingPolicy}
	services.Runner = executor
	services.ModelCaller = executor

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "读取当前仓库并给出一句摘要",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run with mutating context policy: %v", err)
	}
	if normalizePolicyPipelineOutput(response.Result.Output) != normalizePolicyPipelineOutput(baselineResponse.Result.Output) {
		t.Fatalf("expected context mutations to be isolated, baseline=%q with_policy=%q", baselineResponse.Result.Output, response.Result.Output)
	}
	if mutatingPolicy.afterModelCount.Load() == 0 {
		t.Fatal("expected AfterModel hook to be called")
	}
	if !mutatingPolicy.sawIsolatedPlan.Load() {
		t.Fatal("expected plan snapshot mutations to stay isolated")
	}
	if !mutatingPolicy.sawIsolatedCurrentStep.Load() {
		t.Fatal("expected current step snapshot mutations to stay isolated")
	}
	if !mutatingPolicy.sawIsolatedEvidence.Load() {
		t.Fatal("expected working evidence snapshot mutations to stay isolated")
	}
}

func TestExecutorRejectsNoopPolicyThatMutatesAfterActionInputs(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")

	workspace := t.TempDir()
	cfg := config.LoadWithOverrides(workspace, config.Overrides{
		RuntimeRoot: filepath.Join(t.TempDir(), "policy-runtime"),
	})
	services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, toolServices *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &actionSequenceModel{
				responses: []model.Action{
					{Action: "tool", Calls: []model.ToolCall{{
						Tool:  "test.inspect",
						Input: map[string]any{"path": "notes.txt"},
					}}},
					{Action: "final", Answer: "done"},
				},
			}, nil
		}
		registry := toolruntime.NewRegistry()
		registry.Register(staticTool{
			name:   "test.inspect",
			access: toolruntime.AccessReadOnly,
			content: map[string]any{
				"nested": map[string]any{"value": "ok"},
			},
		})
		toolServices.ToolRegistry = registry
		toolServices.ToolExecutor = toolruntime.NewExecutor(registry)
	})

	executor := services.Runner.(*agent.Executor)
	executor.Policies = []policy.RuntimePolicy{&mutatingAfterActionNoopPolicy{}}
	services.ModelCaller = executor

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "读取 notes.txt 并总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err == nil || !strings.Contains(err.Error(), "mutated action/result") {
		t.Fatalf("expected after-action mutation to be rejected, got %v", err)
	}
}

var policyPipelineIDPattern = regexp.MustCompile(`\b(?:plan|step)_[0-9]{14}_[a-f0-9]+\b`)

func normalizePolicyPipelineOutput(output string) string {
	return policyPipelineIDPattern.ReplaceAllString(output, "<id>")
}
