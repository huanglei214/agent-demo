package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	promptpkg "github.com/huanglei214/agent-demo/internal/prompt"
	"github.com/huanglei214/agent-demo/internal/retrieval"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
	"github.com/huanglei214/agent-demo/internal/skill"
)

type runExecution struct {
	task                     harnessruntime.Task
	session                  harnessruntime.Session
	run                      harnessruntime.Run
	plan                     harnessruntime.Plan
	state                    harnessruntime.RunState
	currentStep              *harnessruntime.PlanStep
	activeSkill              *skill.Definition
	recalledMemories         []harnessruntime.MemoryEntry
	summaries                []harnessruntime.Summary
	retrievalProgress        retrieval.RetrievalProgress
	workingEvidence          map[string]any
	explicitMemoryCandidates []harnessruntime.MemoryCandidate
	explicitMemoryAnswer     string
	lastPromptBytes          int
	sequence                 harnessruntime.SequenceCursor
	turnCount                int
	finalAnswer              string
	observer                 RunObserver
	provider                 model.Model
	mode                     policy.ExecutionMode
}

func (e *runExecution) nextSequence() int64 {
	return e.sequence.Next()
}

func (e *runExecution) reserveSequences(count int) int64 {
	return e.sequence.Reserve(count)
}

func (e *runExecution) currentSequence() int64 {
	return e.sequence.Current()
}

func deriveExecutionMode(run harnessruntime.Run, state harnessruntime.RunState) policy.ExecutionMode {
	if run.Role == harnessruntime.RunRoleSubagent {
		return policy.ExecutionModeDelegated
	}
	if state.ResumePhase != "" {
		return policy.ExecutionModeResume
	}
	return policy.ExecutionModeStructured
}

func (e *Executor) policyContext(exec *runExecution) *policy.ExecutionContext {
	planCopy := clonePlan(exec.plan)
	currentStepCopy := clonePlanStepPtr(exec.currentStep)
	return &policy.ExecutionContext{
		Task:               cloneTask(exec.task),
		Session:            exec.session,
		Run:                exec.run,
		State:              cloneRunState(exec.state),
		Plan:               &planCopy,
		CurrentStep:        currentStepCopy,
		Mode:               exec.mode,
		Metadata:           map[string]any{},
		Memories:           cloneMemoryEntries(exec.recalledMemories),
		Summaries:          cloneSummaries(exec.summaries),
		RetrievalProgress:  cloneRetrievalProgress(exec.retrievalProgress),
		WorkingEvidence:    cloneStringAnyMap(exec.workingEvidence),
		ExplicitCandidates: cloneMemoryCandidates(exec.explicitMemoryCandidates),
		FinalAnswer:        exec.finalAnswer,
		TurnCount:          exec.turnCount,
	}
}

func (e *Executor) runPoliciesAfterModel(ctx context.Context, exec *runExecution, action *model.Action, allowed map[policy.PolicyDecision]struct{}, excluded map[policy.PolicyName]struct{}) (*policy.PolicyOutcome, error) {
	for _, p := range e.Policies {
		if excluded != nil {
			if _, ok := excluded[policy.PolicyName(p.Name())]; ok {
				continue
			}
		}
		actionCopy := cloneAction(*action)
		originalAction := cloneAction(*action)
		outcome, err := p.AfterModel(ctx, e.policyContext(exec), &actionCopy)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(actionCopy, originalAction) && !policy.HasEffect(outcome) {
			return nil, errors.New("policy mutated action copy without effect")
		}
		if policy.HasEffect(outcome) {
			if _, ok := allowed[outcome.Decision]; ok {
				return outcome, nil
			}
			return nil, fmt.Errorf("unexpected policy outcome %q after model before implementation", outcome.Decision)
		}
	}
	return nil, nil
}

func clonePlan(plan harnessruntime.Plan) harnessruntime.Plan {
	clone := plan
	clone.Steps = make([]harnessruntime.PlanStep, len(plan.Steps))
	for i, step := range plan.Steps {
		clone.Steps[i] = clonePlanStep(step)
	}
	return clone
}

func cloneTask(task harnessruntime.Task) harnessruntime.Task {
	clone := task
	if len(task.Metadata) == 0 {
		clone.Metadata = nil
		return clone
	}
	clone.Metadata = make(map[string]string, len(task.Metadata))
	for key, value := range task.Metadata {
		clone.Metadata[key] = value
	}
	return clone
}

