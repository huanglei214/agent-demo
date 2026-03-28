package prompt

import (
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
)

type Service interface {
	BuildRunPrompt(role harnessruntime.RunRole, task harnessruntime.Task, plan harnessruntime.Plan, currentStep *harnessruntime.PlanStep, modelContext harnesscontext.ModelContext, tools []map[string]string, activeSkill *skill.Definition) Prompt
	BuildFollowUpPrompt(role harnessruntime.RunRole, task harnessruntime.Task, toolResults []harnessruntime.ToolCallResult, workingEvidence map[string]any, tools []map[string]string, activeSkill *skill.Definition) Prompt
	BuildForcedFinalPrompt(role harnessruntime.RunRole, task harnessruntime.Task, reason string, evidence map[string]any, tools []map[string]string, activeSkill *skill.Definition) Prompt
}
