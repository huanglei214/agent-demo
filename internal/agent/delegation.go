package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	promptpkg "github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
)

func (e *Executor) handleDelegationAction(runCtx context.Context, exec *runExecution, action model.Action) (model.Action, error) {
	outcome, err := e.runPoliciesAfterModel(runCtx, exec, &action, map[policy.PolicyDecision]struct{}{
		policy.DecisionBlock: {},
	}, nil)
	if err != nil {
		return model.Action{}, err
	}
	if outcome != nil && outcome.Decision == policy.DecisionBlock {
		if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "subagent.rejected", "delegation", map[string]any{
			"step_id": exec.currentStep.ID,
			"reason":  outcome.Reason,
		}), exec.observer); err != nil {
			return model.Action{}, err
		}
		return model.Action{}, e.failOnly(exec, errors.New("delegation rejected: "+outcome.Reason), exec.nextSequence())
	}

	delegationGoal := strings.TrimSpace(action.DelegationGoal)
	if delegationGoal == "" {
		delegationGoal = exec.currentStep.Description
	}
	delegationTask := e.DelegationManager.BuildTask(exec.run, exec.plan, *exec.currentStep, delegationGoal, exec.recalledMemories, exec.summaries)
	childResponse, childResult, err := e.spawnChildRun(runCtx, exec.task, exec.session, exec.run, delegationTask)
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	actionOutcome, err := e.runPoliciesAfterAction(runCtx, exec, action, policy.ActionResult{
		Kind:    policy.ActionResultDelegation,
		Success: true,
		Delegation: &harnessruntime.DelegationResult{
			ChildRunID:      childResult.ChildRunID,
			Summary:         childResult.Summary,
			Artifacts:       append([]harnessruntime.DelegationArtifact{}, childResult.Artifacts...),
			Findings:        append([]string{}, childResult.Findings...),
			Risks:           append([]string{}, childResult.Risks...),
			Recommendations: append([]string{}, childResult.Recommendations...),
			NeedsReplan:     childResult.NeedsReplan,
		},
	}, map[policy.PolicyDecision]struct{}{policy.DecisionReplan: {}}, nil)
	if err != nil {
		return model.Action{}, err
	}

	start := exec.reserveSequences(2)
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, start, "subagent.spawned", "delegation", map[string]any{
		"child_run_id": childResponse.Run.ID,
		"step_id":      exec.currentStep.ID,
		"role":         childResponse.Run.Role,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+1, "subagent.completed", "delegation", map[string]any{
		"child_run_id":    childResponse.Run.ID,
		"needs_replan":    childResult.NeedsReplan,
		"summary":         childResult.Summary,
		"recommendations": childResult.Recommendations,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	if actionOutcome != nil && actionOutcome.Decision == policy.DecisionReplan {
		replanned, err := e.Planner.Replan(runCtx, planner.ReplanInput{
			RunID:    exec.run.ID,
			Goal:     exec.task.Instruction,
			Previous: exec.plan,
			Reason:   actionOutcome.Reason,
		})
		if err != nil {
			return model.Action{}, err
		}
		currentStatus := exec.currentStep.Status
		exec.plan = replanned
		if len(exec.plan.Steps) == 0 {
			return model.Action{}, errors.New("replan returned an empty plan")
		}
		exec.currentStep = &exec.plan.Steps[0]
		exec.currentStep.Status = currentStatus
		exec.plan.UpdatedAt = time.Now()
		if err := e.StateStore.SavePlan(exec.plan); err != nil {
			return model.Action{}, err
		}
		if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "plan.updated", "planner", map[string]any{
			"plan_id": exec.plan.ID,
			"version": exec.plan.Version,
			"reason":  actionOutcome.Reason,
			"step_id": exec.currentStep.ID,
		}), exec.observer); err != nil {
			return model.Action{}, err
		}
	}

	delegationEvidence := mergeWorkingEvidence(exec.workingEvidence, promptpkg.BuildWorkingEvidenceForPrompt([]harnessruntime.ToolCallResult{{
		ToolCallID: childResponse.Run.ID,
		Tool:       "subagent",
		Input:      map[string]any{"child_run_id": childResponse.Run.ID},
		Result:     delegationResultContent(childResult),
	}}))
	followUpPrompt := e.PromptBuilder.BuildFollowUpPrompt(exec.run.Role, exec.task, []harnessruntime.ToolCallResult{{
		ToolCallID: childResponse.Run.ID,
		Tool:       "subagent",
		Input:      map[string]any{"child_run_id": childResponse.Run.ID},
		Result:     delegationResultContent(childResult),
	}}, delegationEvidence, e.promptToolMetadataForSkill(exec.activeSkill), exec.activeSkill)
	followUpPrompt = promptpkg.InjectTodoContext(followUpPrompt, exec.run, exec.state)
	modelSequence := exec.nextSequence()
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, modelSequence, "model.called", "runtime", map[string]any{
		"provider": exec.run.Provider,
		"model":    exec.run.Model,
		"role":     exec.run.Role,
		"phase":    "post_delegation",
		"child":    childResponse.Run.ID,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpRequest := model.Request{
		SystemPrompt: followUpPrompt.System,
		Input:        followUpPrompt.Input,
		Metadata:     followUpPrompt.Metadata,
	}
	followUpResponse, err := e.generateModelResponse(runCtx, exec, followUpRequest)
	if appendErr := e.appendModelCall(exec.run, modelSequence, "post_delegation", "subagent", followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
		return model.Action{}, appendErr
	}
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "model.responded", "model", map[string]any{
		"finish_reason": followUpResponse.FinishReason,
		"phase":         "post_delegation",
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpAction := parseAction(followUpResponse.Text)
	if err := validateActionForRole(exec.run.Role, followUpAction); err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if followUpAction.Action != "final" || strings.TrimSpace(followUpAction.Answer) == "" {
		return model.Action{}, e.failOnly(exec, errors.New("post-delegation model response did not produce a final answer"), exec.nextSequence())
	}
	if _, err := e.runPoliciesAfterModel(runCtx, exec, &followUpAction, nil, map[policy.PolicyName]struct{}{
		policy.PolicyNameDelegation: {},
	}); err != nil {
		return model.Action{}, err
	}

	exec.finalAnswer = followUpAction.Answer
	exec.turnCount++
	return followUpAction, nil
}

func (e *Executor) spawnChildRun(runCtx context.Context, parentTask harnessruntime.Task, session harnessruntime.Session, parentRun harnessruntime.Run, task harnessruntime.DelegationTask) (ExecutionResponse, harnessruntime.DelegationResult, error) {
	if parentRun.Role != harnessruntime.RunRoleLead {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, errors.New("only lead-agent can delegate child runs")
	}
	childTask := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: delegation.BuildChildInstruction(task),
		Workspace:   parentTask.Workspace,
		Metadata: map[string]string{
			"delegated":                     "true",
			"delegated_allowed_tools":       mustJSONString(task.AllowedTools),
			"delegated_constraints":         mustJSONString(task.Constraints),
			"delegated_completion_criteria": mustJSONString(task.CompletionCriteria),
			"delegated_task_local_context":  mustJSONString(task.TaskLocalContext),
		},
		CreatedAt: time.Now(),
	}
	childRun := harnessruntime.Run{
		ID:          harnessruntime.NewID("run"),
		TaskID:      childTask.ID,
		SessionID:   session.ID,
		ParentRunID: parentRun.ID,
		Role:        harnessruntime.RunRoleSubagent,
		Status:      harnessruntime.RunPending,
		Provider:    parentRun.Provider,
		Model:       parentRun.Model,
		MaxTurns:    3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	childPlan, err := e.Planner.CreatePlan(context.Background(), planner.PlanInput{
		RunID:     childRun.ID,
		Goal:      task.Goal,
		Workspace: parentTask.Workspace,
	})
	if err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	state := harnessruntime.RunState{
		RunID:     childRun.ID,
		TurnCount: 0,
		UpdatedAt: time.Now(),
	}

	if err := e.StateStore.SaveTask(childTask); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := e.StateStore.SaveRun(childRun); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := e.StateStore.SavePlan(childPlan); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := e.StateStore.SaveState(state); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}

	nextSequence, err := e.EventStore.NextSequence(childRun.ID)
	if err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	events := []harnessruntime.Event{
		e.newEvent(childRun, childTask.ID, session.ID, nextSequence, "task.created", "system", map[string]any{"task_id": childTask.ID}),
		e.newEvent(childRun, childTask.ID, session.ID, nextSequence+1, "run.created", "system", map[string]any{"status": childRun.Status}),
		e.newEvent(childRun, childTask.ID, session.ID, nextSequence+2, "run.role_assigned", "runtime", map[string]any{"role": childRun.Role}),
		e.newEvent(childRun, childTask.ID, session.ID, nextSequence+3, "plan.created", "planner", map[string]any{"plan_id": childPlan.ID, "version": childPlan.Version}),
	}
	if err := e.appendEvents(events, nil); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}

	initialRecord := delegation.ChildRecord{
		Task: task,
		Run:  childRun,
		Result: harnessruntime.DelegationResult{
			ChildRunID:      childRun.ID,
			Summary:         "",
			Artifacts:       []harnessruntime.DelegationArtifact{},
			Findings:        []string{},
			Risks:           []string{},
			Recommendations: []string{},
			NeedsReplan:     false,
		},
		UpdatedAt: time.Now(),
	}
	if err := e.DelegationManager.SaveChild(parentRun.ID, initialRecord); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}

	response, err := e.executeRun(runCtx, childTask, session, childRun, childPlan, state, true, nil)
	if err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	result, err := delegation.BuildResult(response.Run, response.Result)
	if err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := e.DelegationManager.SaveChild(parentRun.ID, delegation.ChildRecord{
		Task:      task,
		Run:       response.Run,
		Result:    result,
		UpdatedAt: time.Now(),
	}); err != nil {
		return ExecutionResponse{}, harnessruntime.DelegationResult{}, err
	}
	return response, result, nil
}

func BuildDelegationResult(response ExecutionResponse) (harnessruntime.DelegationResult, error) {
	return delegation.BuildResult(response.Run, response.Result)
}

func delegationResultContent(result harnessruntime.DelegationResult) map[string]any {
	return delegation.ResultContent(result)
}

func mustJSONString(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return harnessruntime.MustJSON(values)
}
