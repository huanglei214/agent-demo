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

func TestBuildCompactsConversationHistory(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	messages := make([]harnessruntime.SessionMessage, 0, maxConversationMessages+2)
	for i := 0; i < maxConversationMessages+2; i++ {
		messages = append(messages, harnessruntime.SessionMessage{
			Role:    harnessruntime.MessageRoleUser,
			Content: "message-" + string(rune('0'+i)),
		})
	}

	modelContext := manager.Build(BuildInput{
		Task: harnessruntime.Task{
			ID:          "task_1",
			Instruction: "summarize the repo",
			Workspace:   "/workspace",
		},
		Plan: harnessruntime.Plan{
			ID:      "plan_1",
			Goal:    "summarize the repo",
			Version: 1,
		},
		Messages: messages,
	})

	if got := len(modelContext.Messages); got != maxConversationMessages {
		t.Fatalf("expected %d conversation messages, got %d", maxConversationMessages, got)
	}
	if omitted := modelContext.Metadata["conversation_omitted"]; omitted != 2 {
		t.Fatalf("expected 2 omitted messages, got %#v", omitted)
	}
	if strings.Contains(modelContext.Render(), "message-0") || strings.Contains(modelContext.Render(), "message-1") {
		t.Fatalf("expected oldest messages to be omitted, got:\n%s", modelContext.Render())
	}
}

func TestBuildSummarizesLongAssistantMessages(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	longAnswer := strings.Repeat("仓库总结", 260)
	modelContext := manager.Build(BuildInput{
		Task: harnessruntime.Task{
			ID:          "task_1",
			Instruction: "summarize the repo",
			Workspace:   "/workspace",
		},
		Plan: harnessruntime.Plan{
			ID:      "plan_1",
			Goal:    "summarize the repo",
			Version: 1,
		},
		Messages: []harnessruntime.SessionMessage{
			{
				Role:    harnessruntime.MessageRoleAssistant,
				Content: harnessruntime.MustJSON(map[string]any{"action": "final", "answer": longAnswer}),
			},
		},
	})

	rendered := modelContext.Render()
	if strings.Contains(rendered, `"action":"final"`) {
		t.Fatalf("expected assistant final wrapper to be removed, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[truncated ") {
		t.Fatalf("expected long assistant message to be summarized, got:\n%s", rendered)
	}
}