func cloneRunState(state harnessruntime.RunState) harnessruntime.RunState {
	clone := state
	clone.PendingToolResult = cloneStringAnyMap(state.PendingToolResult)
	clone.PendingToolResults = cloneToolCallResults(state.PendingToolResults)
	clone.Todos = cloneTodoItems(state.Todos)
	return clone
}

func clonePlanStep(step harnessruntime.PlanStep) harnessruntime.PlanStep {
	clone := step
	clone.Dependencies = append([]string(nil), step.Dependencies...)
	return clone
}

func clonePlanStepPtr(step *harnessruntime.PlanStep) *harnessruntime.PlanStep {
	if step == nil {
		return nil
	}
	clone := clonePlanStep(*step)
	return &clone
}

func cloneMemoryEntries(entries []harnessruntime.MemoryEntry) []harnessruntime.MemoryEntry {
	clone := make([]harnessruntime.MemoryEntry, len(entries))
	for i, entry := range entries {
		clone[i] = entry
		clone[i].Tags = append([]string(nil), entry.Tags...)
	}
	return clone
}

func cloneMemoryCandidates(candidates []harnessruntime.MemoryCandidate) []harnessruntime.MemoryCandidate {
	clone := make([]harnessruntime.MemoryCandidate, len(candidates))
	for i, candidate := range candidates {
		clone[i] = candidate
		clone[i].Tags = append([]string(nil), candidate.Tags...)
	}
	return clone
}

func cloneSummaries(summaries []harnessruntime.Summary) []harnessruntime.Summary {
	return append([]harnessruntime.Summary(nil), summaries...)
}

func cloneRetrievalProgress(progress retrieval.RetrievalProgress) retrieval.RetrievalProgress {
	clone := progress
	clone.SearchQueries = append([]string(nil), progress.SearchQueries...)
	clone.FetchedURLs = append([]string(nil), progress.FetchedURLs...)
	clone.Evidence = append([]retrieval.RetrievalEvidence(nil), progress.Evidence...)
	return clone
}

func cloneAction(action model.Action) model.Action {
	clone := action
	clone.Calls = cloneToolCalls(action.Calls)
	if action.Subtask != nil {
		subtask := *action.Subtask
		clone.Subtask = &subtask
	}
	if action.Todo != nil {
		clone.Todo = &model.TodoAction{
			Operation: action.Todo.Operation,
			Items:     cloneTodoItems(action.Todo.Items),
		}
	}
	return clone
}

func cloneTodoItems(items []harnessruntime.TodoItem) []harnessruntime.TodoItem {
	if len(items) == 0 {
		return nil
	}
	clone := make([]harnessruntime.TodoItem, len(items))
	copy(clone, items)
	return clone
}

func cloneToolCalls(calls []model.ToolCall) []model.ToolCall {
	clone := make([]model.ToolCall, len(calls))
	for i, call := range calls {
		clone[i] = model.ToolCall{
			Tool:  call.Tool,
			Input: cloneStringAnyMap(call.Input),
		}
	}
	return clone
}

func cloneToolCallResults(results []harnessruntime.ToolCallResult) []harnessruntime.ToolCallResult {
	clone := make([]harnessruntime.ToolCallResult, len(results))
	for i, result := range results {
		clone[i] = harnessruntime.ToolCallResult{
			ToolCallID: result.ToolCallID,
			Tool:       result.Tool,
			Input:      cloneStringAnyMap(result.Input),
			Result:     cloneStringAnyMap(result.Result),
		}
	}
	return clone
}

func cloneDelegationResult(result *harnessruntime.DelegationResult) *harnessruntime.DelegationResult {
	if result == nil {
		return nil
	}
	clone := *result
	clone.Artifacts = make([]harnessruntime.DelegationArtifact, len(result.Artifacts))
	for i, artifact := range result.Artifacts {
		clone.Artifacts[i] = harnessruntime.DelegationArtifact{
			Value: artifact.Value,
			Name:  artifact.Name,
			Path:  artifact.Path,
			URL:   artifact.URL,
			Extra: cloneStringAnyMap(artifact.Extra),
		}
	}
	clone.Findings = append([]string(nil), result.Findings...)
	clone.Risks = append([]string(nil), result.Risks...)
	clone.Recommendations = append([]string(nil), result.Recommendations...)
	return &clone
}

