package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestStartRunWithDelegationCreatesChildRunArtifacts(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := NewServices(config.Load(workspace))
	configureDelegationWithoutReplan(t, &services)
	response, err := services.StartRun(RunRequest{
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
	if children[0].Result.Summary == "" {
		t.Fatalf("expected structured child summary, got %#v", children[0].Result)
	}

	events, err := services.ReplayRun(response.Run.ID)
	if err != nil {
		t.Fatalf("replay delegated run: %v", err)
	}
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
	response, err := services.StartRun(RunRequest{
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

	services := NewServices(config.Load(workspace))
	configureDelegationWithoutReplan(t, &services)
	response, err := services.StartRun(RunRequest{
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

	if decision := decideChildReplan(harnessruntime.DelegationResult{}); decision.ShouldReplan {
		t.Fatalf("expected empty child result not to trigger replan, got %#v", decision)
	}

	decision := decideChildReplan(harnessruntime.DelegationResult{
		NeedsReplan: true,
		Summary:     "child discovered new constraints",
	})
	if !decision.ShouldReplan || decision.Reason != replanReasonChildRequested {
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

func configureDelegationWithoutReplan(t *testing.T, services *Services) {
	t.Helper()

	factoryCalls := 0
	services.ModelFactory = func() (model.Model, error) {
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
