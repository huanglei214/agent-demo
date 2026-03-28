package service

import (
	"errors"
	"testing"

	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
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

	_, err := NewServices(cfg).ModelFactory()
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
	if !errors.Is(err, harnessruntime.ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
	}
}

func TestNewServicesFromDependenciesAcceptsInterfaceImplementations(t *testing.T) {
	t.Parallel()

	cfg := config.Load(t.TempDir())
	deps := NewDependencies(cfg)
	deps.MemoryManager = stubMemoryService{}
	deps.ContextManager = stubContextService{}
	deps.PromptBuilder = stubPromptService{}

	services := NewServicesFromDependencies(deps)
	if services.MemoryManager == nil {
		t.Fatal("expected custom memory service to be wired")
	}
	if services.ContextManager == nil {
		t.Fatal("expected custom context service to be wired")
	}
	if services.PromptBuilder == nil {
		t.Fatal("expected custom prompt service to be wired")
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