func cloneActionResult(result policy.ActionResult) policy.ActionResult {
	clone := result
	clone.ToolCalls = cloneToolCalls(result.ToolCalls)
	clone.ToolResults = cloneToolCallResults(result.ToolResults)
	clone.Delegation = cloneDelegationResult(result.Delegation)
	clone.Metadata = cloneStringAnyMap(result.Metadata)
	return clone
}

func cloneStringAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	clone := make(map[string]any, len(input))
	for key, value := range input {
		clone[key] = cloneAny(value)
	}
	return clone
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneStringAnyMap(typed)
	case []any:
		clone := make([]any, len(typed))
		for i, item := range typed {
			clone[i] = cloneAny(item)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return typed
	}
}

func (e *Executor) runPoliciesAfterAction(ctx context.Context, exec *runExecution, action model.Action, result policy.ActionResult, allowed map[policy.PolicyDecision]struct{}, excluded map[policy.PolicyName]struct{}) (*policy.PolicyOutcome, error) {
	for _, p := range e.Policies {
		if excluded != nil {
			if _, ok := excluded[policy.PolicyName(p.Name())]; ok {
				continue
			}
		}
		actionCopy := cloneAction(action)
		resultCopy := cloneActionResult(result)
		originalAction := cloneAction(action)
		originalResult := cloneActionResult(result)
		outcome, err := p.AfterAction(ctx, e.policyContext(exec), actionCopy, resultCopy)
		if err != nil {
			return nil, err
		}
		if (!reflect.DeepEqual(actionCopy, originalAction) || !reflect.DeepEqual(resultCopy, originalResult)) && !policy.HasEffect(outcome) {
			return nil, errors.New("policy mutated action/result copy without effect")
		}
		if policy.HasEffect(outcome) {
			if _, ok := allowed[outcome.Decision]; ok {
				return outcome, nil
			}
			return nil, fmt.Errorf("unexpected policy outcome %q after action before implementation", outcome.Decision)
		}
	}
	return nil, nil
}

