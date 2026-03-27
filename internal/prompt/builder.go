package prompt

import (
	"fmt"
	"strings"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
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

func (b Builder) BuildRunPrompt(task harnessruntime.Task, plan harnessruntime.Plan, currentStep *harnessruntime.PlanStep, modelContext harnesscontext.ModelContext, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.templates.defaultRole,
		b.templates.taskGuidance,
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))

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
			"layers":       promptLayers(activeSkill != nil),
			"tool_count":   len(tools),
			"skill":        activeSkillName(activeSkill),
		},
	}
}

func (b Builder) BuildFollowUpPrompt(task harnessruntime.Task, toolName string, toolResult map[string]any, summaries []harnessruntime.Summary, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.templates.defaultRole,
		`Follow-up rule:
You have already received a tool result.
If the result is not yet sufficient, you may call another provided tool.
If the user asked for a factual answer, do not stop at raw links or search results when a follow-up fetch can answer more directly.
If you already have a credible fetched page with a readable title or content, prefer giving the best sourced answer you can instead of searching again.
Do not repeat the same search/fetch loop on the same topic once you already have one or two usable fetched pages.
If evidence is partial but enough for a cautious answer, respond with the best supported answer and note uncertainty rather than continuing to loop.
You MUST return valid JSON only in one of these shapes:
{"action":"tool","tool":"...","input":{...}}
or
{"action":"final","answer":"..."}`,
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))
	systemPrompt := strings.Join(sections, "\n\n")

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
			"layers":        promptLayers(activeSkill != nil),
			"summary_count": len(summaries),
			"skill":         activeSkillName(activeSkill),
			"tool_count":    len(tools),
		},
	}
}

func renderSkillLayer(activeSkill *skill.Definition) string {
	if activeSkill == nil {
		return ""
	}
	lines := []string{
		fmt.Sprintf("Active skill: %s", activeSkill.Name),
	}
	if description := strings.TrimSpace(activeSkill.Description); description != "" {
		lines = append(lines, "Skill description:", description)
	}
	if instructions := strings.TrimSpace(activeSkill.Instructions); instructions != "" {
		lines = append(lines, "Skill instructions:", instructions)
	}
	return strings.Join(lines, "\n")
}

func promptLayers(hasSkill bool) []string {
	if hasSkill {
		return []string{"base", "role", "task", "skill", "tooling"}
	}
	return []string{"base", "role", "task", "tooling"}
}

func activeSkillName(activeSkill *skill.Definition) string {
	if activeSkill == nil {
		return ""
	}
	return activeSkill.Name
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
