package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

var titleCaser = cases.Title(language.Und, cases.NoLower)

const (
	maxConversationMessages    = 6
	maxUserMessageRunes        = 600
	maxAssistantMessageRunes   = 900
	maxConversationExcerptHead = 500
	maxConversationExcerptTail = 220
)

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

	messageItems := buildConversationItems(input.Messages)
	messagesOmitted := len(input.Messages) - len(messageItems)
	if messagesOmitted < 0 {
		messagesOmitted = 0
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
			Title:   titleCaser.String(summary.Scope) + " Summary",
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
			"pinned_count":         len(pinned),
			"message_count":        len(messageItems),
			"message_source_count": len(input.Messages),
			"conversation_omitted": messagesOmitted,
			"plan_count":           len(planItems),
			"memory_count":         len(memoryItems),
			"summary_count":        len(summaryItems),
			"recent_count":         len(recentItems),
		},
	}
}

func buildConversationItems(messages []harnessruntime.SessionMessage) []Item {
	start := 0
	if len(messages) > maxConversationMessages {
		start = len(messages) - maxConversationMessages
	}

	items := make([]Item, 0, len(messages)-start)
	for _, message := range messages[start:] {
		items = append(items, Item{
			Kind:    "message",
			Title:   titleCaser.String(string(message.Role)),
			Content: summarizeSessionMessageContent(message),
		})
	}
	return items
}

func summarizeSessionMessageContent(message harnessruntime.SessionMessage) string {
	content := strings.TrimSpace(message.Content)
	if nested := unwrapFinalAnswer(content); nested != "" {
		content = nested
	}

	maxRunes := maxUserMessageRunes
	if message.Role == harnessruntime.MessageRoleAssistant {
		maxRunes = maxAssistantMessageRunes
	}
	return summarizeConversationText(content, maxRunes)
}

func unwrapFinalAnswer(content string) string {
	var payload struct {
		Action string `json:"action"`
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}
	if payload.Action != "final" {
		return ""
	}
	return strings.TrimSpace(payload.Answer)
}

func summarizeConversationText(content string, maxRunes int) string {
	if maxRunes <= 0 {
		return content
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}

	head := maxConversationExcerptHead
	if head > len(runes) {
		head = len(runes)
	}
	tail := maxConversationExcerptTail
	if tail > len(runes)-head {
		tail = len(runes) - head
	}
	if tail < 0 {
		tail = 0
	}

	var builder strings.Builder
	builder.WriteString(string(runes[:head]))
	builder.WriteString("\n...\n")
	if tail > 0 {
		builder.WriteString(string(runes[len(runes)-tail:]))
	}
	builder.WriteString(fmt.Sprintf("\n[truncated %d chars]", len(runes)-head-tail))
	return builder.String()
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
