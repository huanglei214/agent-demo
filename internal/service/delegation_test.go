package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestStartRunWithDelegationCreatesChildRunArtifacts(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := newTestServices(t, config.Load(workspace), configureDelegationWithoutReplan(t))
	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start delegated run: %v", err)
	}

	children, err := services.DelegationManager.ListChildren(response.Run.ID)
	if err != nil {
		t.Fatalf("list child runs: %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("expected exactly one child run, got %d", len(children))
	}
	if response.Run.Role != harnessruntime.RunRoleLead {
		t.Fatalf("expected parent run role %q, got %q", harnessruntime.RunRoleLead, response.Run.Role)
	}
	if children[0].Run.Role != harnessruntime.RunRoleSubagent {
		t.Fatalf("expected child run role %q, got %q", harnessruntime.RunRoleSubagent, children[0].Run.Role)
	}
	if children[0].Result.Summary == "" {
		t.Fatalf("expected structured child summary, got %#v", children[0].Result)
	}

	parentModelCalls, err := services.StateStore.LoadModelCalls(response.Run.ID)
	if err != nil {
		t.Fatalf("load parent model calls: %v", err)
	}
	if len(parentModelCalls) == 0 {
		t.Fatalf("expected parent model calls")
	}
	for _, call := range parentModelCalls {
		if got := call.Request.Metadata["role"]; got != string(harnessruntime.RunRoleLead) {
			t.Fatalf("expected parent model call role %q, got %#v", harnessruntime.RunRoleLead, got)
		}
	}

	childModelCalls, err := services.StateStore.LoadModelCalls(children[0].Run.ID)
	if err != nil {
		t.Fatalf("load child model calls: %v", err)
	}
	if len(childModelCalls) == 0 {
		t.Fatalf("expected child model calls")
	}
	for _, call := range childModelCalls {
		if got := call.Request.Metadata["role"]; got != string(harnessruntime.RunRoleSubagent) {
			t.Fatalf("expected child model call role %q, got %#v", harnessruntime.RunRoleSubagent, got)
		}
		if strings.Contains(call.Request.Input, "Conversation History:") {
			t.Fatalf("expected child model call input to exclude conversation history, got:\n%s", call.Request.Input)
		}
		if strings.Contains(call.Request.Input, "Parent goal:") {
			t.Fatalf("expected child model call input to exclude parent goal, got:\n%s", call.Request.Input)
		}
		if !strings.Contains(call.Request.Input, "Delegated task:") {
			t.Fatalf("expected child model call input to be task-scoped, got:\n%s", call.Request.Input)
		}
	}

	inspect, err := services.InspectRun(response.Run.ID)
	if err != nil {
		t.Fatalf("inspect delegated run: %v", err)
	}
	if inspect.Run.Role != harnessruntime.RunRoleLead {
		t.Fatalf("expected inspect run role %q, got %q", harnessruntime.RunRoleLead, inspect.Run.Role)
	}
	if len(inspect.ChildRuns) != 1 || inspect.ChildRuns[0].Role != harnessruntime.RunRoleSubagent {
		t.Fatalf("expected inspect child role %q, got %#v", harnessruntime.RunRoleSubagent, inspect.ChildRuns)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay delegated run: %v", err)
	}
	assertEventPresent(t, events, "run.role_assigned")
	assertEventPresent(t, events, "subagent.spawned")
	assertEventPresent(t, events, "subagent.completed")
}

func TestStartRunWithDelegationCanTriggerReplan(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := NewServices(config.Load(workspace))
	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，并在需要重新规划时告诉我",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start delegated run with replan: %v", err)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventPresent(t, events, "plan.updated")
}

func TestStartRunWithDelegationSkipsReplanWithoutSignal(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := newTestServices(t, config.Load(workspace), configureDelegationWithoutReplan(t))
	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start delegated run: %v", err)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay run: %v", err)
	}
	assertEventAbsent(t, events, "plan.updated")
}

func TestDecideChildReplanRequiresStructuredSignal(t *testing.T) {
	t.Parallel()

	if decision := planner.DecideChildReplan(harnessruntime.DelegationResult{}); decision.ShouldReplan {
		t.Fatalf("expected empty child result not to trigger replan, got %#v", decision)
	}

	decision := planner.DecideChildReplan(harnessruntime.DelegationResult{
		NeedsReplan: true,
		Summary:     "child discovered new constraints",
	})
	if !decision.ShouldReplan || decision.Reason != "child_result_requested_replan" {
		t.Fatalf("expected structured child result to request replan, got %#v", decision)
	}
}

type scriptedModel struct {
	responses []model.Response
	index     int
}

func (m *scriptedModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	if m.index >= len(m.responses) {
		return model.Response{}, nil
	}
	response := m.responses[m.index]
	m.index++
	return response, nil
}

func mustActionJSON(t *testing.T, action model.Action) string {
	t.Helper()

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	return string(data)
}

