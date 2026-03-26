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
