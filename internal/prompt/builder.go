package prompt

import (
	"encoding/json"
	"fmt"
	"sort"
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

func (b Builder) BuildRunPrompt(role harnessruntime.RunRole, task harnessruntime.Task, plan harnessruntime.Plan, currentStep *harnessruntime.PlanStep, modelContext harnesscontext.ModelContext, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.roleLayer(role),
		b.taskGuidance(role),
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))

	inputParts := b.runInputParts(role, task, modelContext)
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
			"role":         string(normalizeRole(role)),
			"layers":       promptLayers(activeSkill != nil),
			"tool_count":   len(tools),
			"skill":        activeSkillName(activeSkill),
		},
	}
}

func (b Builder) BuildFollowUpPrompt(role harnessruntime.RunRole, task harnessruntime.Task, toolResults []harnessruntime.ToolCallResult, workingEvidence map[string]any, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.roleLayer(role),
		b.followUpRule(role),
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))
	systemPrompt := strings.Join(sections, "\n\n")

	inputParts := []string{}
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		inputParts = append(inputParts, renderDelegatedTaskInput(task))
	default:
		inputParts = append(inputParts, "Original instruction:\n"+task.Instruction)
	}
	inputParts = append(inputParts, "New tool results:\n"+harnessruntime.MustJSON(summarizeToolResultsForPrompt(toolResults)))
	if len(workingEvidence) > 0 {
		inputParts = append(inputParts, "Working evidence:\n"+harnessruntime.MustJSON(workingEvidence))
	}

	return Prompt{
		System: systemPrompt,
		Input:  strings.Join(inputParts, "\n\n"),
		Metadata: map[string]any{
			"task_id":       task.ID,
			"role":          string(normalizeRole(role)),
			"layers":        promptLayers(activeSkill != nil),
			"skill":         activeSkillName(activeSkill),
			"tool_count":    len(tools),
			"new_tool_count": len(toolResults),
		},
	}
}

func (b Builder) BuildForcedFinalPrompt(role harnessruntime.RunRole, task harnessruntime.Task, reason string, evidence map[string]any, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.roleLayer(role),
		b.forcedFinalRule(role),
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))
	systemPrompt := strings.Join(sections, "\n\n")

	inputParts := []string{}
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		inputParts = append(inputParts, renderDelegatedTaskInput(task))
	default:
		inputParts = append(inputParts, "Original instruction:\n"+task.Instruction)
	}
	if strings.TrimSpace(reason) != "" {
		inputParts = append(inputParts, "Why you must answer now:\n"+reason)
	}
	inputParts = append(inputParts, "Retrieved evidence:\n"+harnessruntime.MustJSON(evidence))

	return Prompt{
		System: systemPrompt,
		Input:  strings.Join(inputParts, "\n\n"),
		Metadata: map[string]any{
			"task_id":       task.ID,
			"role":          string(normalizeRole(role)),
			"layers":        promptLayers(activeSkill != nil),
			"skill":         activeSkillName(activeSkill),
			"tool_count":    len(tools),
			"forced_final":  true,
		},
	}
}

func summarizeToolResultsForPrompt(toolResults []harnessruntime.ToolCallResult) []map[string]any {
	if len(toolResults) == 0 {
		return nil
	}
	summaries := make([]map[string]any, 0, len(toolResults))
	for _, toolResult := range toolResults {
		summaries = append(summaries, map[string]any{
			"tool_call_id": toolResult.ToolCallID,
			"tool":         toolResult.Tool,
			"input":        toolResult.Input,
			"result":       summarizeToolResultForPrompt(toolResult.Tool, toolResult.Result),
		})
	}
	return summaries
}

func BuildWorkingEvidenceForPrompt(toolResults []harnessruntime.ToolCallResult) map[string]any {
	if len(toolResults) == 0 {
		return nil
	}
	grouped := map[string][]map[string]any{}
	for _, toolResult := range toolResults {
		grouped[toolResult.Tool] = append(grouped[toolResult.Tool], map[string]any{
			"tool_call_id": toolResult.ToolCallID,
			"input":        toolResult.Input,
			"result":       summarizeToolResultForPrompt(toolResult.Tool, toolResult.Result),
		})
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := map[string]any{}
	for _, key := range keys {
		ordered[key] = grouped[key]
	}
	return ordered
}

func (b Builder) runInputParts(role harnessruntime.RunRole, task harnessruntime.Task, modelContext harnesscontext.ModelContext) []string {
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		return []string{
			renderDelegatedTaskInput(task),
			fmt.Sprintf("Workspace:\n%s", task.Workspace),
		}
	default:
		return []string{
			fmt.Sprintf("User instruction:\n%s", task.Instruction),
			fmt.Sprintf("Workspace:\n%s", task.Workspace),
			modelContext.Render(),
		}
	}
}

func renderDelegatedTaskInput(task harnessruntime.Task) string {
	if task.Metadata == nil || task.Metadata["delegated"] != "true" {
		return "Delegated task:\n- goal: " + task.Instruction
	}

	var builder strings.Builder
	builder.WriteString("Delegated task:\n")
	builder.WriteString("- goal: ")
	builder.WriteString(task.Instruction)
	builder.WriteString("\n")

	if tools := decodeStringSliceMetadata(task.Metadata["delegated_allowed_tools"]); len(tools) > 0 {
		builder.WriteString("- allowed_tools:\n")
		for _, tool := range tools {
			builder.WriteString("  - ")
			builder.WriteString(tool)
			builder.WriteString("\n")
		}
	}
	if constraints := decodeStringSliceMetadata(task.Metadata["delegated_constraints"]); len(constraints) > 0 {
		builder.WriteString("- constraints:\n")
		for _, constraint := range constraints {
			builder.WriteString("  - ")
			builder.WriteString(constraint)
			builder.WriteString("\n")
		}
	}
	if criteria := decodeStringSliceMetadata(task.Metadata["delegated_completion_criteria"]); len(criteria) > 0 {
		builder.WriteString("- completion_criteria:\n")
		for _, criterion := range criteria {
			builder.WriteString("  - ")
			builder.WriteString(criterion)
			builder.WriteString("\n")
		}
	}
	if contextItems := decodeStringSliceMetadata(task.Metadata["delegated_task_local_context"]); len(contextItems) > 0 {
		builder.WriteString("Relevant task context:\n")
		for _, item := range contextItems {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

func decodeStringSliceMetadata(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return nil
}

func (b Builder) roleLayer(role harnessruntime.RunRole) string {
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		return b.templates.subagentRole
	default:
		return b.templates.leadRole
	}
}

func (b Builder) followUpRule(role harnessruntime.RunRole) string {
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		return b.templates.subagentFollowUpRule
	default:
		return b.templates.leadFollowUpRule
	}
}

func (b Builder) forcedFinalRule(role harnessruntime.RunRole) string {
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		return b.templates.subagentForcedFinalRule
	default:
		return b.templates.leadForcedFinalRule
	}
}

func (b Builder) taskGuidance(role harnessruntime.RunRole) string {
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		return b.templates.subagentTaskGuidance
	default:
		return b.templates.leadTaskGuidance
	}
}

func normalizeRole(role harnessruntime.RunRole) harnessruntime.RunRole {
	if strings.TrimSpace(string(role)) == "" {
		return harnessruntime.RunRoleLead
	}
	return role
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