func configureDelegationWithoutReplan(t *testing.T) testServicesMutator {
	t.Helper()

	return func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		factoryCalls := 0
		modelServices.ModelFactory = func() (model.Model, error) {
			factoryCalls++
			switch factoryCalls {
			case 1:
				return &scriptedModel{responses: []model.Response{
					{
						Text: mustActionJSON(t, model.Action{
							Action:         "delegate",
							DelegationGoal: "分析当前仓库并给出简短摘要",
						}),
						FinishReason: "stop",
					},
					{
						Text: mustActionJSON(t, model.Action{
							Action: "final",
							Answer: "parent summary after delegation",
						}),
						FinishReason: "stop",
					},
				}}, nil
			case 2:
				return &scriptedModel{responses: []model.Response{
					{
						Text:         `{"summary":"Delegated child analysis completed successfully.","artifacts":[],"findings":[],"risks":[],"recommendations":[],"needs_replan":false}`,
						FinishReason: "stop",
					},
				}}, nil
			default:
				t.Fatalf("unexpected model factory call %d", factoryCalls)
				return nil, nil
			}
		}
	}
}

func TestStartRunWithDelegationRejectsUnstructuredChildResult(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		factoryCalls := 0
		modelServices.ModelFactory = func() (model.Model, error) {
			factoryCalls++
			switch factoryCalls {
			case 1:
				return &scriptedModel{responses: []model.Response{
					{
						Text: mustActionJSON(t, model.Action{
							Action:         "delegate",
							DelegationGoal: "分析当前仓库并给出简短摘要",
						}),
						FinishReason: "stop",
					},
				}}, nil
			case 2:
				return &scriptedModel{responses: []model.Response{
					{
						Text:         `不是结构化结果`,
						FinishReason: "stop",
					},
				}}, nil
			default:
				t.Fatalf("unexpected model factory call %d", factoryCalls)
				return nil, nil
			}
		}
	})

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err == nil {
		t.Fatalf("expected unstructured child result to fail")
	}
	if !strings.Contains(err.Error(), "structured result") {
		t.Fatalf("expected structured result error, got %v", err)
	}
}

func TestStartRunWithDelegationRejectsNestedDelegationFromChild(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		factoryCalls := 0
		modelServices.ModelFactory = func() (model.Model, error) {
			factoryCalls++
			switch factoryCalls {
			case 1:
				return &scriptedModel{responses: []model.Response{
					{
						Text: mustActionJSON(t, model.Action{
							Action:         "delegate",
							DelegationGoal: "分析当前仓库并给出简短摘要",
						}),
						FinishReason: "stop",
					},
				}}, nil
			case 2:
				return &scriptedModel{responses: []model.Response{
					{
						Text: mustActionJSON(t, model.Action{
							Action:         "delegate",
							DelegationGoal: "继续委派给另一个子代理",
						}),
						FinishReason: "stop",
					},
				}}, nil
			default:
				t.Fatalf("unexpected model factory call %d", factoryCalls)
				return nil, nil
			}
		}
	})

	_, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err == nil {
		t.Fatalf("expected nested child delegation to fail")
	}
	if !strings.Contains(err.Error(), "subagent cannot delegate further work") {
		t.Fatalf("expected subagent delegation guard error, got %v", err)
	}
}

func TestStartRunDoesNotInjectConversationHistoryIntoPrompt(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	services := NewServices(config.Load(workspace))

	firstRun, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "第一轮问题",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start first run: %v", err)
	}

	secondRun, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "第二轮问题",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   firstRun.Run.SessionID,
	})
	if err != nil {
		t.Fatalf("start second run: %v", err)
	}

	modelCalls, err := services.StateStore.LoadModelCalls(secondRun.Run.ID)
	if err != nil {
		t.Fatalf("load second run model calls: %v", err)
	}
	if len(modelCalls) == 0 {
		t.Fatal("expected second run model calls")
	}
	for _, call := range modelCalls {
		if strings.Contains(call.Request.Input, "Conversation History:") {
			t.Fatalf("expected conversation history to be excluded from prompts, got:\n%s", call.Request.Input)
		}
	}
}

func TestBuildDelegationResultUnwrapsFinalWrapper(t *testing.T) {
	response := RunResponse{
		Run: harnessruntime.Run{ID: "child-run"},
		Result: &harnessruntime.RunResult{
			Output: `{"action":"final","answer":"{\"summary\":\"天气查询完成\",\"artifacts\":[],\"findings\":[],\"risks\":[],\"recommendations\":[],\"needs_replan\":false}"}`,
		},
	}

	result, err := delegation.BuildResult(response.Run, response.Result)
	if err != nil {
		t.Fatalf("build delegation result: %v", err)
	}
	if result.Summary != "天气查询完成" {
		t.Fatalf("expected unwrapped structured summary, got %#v", result)
	}
}
