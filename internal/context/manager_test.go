package context

import (
	"strings"
	"testing"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestBuildOrdersContextSections(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize the repo",
		Workspace:   "/workspace",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    "summarize the repo",
		Version: 1,
		Steps: []harnessruntime.PlanStep{
			{
				ID:          "step_1",
				Title:       "Read files",
				Description: "Read important files",
				Status:      harnessruntime.StepRunning,
			},
		},
	}

	modelContext := manager.Build(BuildInput{
		Task:        task,
		Plan:        plan,
		CurrentStep: &plan.Steps[0],
		Messages: []harnessruntime.SessionMessage{
			{Role: harnessruntime.MessageRoleUser, Content: "first question"},
			{Role: harnessruntime.MessageRoleAssistant, Content: "first answer"},
		},
		RecentEvents: []harnessruntime.Event{
			{Type: "tool.succeeded", Actor: "tool", Sequence: 3},
		},
		Summaries: []harnessruntime.Summary{
			{Scope: "run", Content: "summary content"},
		},
		Memories: []harnessruntime.MemoryEntry{
			{Kind: "decision", Scope: "session", Content: "Use Hertz for HTTP."},
		},
	})

	rendered := modelContext.Render()
	for _, expected := range []string{
		"Pinned Context:",
		"Conversation History:",
		"Plan Context:",
		"Recalled Memories:",
		"Summaries:",
		"Recent Events:",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered context to contain %q, got:\n%s", expected, rendered)
		}
	}
}

func TestShouldCompactAndCompact(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	shouldCompact, reason := manager.ShouldCompact(CompactionCheckInput{
		TokenUsage:  81,
		TokenBudget: 100,
	})
	if !shouldCompact || reason != "token_threshold" {
		t.Fatalf("unexpected compaction decision: %v %s", shouldCompact, reason)
	}

	plan := harnessruntime.Plan{Goal: "inspect repo"}
	step := harnessruntime.PlanStep{Title: "Read files", Status: harnessruntime.StepRunning}
	summary, err := manager.Compact(CompactInput{
		RunID:       "run_1",
		Scope:       "step",
		Plan:        plan,
		CurrentStep: &step,
		RecentEvents: []harnessruntime.Event{
			{Type: "tool.called", Actor: "runtime"},
			{Type: "tool.succeeded", Actor: "tool"},
		},
	})
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if summary.Scope != "step" || !strings.Contains(summary.Content, "inspect repo") {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
