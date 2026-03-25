package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/huanglei214/agent-demo/internal/config"
)

func TestStartRunWithDelegationCreatesChildRunArtifacts(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	services := NewServices(config.Load(workspace))
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
