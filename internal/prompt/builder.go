package prompt

import (
	"fmt"
	"strings"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type Prompt struct {
	System   string         `json:"system"`
	Input    string         `json:"input"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Builder struct {
	templates templates
}

func NewBuilder() Builder {
	return Builder{templates: defaultTemplates()}
}

func (b Builder) BuildRunPrompt(task harnessruntime.Task, plan harnessruntime.Plan, currentStep *harnessruntime.PlanStep, modelContext harnesscontext.ModelContext, tools []map[string]string) Prompt {
	sections := []string{
		b.templates.base,
		b.templates.defaultRole,
		b.templates.taskGuidance,
		renderToolingLayer(tools),
	}

	inputParts := []string{
		fmt.Sprintf("User instruction:\n%s", task.Instruction),
		fmt.Sprintf("Workspace:\n%s", task.Workspace),
		modelContext.Render(),
	}
	if currentStep != nil {
		inputParts = append(inputParts, fmt.Sprintf("Current step:\n- id=%s\n- title=%s\n- status=%s", currentStep.ID, currentStep.Title, currentStep.Status))
	}

	return Prompt{
		System: strings.Join(sections, "\n\n"),
		Input:  strings.TrimSpace(strings.Join(inputParts, "\n\n")),
		Metadata: map[string]any{
			"task_id":      task.ID,
			"plan_id":      plan.ID,
			"plan_version": plan.Version,
			"role":         "default-agent",
			"layers":       []string{"base", "role", "task", "tooling"},
			"tool_count":   len(tools),
		},
	}
}

func (b Builder) BuildFollowUpPrompt(task harnessruntime.Task, toolName string, toolResult map[string]any, summaries []harnessruntime.Summary) Prompt {
	systemPrompt := strings.Join([]string{
		b.templates.base,
		b.templates.defaultRole,
		`Follow-up rule:
You have already received a tool result and now must answer the user.
You MUST return valid JSON only in this shape:
{"action":"final","answer":"..."}`,
	}, "\n\n")

	inputParts := []string{
		"Original instruction:\n" + task.Instruction,
		"Tool used:\n" + toolName,
		"Tool result:\n" + harnessruntime.MustJSON(toolResult),
	}
	if len(summaries) > 0 {
		parts := make([]string, 0, len(summaries))
		for _, summary := range summaries {
			parts = append(parts, summary.Scope+": "+summary.Content)
		}
		inputParts = append(inputParts, "Available summaries:\n"+strings.Join(parts, "\n\n"))
	}

	return Prompt{
		System: systemPrompt,
		Input:  strings.Join(inputParts, "\n\n"),
		Metadata: map[string]any{
			"task_id":       task.ID,
			"tool":          toolName,
			"layers":        []string{"base", "role", "task"},
			"summary_count": len(summaries),
		},
	}
}

func renderToolingLayer(tools []map[string]string) string {
	lines := []string{
		`Tooling rules:
- You may only call tools from the provided tool list.
- Tool input must be valid JSON.
- Do not invent tool names or arguments.
Available tools:`,
	}
	for _, tool := range tools {
		if access := strings.TrimSpace(tool["access"]); access != "" {
			lines = append(lines, fmt.Sprintf("- %s (%s): %s", tool["name"], access, tool["description"]))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", tool["name"], tool["description"]))
	}
	return strings.Join(lines, "\n")
}
