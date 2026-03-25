package prompt

import (
	"strings"
	"testing"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestBuildRunPromptIncludesFourLayers(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize README",
		Workspace:   "/workspace",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    "summarize README",
		Version: 1,
	}
	step := harnessruntime.PlanStep{
		ID:     "step_1",
		Title:  "Read README",
		Status: harnessruntime.StepRunning,
	}
	modelContext := harnesscontext.ModelContext{
		Pinned: []harnesscontext.Item{{Title: "Goal", Content: "summarize README"}},
	}

	prompt := builder.BuildRunPrompt(task, plan, &step, modelContext, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	})

	for _, fragment := range []string{
		"You are operating inside a local Go-based agent harness.",
		"Role: default-agent.",
		"Task handling rules:",
		"Tooling rules:",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected system prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}

	if prompt.Metadata["role"] != "default-agent" {
		t.Fatalf("unexpected metadata: %#v", prompt.Metadata)
	}
}

func TestBuildFollowUpPromptIncludesSummaries(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize README",
	}

	prompt := builder.BuildFollowUpPrompt(task, "fs.read_file", map[string]any{
		"path": "README.md",
	}, []harnessruntime.Summary{
		{Scope: "run", Content: "summary content"},
	})

	if !strings.Contains(prompt.Input, "Available summaries:") {
		t.Fatalf("expected follow-up prompt to include summaries, got:\n%s", prompt.Input)
	}
	if prompt.Metadata["summary_count"] != 1 {
		t.Fatalf("unexpected metadata: %#v", prompt.Metadata)
	}
}
