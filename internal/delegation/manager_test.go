package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestCanDelegateAndSaveChild(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)

	run := harnessruntime.Run{ID: "run_parent", SessionID: "session_1"}
	step := harnessruntime.PlanStep{ID: "step_1", Delegatable: true}

	ok, reason := manager.CanDelegate(context.Background(), run, step)
	if !ok || reason != "" {
		t.Fatalf("expected delegation allowed, got %v %q", ok, reason)
	}

	task := manager.BuildTask(run, harnessruntime.Plan{Goal: "design harness"}, harnessruntime.PlanStep{
		ID:          "step_1",
		Title:       "Delegate task",
		Description: "bounded child task",
	}, "analyze repo", nil, nil)

	record := ChildRecord{
		Task: task,
		Run: harnessruntime.Run{
			ID:          "run_child",
			ParentRunID: run.ID,
		},
		Result: harnessruntime.DelegationResult{
			ChildRunID:      "run_child",
			Summary:         "child summary",
			Artifacts:       []harnessruntime.DelegationArtifact{},
			Findings:        []string{},
			Risks:           []string{},
			Recommendations: []string{},
			NeedsReplan:     false,
		},
		UpdatedAt: time.Now(),
	}
	if err := manager.SaveChild(run.ID, record); err != nil {
		t.Fatalf("save child: %v", err)
	}

	children, err := manager.ListChildren(run.ID)
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	if len(children) != 1 || children[0].Run.ID != "run_child" {
		t.Fatalf("unexpected children: %#v", children)
	}
}

func TestValidateToolsRejectsWriteForChild(t *testing.T) {
	t.Parallel()

	manager := NewManager(store.NewPaths(t.TempDir()))
	childRun := harnessruntime.Run{ID: "run_child", ParentRunID: "run_parent"}

	if err := manager.ValidateTools(childRun, "fs.read_file"); err != nil {
		t.Fatalf("expected read tool allowed, got %v", err)
	}
	if err := manager.ValidateTools(childRun, "fs.write_file"); err == nil {
		t.Fatal("expected write tool to be rejected for child run")
	}
}

func TestCanDelegateRejectsSubagentRun(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)
	parentRun := harnessruntime.Run{ID: "run_parent"}
	childRun := harnessruntime.Run{ID: "run_child", ParentRunID: "run_parent", Role: harnessruntime.RunRoleSubagent}
	mustWriteRun(t, paths.RunPath(parentRun.ID), parentRun)
	step := harnessruntime.PlanStep{ID: "step_1", Delegatable: true}

	ok, reason := manager.CanDelegate(context.Background(), childRun, step)
	if ok || reason != "subagent_cannot_delegate" {
		t.Fatalf("expected grandchild run to be rejected, got %v %q", ok, reason)
	}
}

func TestCanDelegateRejectsWhenActiveChildrenReachLimit(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)
	parentRun := harnessruntime.Run{ID: "run_parent"}
	step := harnessruntime.PlanStep{ID: "step_1", Delegatable: true}

	for i := 0; i < 2; i++ {
		record := ChildRecord{
			Task: harnessruntime.DelegationTask{ID: harnessruntime.NewID("delegation")},
			Run: harnessruntime.Run{
				ID:        harnessruntime.NewID("run"),
				Status:    harnessruntime.RunRunning,
				CreatedAt: time.Now(),
			},
			Result:    harnessruntime.DelegationResult{ChildRunID: "child", Summary: "", Artifacts: []harnessruntime.DelegationArtifact{}, Findings: []string{}, Risks: []string{}, Recommendations: []string{}},
			UpdatedAt: time.Now(),
		}
		if err := manager.SaveChild(parentRun.ID, record); err != nil {
			t.Fatalf("save child record: %v", err)
		}
	}

	ok, reason := manager.CanDelegate(context.Background(), parentRun, step)
	if ok || reason != "max_children_exceeded" {
		t.Fatalf("expected max children rejection, got %v %q", ok, reason)
	}
}

func TestBuildTaskDoesNotForwardParentMemoriesOrSummariesByDefault(t *testing.T) {
	t.Parallel()

	manager := NewManager(store.NewPaths(t.TempDir()))
	task := manager.BuildTask(
		harnessruntime.Run{ID: "run_parent", SessionID: "session_1"},
		harnessruntime.Plan{Goal: "analyze repo"},
		harnessruntime.PlanStep{ID: "step_1", Title: "Analyze repo", Description: "Inspect key files"},
		"分析仓库结构并返回结构化总结",
		[]harnessruntime.MemoryEntry{{Content: "用户偏好输出中文"}},
		[]harnessruntime.Summary{{Content: "父运行已经读取过 README"}},
	)

	if len(task.TaskLocalContext) != 0 {
		t.Fatalf("expected no task-local context by default, got %#v", task.TaskLocalContext)
	}
}

func mustWriteRun(t *testing.T, path string, run harnessruntime.Run) {
	t.Helper()

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write run: %v", err)
	}
}
