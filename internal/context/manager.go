package context

import (
	"fmt"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type Item struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ModelContext struct {
	Pinned    []Item         `json:"pinned"`
	Messages  []Item         `json:"messages"`
	Plan      []Item         `json:"plan"`
	Memories  []Item         `json:"memories"`
	Summaries []Item         `json:"summaries"`
	Recent    []Item         `json:"recent"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type BuildInput struct {
	Task         harnessruntime.Task
	Plan         harnessruntime.Plan
	CurrentStep  *harnessruntime.PlanStep
	RecentEvents []harnessruntime.Event
	Summaries    []harnessruntime.Summary
	Memories     []harnessruntime.MemoryEntry
	Messages     []harnessruntime.SessionMessage
}

type CompactionCheckInput struct {
	TokenUsage       int
	TokenBudget      int
	RecentEventCount int
	LastToolBytes    int
}

type CompactInput struct {
	RunID        string
	Scope        string
	Plan         harnessruntime.Plan
	CurrentStep  *harnessruntime.PlanStep
	RecentEvents []harnessruntime.Event
}

type Manager struct{}

func NewManager() Manager {
	return Manager{}
}

func (m Manager) Build(input BuildInput) ModelContext {
	pinned := []Item{
		{
			Kind:    "instruction",
			Title:   "Goal",
			Content: input.Task.Instruction,
		},
		{
			Kind:    "workspace",
			Title:   "Workspace",
			Content: input.Task.Workspace,
		},
	}

	planItems := []Item{
		{
			Kind:    "plan",
			Title:   "Plan",
			Content: fmt.Sprintf("plan_id=%s version=%d goal=%s", input.Plan.ID, input.Plan.Version, input.Plan.Goal),
		},
	}
	if input.CurrentStep != nil {
		planItems = append(planItems, Item{
			Kind:    "step",
			Title:   "Active Step",
			Content: fmt.Sprintf("step_id=%s title=%s status=%s description=%s", input.CurrentStep.ID, input.CurrentStep.Title, input.CurrentStep.Status, input.CurrentStep.Description),
		})
	}

	messageItems := make([]Item, 0, len(input.Messages))
	for _, message := range input.Messages {
		messageItems = append(messageItems, Item{
			Kind:    "message",
			Title:   strings.Title(string(message.Role)),
			Content: message.Content,
		})
	}

	memoryItems := make([]Item, 0, len(input.Memories))
	for _, entry := range input.Memories {
		memoryItems = append(memoryItems, Item{
			Kind:    "memory",
			Title:   fmt.Sprintf("%s (%s)", entry.Kind, entry.Scope),
			Content: entry.Content,
		})
	}

	summaryItems := make([]Item, 0, len(input.Summaries))
	for _, summary := range input.Summaries {
		summaryItems = append(summaryItems, Item{
			Kind:    "summary",
			Title:   strings.Title(summary.Scope) + " Summary",
			Content: summary.Content,
		})
	}

	recentItems := make([]Item, 0, len(input.RecentEvents))
	start := 0
	if len(input.RecentEvents) > 5 {
		start = len(input.RecentEvents) - 5
	}
	for _, event := range input.RecentEvents[start:] {
		recentItems = append(recentItems, Item{
			Kind:    "event",
			Title:   event.Type,
			Content: fmt.Sprintf("actor=%s sequence=%d", event.Actor, event.Sequence),
		})
	}

	return ModelContext{
		Pinned:    pinned,
		Messages:  messageItems,
		Plan:      planItems,
		Memories:  memoryItems,
		Summaries: summaryItems,
		Recent:    recentItems,
		Metadata: map[string]any{
			"pinned_count":  len(pinned),
			"message_count": len(messageItems),
			"plan_count":    len(planItems),
			"memory_count":  len(memoryItems),
			"summary_count": len(summaryItems),
			"recent_count":  len(recentItems),
		},
	}
}

func (m ModelContext) Render() string {
	sections := []struct {
		title string
		items []Item
	}{
		{title: "Pinned Context", items: m.Pinned},
		{title: "Conversation History", items: m.Messages},
		{title: "Plan Context", items: m.Plan},
		{title: "Recalled Memories", items: m.Memories},
		{title: "Summaries", items: m.Summaries},
		{title: "Recent Events", items: m.Recent},
	}

	var builder strings.Builder
	for _, section := range sections {
		if len(section.items) == 0 {
			continue
		}
		builder.WriteString(section.title)
		builder.WriteString(":\n")
		for _, item := range section.items {
			builder.WriteString("- ")
			builder.WriteString(item.Title)
			builder.WriteString(": ")
			builder.WriteString(item.Content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func (m Manager) ShouldCompact(input CompactionCheckInput) (bool, string) {
	if input.TokenBudget > 0 && input.TokenUsage >= int(float64(input.TokenBudget)*0.8) {
		return true, "token_threshold"
	}
	if input.RecentEventCount > 12 {
		return true, "recent_events_threshold"
	}
	if input.LastToolBytes > 8*1024 {
		return true, "tool_output_threshold"
	}
	return false, ""
}

func (m Manager) Compact(input CompactInput) (harnessruntime.Summary, error) {
	lines := []string{
		"Compaction summary",
		"goal: " + input.Plan.Goal,
	}
	if input.CurrentStep != nil {
		lines = append(lines, fmt.Sprintf("active_step: %s (%s)", input.CurrentStep.Title, input.CurrentStep.Status))
	}

	start := 0
	if len(input.RecentEvents) > 5 {
		start = len(input.RecentEvents) - 5
	}
	for _, event := range input.RecentEvents[start:] {
		lines = append(lines, fmt.Sprintf("event: %s by %s", event.Type, event.Actor))
	}

	return harnessruntime.Summary{
		ID:        harnessruntime.NewID("summary"),
		RunID:     input.RunID,
		Scope:     defaultScope(input.Scope),
		Content:   strings.Join(lines, "\n"),
		CreatedAt: time.Now(),
	}, nil
}

func defaultScope(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return "run"
	}
	return scope
}