func (e *Executor) ExecuteRun(ctx context.Context, task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (ExecutionResponse, error) {
	observer = ensureRunObserver(observer)
	if ctx == nil {
		ctx = context.Background()
	}
	if len(plan.Steps) == 0 {
		return ExecutionResponse{}, errors.New("plan has no steps to execute")
	}
	activeSkill, err := e.activeSkillForTask(task)
	if err != nil {
		return ExecutionResponse{}, err
	}

	currentStep := &plan.Steps[0]
	if state.CurrentStepID != "" {
		for i := range plan.Steps {
			if plan.Steps[i].ID == state.CurrentStepID {
				currentStep = &plan.Steps[i]
				break
			}
		}
	}

	nextSequence, err := e.EventStore.NextSequence(run.ID)
	if err != nil {
		return ExecutionResponse{}, err
	}

	exec := &runExecution{
		task:        task,
		session:     session,
		run:         run,
		plan:        plan,
		state:       state,
		currentStep: currentStep,
		activeSkill: activeSkill,
		sequence:    harnessruntime.NewSequenceCursor(nextSequence),
		observer:    observer,
		mode:        deriveExecutionMode(run, state),
	}

	if err := e.runPoliciesBeforeRun(ctx, exec); err != nil {
		return ExecutionResponse{}, err
	}

	if activate {
		if err := e.activateRun(exec); err != nil {
			return ExecutionResponse{}, err
		}
	}
	if err := e.loadExecutionContext(exec); err != nil {
		return ExecutionResponse{}, err
	}

	provider, err := e.ModelFactory()
	if err != nil {
		return e.failRun(exec.run, exec.plan, exec.task.ID, exec.session.ID, exec.state, err, exec.nextSequence(), exec.observer)
	}
	exec.provider = provider

	runCtx := ctx
	action, err := e.resolveInitialAction(runCtx, exec, activate)
	if err != nil {
		return ExecutionResponse{}, err
	}
	for {
		switch action.Action {
		case "todo":
			action, err = e.handleTodoAction(runCtx, exec, action)
			if err != nil {
				return ExecutionResponse{}, err
			}
			continue
		case "tool":
			action, err = e.dispatchToolActions(runCtx, exec, action)
			if err != nil {
				return ExecutionResponse{}, err
			}
			continue
		case "delegate":
			action, err = e.handleDelegationAction(runCtx, exec, action)
			if err != nil {
				return ExecutionResponse{}, err
			}
			continue
		case "final":
			if _, err := e.runPoliciesAfterAction(runCtx, exec, action, policy.ActionResult{
				Kind:    policy.ActionResultFinal,
				Success: true,
			}, nil, nil); err != nil {
				return ExecutionResponse{}, err
			}
			if strings.TrimSpace(exec.finalAnswer) == "" {
				return e.failRun(exec.run, exec.plan, exec.task.ID, exec.session.ID, exec.state, errors.New("model returned an empty final answer"), exec.nextSequence(), exec.observer)
			}
			return e.completeRun(exec)
		default:
			return e.failRun(exec.run, exec.plan, exec.task.ID, exec.session.ID, exec.state, errors.New("model returned an unsupported action"), exec.nextSequence(), exec.observer)
		}
	}
}

func (e *Executor) activateRun(exec *runExecution) error {
	exec.run.Status = harnessruntime.RunRunning
	exec.run.CurrentStepID = exec.currentStep.ID
	exec.run.UpdatedAt = time.Now()
	exec.currentStep.Status = harnessruntime.StepRunning
	exec.plan.UpdatedAt = exec.run.UpdatedAt
	exec.state.CurrentStepID = exec.currentStep.ID
	exec.state.UpdatedAt = exec.run.UpdatedAt

	if err := e.StateStore.SaveRun(exec.run); err != nil {
		return err
	}
	if err := e.StateStore.SavePlan(exec.plan); err != nil {
		return err
	}
	if err := e.StateStore.SaveState(exec.state); err != nil {
		return err
	}

	start := exec.reserveSequences(3)
	lifecycleEvents := []harnessruntime.Event{
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start, "run.status_changed", "runtime", map[string]any{"from": harnessruntime.RunPending, "to": harnessruntime.RunRunning}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+1, "run.started", "runtime", map[string]any{"run_id": exec.run.ID}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+2, "plan.step.started", "runtime", map[string]any{"step_id": exec.currentStep.ID}),
	}
	if err := e.appendEvents(lifecycleEvents, exec.observer); err != nil {
		return err
	}
	return nil
}

