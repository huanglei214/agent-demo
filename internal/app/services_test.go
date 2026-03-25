package app

import (
	"testing"

	"github.com/huanglei214/agent-demo/internal/config"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
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
	}
}