func (e *Executor) loadExecutionContext(exec *runExecution) error {
	recentEvents, err := e.EventStore.ReadAll(exec.run.ID)
	if err != nil {
		return err
	}
	exec.retrievalProgress = retrieval.BuildRetrievalProgress(recentEvents)
	exec.workingEvidence = promptpkg.BuildWorkingEvidenceForPrompt(collectSuccessfulToolResults(recentEvents))

	recalledMemories, err := e.MemoryManager.Recall(memory.RecallQuery{
		SessionID: exec.session.ID,
		Goal:      exec.task.Instruction,
		Limit:     5,
	})
	if err != nil {
		return err
	}
	if exec.run.Role == harnessruntime.RunRoleSubagent {
		recalledMemories = nil
	}
	exec.recalledMemories = recalledMemories

	summaries, err := e.StateStore.LoadSummaries(exec.run.ID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	exec.summaries = summaries

	return e.StateStore.SaveRunMemories(harnessruntime.RunMemories{
		RunID:     exec.run.ID,
		Recalled:  exec.recalledMemories,
		UpdatedAt: time.Now(),
	})
}

func (e *Executor) resolveInitialAction(runCtx context.Context, exec *runExecution, activate bool) (model.Action, error) {
	recentMessages := []harnessruntime.SessionMessage{}
	recentEvents, err := e.EventStore.ReadAll(exec.run.ID)
	if err != nil {
		return model.Action{}, err
	}
	modelContext := e.ContextManager.Build(harnesscontext.BuildInput{
		Task:         exec.task,
		Plan:         exec.plan,
		CurrentStep:  exec.currentStep,
		RecentEvents: recentEvents,
		Summaries:    exec.summaries,
		Memories:     exec.recalledMemories,
		Messages:     recentMessages,
	})
	runPrompt := e.PromptBuilder.BuildRunPrompt(exec.run.Role, exec.task, exec.plan, exec.currentStep, modelContext, e.promptToolMetadataForSkill(exec.activeSkill), exec.activeSkill)
	runPrompt = promptpkg.InjectTodoContext(runPrompt, exec.run, exec.state)
	exec.lastPromptBytes = len(runPrompt.System) + len(runPrompt.Input)
	explicitMemoryCandidates, explicitMemoryAnswer, routedToMemory := e.MemoryManager.DetectExplicitRemember(memory.ExplicitRememberInput{
		SessionID:   exec.session.ID,
		RunID:       exec.run.ID,
		Instruction: exec.task.Instruction,
	})
	exec.explicitMemoryCandidates = explicitMemoryCandidates
	exec.explicitMemoryAnswer = explicitMemoryAnswer

	start := exec.reserveSequences(3)
	preModelEvents := []harnessruntime.Event{
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start, "memory.recalled", "memory", map[string]any{"count": len(exec.recalledMemories)}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+1, "prompt.built", "prompt", runPrompt.Metadata),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+2, "context.built", "context", map[string]any{
			"task_id":       exec.task.ID,
			"plan_id":       exec.plan.ID,
			"current_step":  exec.currentStep.ID,
			"message_count": len(recentMessages),
			"memory_count":  len(exec.recalledMemories),
			"summary_count": len(exec.summaries),
			"recent_count":  len(modelContext.Recent),
		}),
	}
	if err := e.appendEvents(preModelEvents, exec.observer); err != nil {
		return model.Action{}, err
	}

	if !activate && exec.state.ResumePhase == "post_tool" && hasPendingToolResults(exec.state) {
		return e.resumePostToolAction(runCtx, exec)
	}
	if routedToMemory {
		if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "memory.routed", "memory", map[string]any{
			"count": len(exec.explicitMemoryCandidates),
		}), exec.observer); err != nil {
			return model.Action{}, err
		}
		exec.finalAnswer = exec.explicitMemoryAnswer
		return model.Action{Action: "final", Answer: exec.finalAnswer}, nil
	}

	modelSequence := exec.nextSequence()
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, modelSequence, "model.called", "runtime", map[string]any{
		"provider": exec.run.Provider,
		"model":    exec.run.Model,
		"role":     exec.run.Role,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	modelRequest := model.Request{
		SystemPrompt: runPrompt.System,
		Input:        runPrompt.Input,
		Metadata:     runPrompt.Metadata,
	}
	modelResponse, err := e.generateModelResponse(runCtx, exec, modelRequest)
	if appendErr := e.appendModelCall(exec.run, modelSequence, "", "", modelRequest, responsePtr(modelResponse, err), err); appendErr != nil {
		return model.Action{}, appendErr
	}
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "model.responded", "model", map[string]any{
		"finish_reason": modelResponse.FinishReason,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	action := parseAction(modelResponse.Text)
	if err := validateActionForRole(exec.run.Role, action); err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	exec.finalAnswer = action.Answer
	exec.turnCount = exec.state.TurnCount + 1
	if _, err := e.runPoliciesAfterModel(runCtx, exec, &action, nil, map[policy.PolicyName]struct{}{
		policy.PolicyNameDelegation: {},
	}); err != nil {
		return model.Action{}, err
	}
	return action, nil
}

func (e *Executor) generateModelResponse(runCtx context.Context, exec *runExecution, req model.Request) (model.Response, error) {
	if streamingProvider, ok := exec.provider.(model.StreamingModel); ok {
		messageID := harnessruntime.NewID("msg")
		accumulator := &answerStreamAccumulator{
			observer:  ensureRunObserver(exec.observer),
			runID:     exec.run.ID,
			sessionID: exec.session.ID,
			messageID: messageID,
		}
		if err := e.generateStreamWithModelTimeout(runCtx, streamingProvider, req, accumulator); err != nil {
			_ = accumulator.Fail(err)
			return model.Response{}, err
		}
		exec.finalAnswer = accumulator.text()
		return model.Response{Text: exec.finalAnswer, FinishReason: "stop"}, nil
	}

	return e.generateWithModelTimeout(runCtx, exec.provider, req)
}

func (e *Executor) completeRun(exec *runExecution) (ExecutionResponse, error) {
	result := harnessruntime.RunResult{
		RunID:       exec.run.ID,
		Status:      harnessruntime.RunCompleted,
		Output:      exec.finalAnswer,
		CompletedAt: time.Now(),
	}

	exec.currentStep.Status = harnessruntime.StepCompleted
	exec.plan.UpdatedAt = result.CompletedAt
	exec.run.Status = harnessruntime.RunCompleted
	exec.run.TurnCount = exec.turnCount
	exec.run.UpdatedAt = result.CompletedAt
	exec.run.CompletedAt = result.CompletedAt
	exec.state.TurnCount = exec.turnCount
	exec.state.UpdatedAt = result.CompletedAt

	assistantMessage := harnessruntime.SessionMessage{
		ID:        harnessruntime.NewID("msg"),
		SessionID: exec.session.ID,
		RunID:     exec.run.ID,
		Role:      harnessruntime.MessageRoleAssistant,
		Content:   exec.finalAnswer,
		CreatedAt: result.CompletedAt,
	}
	if err := e.StateStore.AppendSessionMessage(assistantMessage); err != nil {
		return ExecutionResponse{}, err
	}
	if err := e.StateStore.SaveResult(result); err != nil {
		return ExecutionResponse{}, err
	}
	if err := e.StateStore.SavePlan(exec.plan); err != nil {
		return ExecutionResponse{}, err
	}
	if err := e.StateStore.SaveRun(exec.run); err != nil {
		return ExecutionResponse{}, err
	}
	if err := e.StateStore.SaveState(exec.state); err != nil {
		return ExecutionResponse{}, err
	}

	candidates := append([]harnessruntime.MemoryCandidate{}, exec.explicitMemoryCandidates...)
	candidates = append(candidates, e.MemoryManager.ExtractCandidates(memory.ExtractInput{
		SessionID: exec.session.ID,
		RunID:     exec.run.ID,
		Goal:      exec.task.Instruction,
		Result:    result.Output,
		Provider:  exec.run.Provider,
		Model:     exec.run.Model,
	})...)
	committedEntries := []harnessruntime.MemoryEntry{}
	if err := e.StateStore.SaveRunMemories(harnessruntime.RunMemories{
		RunID:      exec.run.ID,
		Recalled:   exec.recalledMemories,
		Candidates: candidates,
		UpdatedAt:  result.CompletedAt,
	}); err != nil {
		return ExecutionResponse{}, err
	}
	if exec.run.ParentRunID == "" {
		var err error
		committedEntries, err = e.MemoryManager.CommitCandidates(exec.session.ID, candidates)
		if err != nil {
			return ExecutionResponse{}, err
		}
		if err := e.StateStore.SaveRunMemories(harnessruntime.RunMemories{
			RunID:      exec.run.ID,
			Recalled:   exec.recalledMemories,
			Candidates: candidates,
			Committed:  committedEntries,
			UpdatedAt:  result.CompletedAt,
		}); err != nil {
			return ExecutionResponse{}, err
		}
	}

	count := 6
	if exec.run.ParentRunID == "" {
		count = 7
	}
	start := exec.reserveSequences(count)
	finalEvents := []harnessruntime.Event{
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start, "assistant.message", "assistant", map[string]any{
			"message_id": assistantMessage.ID,
			"content":    assistantMessage.Content,
		}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+1, "result.generated", "runtime", map[string]any{"bytes": len(result.Output)}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+2, "memory.candidate_extracted", "memory", map[string]any{"count": len(candidates)}),
	}
	offset := int64(3)
	if exec.run.ParentRunID == "" {
		finalEvents = append(finalEvents, e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+3, "memory.committed", "memory", map[string]any{"count": len(committedEntries)}))
		offset++
	}
	finalEvents = append(finalEvents,
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+offset, "plan.step.completed", "runtime", map[string]any{"step_id": exec.currentStep.ID}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+offset+1, "run.status_changed", "runtime", map[string]any{"from": harnessruntime.RunRunning, "to": harnessruntime.RunCompleted}),
		e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+offset+2, "run.completed", "runtime", map[string]any{"run_id": exec.run.ID}),
	)
	if err := e.appendEvents(finalEvents, exec.observer); err != nil {
		return ExecutionResponse{}, err
	}

	return ExecutionResponse{
		Task:   exec.task,
		Run:    exec.run,
		Result: &result,
	}, nil
}

func (e *Executor) failOnly(exec *runExecution, err error, sequence int64) error {
	_, failErr := e.failRun(exec.run, exec.plan, exec.task.ID, exec.session.ID, exec.state, err, sequence, exec.observer)
	return failErr
}
