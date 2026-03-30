package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/model"
	promptpkg "github.com/huanglei214/agent-demo/internal/prompt"
	"github.com/huanglei214/agent-demo/internal/retrieval"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

func (e *Executor) resumePostToolAction(runCtx context.Context, exec *runExecution) (model.Action, error) {
	pendingToolResults := pendingToolResultsFromState(exec.state)
	followUpPrompt := e.PromptBuilder.BuildFollowUpPrompt(exec.run.Role, exec.task, pendingToolResults, exec.workingEvidence, e.promptToolMetadataForSkill(exec.activeSkill), exec.activeSkill)
	modelSequence := exec.nextSequence()
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, modelSequence, "model.called", "runtime", map[string]any{
		"provider": exec.run.Provider,
		"model":    exec.run.Model,
		"role":     exec.run.Role,
		"phase":    "post_tool_resume",
		"tool":     exec.state.PendingToolName,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpRequest := model.Request{
		SystemPrompt: followUpPrompt.System,
		Input:        followUpPrompt.Input,
		Metadata:     followUpPrompt.Metadata,
	}
	followUpResponse, err := e.generateWithModelTimeout(runCtx, exec.provider, followUpRequest)
	if appendErr := e.appendModelCall(exec.run, modelSequence, "post_tool_resume", exec.state.PendingToolName, followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
		return model.Action{}, appendErr
	}
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}

	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "model.responded", "model", map[string]any{
		"finish_reason": followUpResponse.FinishReason,
		"phase":         "post_tool_resume",
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpAction := parseAction(followUpResponse.Text)
	if followUpAction.Action == "" {
		return model.Action{}, e.failOnly(exec, errors.New("resumed post-tool model response did not produce a valid action"), exec.nextSequence())
	}
	if err := validateActionForRole(exec.run.Role, followUpAction); err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}

	exec.finalAnswer = followUpAction.Answer
	exec.turnCount = exec.state.TurnCount + 1
	exec.state.ResumePhase = ""
	exec.state.PendingToolName = ""
	exec.state.PendingToolResult = nil
	exec.state.UpdatedAt = time.Now()
	if err := e.StateStore.SaveState(exec.state); err != nil {
		return model.Action{}, err
	}
	return followUpAction, nil
}

func (e *Executor) dispatchToolActions(runCtx context.Context, exec *runExecution, action model.Action) (model.Action, error) {
	for action.Action == "tool" {
		calls, err := toolCallsFromAction(action)
		if err != nil {
			return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
		}
		if len(exec.explicitMemoryCandidates) > 0 && containsTool(calls, "fs.write_file") {
			if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "memory.routed", "memory", map[string]any{
				"count":      len(exec.explicitMemoryCandidates),
				"source":     "tool_intercept",
				"tool_batch": extractToolNames(calls),
			}), exec.observer); err != nil {
				return model.Action{}, err
			}
			exec.finalAnswer = exec.explicitMemoryAnswer
			return model.Action{Action: "final", Answer: exec.finalAnswer}, nil
		}

		if err := e.validateToolCalls(exec, calls); err != nil {
			return model.Action{}, err
		}
		toolResults, err := e.runToolBatch(runCtx, exec, calls)
		if err != nil {
			return model.Action{}, err
		}

		recentEvents, err := e.EventStore.ReadAll(exec.run.ID)
		if err != nil {
			return model.Action{}, err
		}
		exec.workingEvidence = promptpkg.BuildWorkingEvidenceForPrompt(collectSuccessfulToolResults(recentEvents))

		if err := e.maybeCompactContext(exec, recentEvents, toolResults); err != nil {
			return model.Action{}, err
		}

		exec.state.ResumePhase = "post_tool"
		exec.state.PendingToolName = ""
		exec.state.PendingToolResult = nil
		exec.state.PendingToolResults = toolResults
		exec.state.UpdatedAt = time.Now()
		if err := e.StateStore.SaveState(exec.state); err != nil {
			return model.Action{}, err
		}

		followUpAction, err := e.followUpAfterTools(runCtx, exec, calls, toolResults)
		if err != nil {
			return model.Action{}, err
		}

		action = followUpAction
		exec.finalAnswer = followUpAction.Answer
		exec.state.ResumePhase = ""
		exec.state.PendingToolName = ""
		exec.state.PendingToolResult = nil
		exec.state.PendingToolResults = nil
		exec.state.UpdatedAt = time.Now()
		if err := e.StateStore.SaveState(exec.state); err != nil {
			return model.Action{}, err
		}
		exec.turnCount++
		if action.Action != "tool" {
			return action, nil
		}
	}
	return action, nil
}

func (e *Executor) validateToolCalls(exec *runExecution, calls []model.ToolCall) error {
	for _, call := range calls {
		if err := ensureSkillAllowsTool(exec.activeSkill, call.Tool); err != nil {
			return e.failOnly(exec, err, exec.nextSequence())
		}
		if err := e.DelegationManager.ValidateTools(exec.run, call.Tool); err != nil {
			rejectedSequence := exec.nextSequence()
			if appendErr := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, rejectedSequence, "subagent.rejected", "delegation", map[string]any{
				"tool":   call.Tool,
				"reason": err.Error(),
			}), exec.observer); appendErr != nil {
				return appendErr
			}
			return e.failOnly(exec, err, exec.nextSequence())
		}
	}
	return nil
}

func (e *Executor) runToolBatch(runCtx context.Context, exec *runExecution, calls []model.ToolCall) ([]harnessruntime.ToolCallResult, error) {
	toolCallIDs := make([]string, len(calls))
	for i := range calls {
		toolCallIDs[i] = harnessruntime.NewID("toolcall")
	}
	if len(calls) > 1 {
		if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "tool.batch.started", "runtime", map[string]any{
			"count": len(calls),
			"tools": extractToolNames(calls),
		}), exec.observer); err != nil {
			return nil, err
		}
	}

	start := exec.reserveSequences(len(calls))
	calledEvents := make([]harnessruntime.Event, 0, len(calls))
	for i, call := range calls {
		calledEvents = append(calledEvents, e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+int64(i), "tool.called", "runtime", map[string]any{
			"tool_call_id": toolCallIDs[i],
			"tool":         call.Tool,
			"input":        call.Input,
		}))
	}
	if err := e.appendEvents(calledEvents, exec.observer); err != nil {
		return nil, err
	}

	toolResults, failures := e.executeToolCalls(runCtx, toolCallIDs, calls)
	start = exec.reserveSequences(len(calls))
	resultEvents := make([]harnessruntime.Event, 0, len(calls))
	var firstErr error
	for i, call := range calls {
		if i < len(toolResults) && toolResults[i].Tool != "" {
			resultEvents = append(resultEvents, e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+int64(len(resultEvents)), "tool.succeeded", "tool", map[string]any{
				"tool_call_id": toolResults[i].ToolCallID,
				"tool":         toolResults[i].Tool,
				"input":        toolResults[i].Input,
				"result":       toolResults[i].Result,
			}))
			retrieval.UpdateProgress(&exec.retrievalProgress, toolResults[i].Tool, toolResults[i].Result)
			continue
		}
		var failure toolExecutionError
		for _, item := range failures {
			if item.Index == i {
				failure = item
				break
			}
		}
		var details map[string]any
		if detailedErr, ok := failure.Err.(toolruntime.DetailedError); ok {
			details = detailedErr.Details()
		}
		resultEvents = append(resultEvents, e.newEvent(exec.run, exec.task.ID, exec.session.ID, start+int64(len(resultEvents)), "tool.failed", "tool", map[string]any{
			"tool_call_id": toolCallIDs[i],
			"tool":         call.Tool,
			"input":        call.Input,
			"error":        failure.Err.Error(),
			"details":      details,
		}))
		if firstErr == nil {
			firstErr = failure.Err
		}
	}
	if err := e.appendEvents(resultEvents, exec.observer); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, e.failOnly(exec, firstErr, exec.nextSequence())
	}

	for _, toolResult := range toolResults {
		if toolResult.Tool != "fs.write_file" {
			continue
		}
		eventType := "fs.file_created"
		if mode, ok := toolResult.Result["write_mode"].(string); ok && mode == "updated" {
			eventType = "fs.file_updated"
		}
		if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), eventType, "tool", toolResult.Result), exec.observer); err != nil {
			return nil, err
		}
	}

	return toolResults, nil
}

func (e *Executor) maybeCompactContext(exec *runExecution, recentEvents []harnessruntime.Event, toolResults []harnessruntime.ToolCallResult) error {
	shouldCompact, reason := e.ContextManager.ShouldCompact(harnesscontext.CompactionCheckInput{
		TokenUsage:       exec.lastPromptBytes,
		TokenBudget:      1600,
		RecentEventCount: len(recentEvents),
		LastToolBytes:    totalToolBytes(toolResults),
	})
	if !shouldCompact {
		return nil
	}

	summary, err := e.ContextManager.Compact(harnesscontext.CompactInput{
		RunID:        exec.run.ID,
		Scope:        "run",
		Plan:         exec.plan,
		CurrentStep:  exec.currentStep,
		RecentEvents: recentEvents,
	})
	if err != nil {
		return err
	}
	exec.summaries = append(exec.summaries, summary)
	if err := e.StateStore.SaveSummaries(exec.run.ID, exec.summaries); err != nil {
		return err
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "context.compacted", "context", map[string]any{
		"summary_id": summary.ID,
		"scope":      summary.Scope,
		"reason":     reason,
	}), exec.observer); err != nil {
		return err
	}
	return nil
}

func (e *Executor) followUpAfterTools(runCtx context.Context, exec *runExecution, calls []model.ToolCall, toolResults []harnessruntime.ToolCallResult) (model.Action, error) {
	followUpPrompt := e.PromptBuilder.BuildFollowUpPrompt(exec.run.Role, exec.task, toolResults, exec.workingEvidence, e.promptToolMetadataForSkill(exec.activeSkill), exec.activeSkill)
	modelSequence := exec.nextSequence()
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, modelSequence, "model.called", "runtime", map[string]any{
		"provider": exec.run.Provider,
		"model":    exec.run.Model,
		"role":     exec.run.Role,
		"phase":    "post_tool",
		"tools":    extractToolNames(calls),
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpRequest := model.Request{
		SystemPrompt: followUpPrompt.System,
		Input:        followUpPrompt.Input,
		Metadata:     followUpPrompt.Metadata,
	}
	followUpResponse, err := e.generateWithModelTimeout(runCtx, exec.provider, followUpRequest)
	if appendErr := e.appendModelCall(exec.run, modelSequence, "post_tool", strings.Join(extractToolNames(calls), ","), followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
		return model.Action{}, appendErr
	}
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "model.responded", "model", map[string]any{
		"finish_reason": followUpResponse.FinishReason,
		"phase":         "post_tool",
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	followUpAction := parseAction(followUpResponse.Text)
	if followUpAction.Action == "" {
		return model.Action{}, e.failOnly(exec, errors.New("post-tool model response did not produce a valid action"), exec.nextSequence())
	}
	if err := validateActionForRole(exec.run.Role, followUpAction); err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if decision := retrieval.DecideProgress(exec.retrievalProgress, followUpAction); decision.ShouldForceFinal {
		forcedAction, err := e.forceFinalFromRetrieval(runCtx, exec, calls, decision.Reason)
		if err != nil {
			return model.Action{}, err
		}
		followUpAction = forcedAction
	}

	return followUpAction, nil
}

func (e *Executor) forceFinalFromRetrieval(runCtx context.Context, exec *runExecution, calls []model.ToolCall, reason string) (model.Action, error) {
	forcedPrompt := e.PromptBuilder.BuildForcedFinalPrompt(
		exec.run.Role,
		exec.task,
		reason,
		retrieval.BuildEvidencePayload(exec.retrievalProgress),
		e.promptToolMetadataForSkill(exec.activeSkill),
		exec.activeSkill,
	)
	modelSequence := exec.nextSequence()
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, modelSequence, "model.called", "runtime", map[string]any{
		"provider": exec.run.Provider,
		"model":    exec.run.Model,
		"role":     exec.run.Role,
		"phase":    "forced_final",
		"reason":   reason,
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	forcedRequest := model.Request{
		SystemPrompt: forcedPrompt.System,
		Input:        forcedPrompt.Input,
		Metadata:     forcedPrompt.Metadata,
	}
	forcedResponse, err := e.generateWithModelTimeout(runCtx, exec.provider, forcedRequest)
	if appendErr := e.appendModelCall(exec.run, modelSequence, "forced_final", strings.Join(extractToolNames(calls), ","), forcedRequest, responsePtr(forcedResponse, err), err); appendErr != nil {
		return model.Action{}, appendErr
	}
	if err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if err := e.appendEvent(e.newEvent(exec.run, exec.task.ID, exec.session.ID, exec.nextSequence(), "model.responded", "model", map[string]any{
		"finish_reason": forcedResponse.FinishReason,
		"phase":         "forced_final",
	}), exec.observer); err != nil {
		return model.Action{}, err
	}

	forcedAction := parseAction(forcedResponse.Text)
	if forcedAction.Action == "" {
		return model.Action{}, e.failOnly(exec, errors.New("forced-final model response did not produce a valid action"), exec.nextSequence())
	}
	if err := validateActionForRole(exec.run.Role, forcedAction); err != nil {
		return model.Action{}, e.failOnly(exec, err, exec.nextSequence())
	}
	if forcedAction.Action != "final" || strings.TrimSpace(forcedAction.Answer) == "" {
		return model.Action{}, e.failOnly(exec, errors.New("forced-final model response did not produce a final answer"), exec.nextSequence())
	}
	return forcedAction, nil
}

func hasPendingToolResults(state harnessruntime.RunState) bool {
	if len(state.PendingToolResults) > 0 {
		return true
	}
	return strings.TrimSpace(state.PendingToolName) != "" && len(state.PendingToolResult) > 0
}

func pendingToolResultsFromState(state harnessruntime.RunState) []harnessruntime.ToolCallResult {
	if len(state.PendingToolResults) > 0 {
		return append([]harnessruntime.ToolCallResult(nil), state.PendingToolResults...)
	}
	if strings.TrimSpace(state.PendingToolName) == "" || len(state.PendingToolResult) == 0 {
		return nil
	}
	return []harnessruntime.ToolCallResult{{
		Tool:   state.PendingToolName,
		Result: state.PendingToolResult,
	}}
}

func toolCallsFromAction(action model.Action) ([]model.ToolCall, error) {
	if action.Action != "tool" {
		return nil, fmt.Errorf("action %q is not a tool action", action.Action)
	}
	if len(action.Calls) == 0 {
		return nil, errors.New("tool action did not include any calls")
	}
	return action.Calls, nil
}

func containsTool(calls []model.ToolCall, toolName string) bool {
	for _, call := range calls {
		if call.Tool == toolName {
			return true
		}
	}
	return false
}

func extractToolNames(calls []model.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.Tool) == "" {
			continue
		}
		names = append(names, call.Tool)
	}
	return names
}

func totalToolBytes(results []harnessruntime.ToolCallResult) int {
	total := 0
	for _, result := range results {
		total += toolBytes(result.Result)
	}
	return total
}

func collectSuccessfulToolResults(events []harnessruntime.Event) []harnessruntime.ToolCallResult {
	results := make([]harnessruntime.ToolCallResult, 0)
	for _, event := range events {
		if event.Type != "tool.succeeded" {
			continue
		}
		toolName, _ := event.Payload["tool"].(string)
		if toolName == "" {
			continue
		}
		input, _ := event.Payload["input"].(map[string]any)
		result, _ := event.Payload["result"].(map[string]any)
		results = append(results, harnessruntime.ToolCallResult{
			ToolCallID: firstNonEmptyString(event.Payload["tool_call_id"]),
			Tool:       toolName,
			Input:      input,
			Result:     result,
		})
	}
	return results
}

func mergeWorkingEvidence(base, extra map[string]any) map[string]any {
	if len(base) == 0 {
		return extra
	}
	if len(extra) == 0 {
		return base
	}
	merged := map[string]any{}
	keys := make(map[string]struct{})
	for key := range base {
		keys[key] = struct{}{}
	}
	for key := range extra {
		keys[key] = struct{}{}
	}
	orderedKeys := make([]string, 0, len(keys))
	for key := range keys {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	for _, key := range orderedKeys {
		baseSlice, _ := base[key].([]map[string]any)
		extraSlice, _ := extra[key].([]map[string]any)
		if baseSlice == nil && extraSlice == nil {
			if value, ok := extra[key]; ok {
				merged[key] = value
				continue
			}
			merged[key] = base[key]
			continue
		}
		combined := make([]map[string]any, 0, len(baseSlice)+len(extraSlice))
		combined = append(combined, baseSlice...)
		combined = append(combined, extraSlice...)
		merged[key] = combined
	}
	return merged
}

type toolExecutionError struct {
	Index int
	Call  model.ToolCall
	Err   error
}

func (e *Executor) executeToolCalls(ctx context.Context, toolCallIDs []string, calls []model.ToolCall) ([]harnessruntime.ToolCallResult, []toolExecutionError) {
	if e.canExecuteToolBatchInParallel(calls) {
		return e.executeToolCallsParallel(ctx, toolCallIDs, calls)
	}
	return e.executeToolCallsSerial(ctx, toolCallIDs, calls)
}

func (e *Executor) executeToolCallsSerial(ctx context.Context, toolCallIDs []string, calls []model.ToolCall) ([]harnessruntime.ToolCallResult, []toolExecutionError) {
	results := make([]harnessruntime.ToolCallResult, len(calls))
	failures := make([]toolExecutionError, 0)
	for i, call := range calls {
		result, err := e.ToolExecutor.Execute(ctx, call.Tool, call.Input)
		if err != nil {
			failures = append(failures, toolExecutionError{Index: i, Call: call, Err: err})
			break
		}
		results[i] = harnessruntime.ToolCallResult{
			ToolCallID: toolCallIDs[i],
			Tool:       call.Tool,
			Input:      call.Input,
			Result:     result.Content,
		}
	}
	return results, failures
}

func (e *Executor) executeToolCallsParallel(ctx context.Context, toolCallIDs []string, calls []model.ToolCall) ([]harnessruntime.ToolCallResult, []toolExecutionError) {
	results := make([]harnessruntime.ToolCallResult, len(calls))
	failures := make([]toolExecutionError, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(i int, call model.ToolCall) {
			defer wg.Done()
			result, err := e.ToolExecutor.Execute(ctx, call.Tool, call.Input)
			if err != nil {
				failures[i] = toolExecutionError{Index: i, Call: call, Err: err}
				return
			}
			results[i] = harnessruntime.ToolCallResult{
				ToolCallID: toolCallIDs[i],
				Tool:       call.Tool,
				Input:      call.Input,
				Result:     result.Content,
			}
		}(i, call)
	}
	wg.Wait()
	filtered := make([]toolExecutionError, 0)
	for _, failure := range failures {
		if failure.Err != nil {
			filtered = append(filtered, failure)
		}
	}
	return results, filtered
}

func (e *Executor) canExecuteToolBatchInParallel(calls []model.ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	for _, call := range calls {
		toolDef, ok := e.ToolRegistry.Get(call.Tool)
		if !ok {
			return false
		}
		if toolDef.AccessMode() != toolruntime.AccessReadOnly {
			return false
		}
	}
	return true
}

func validateActionForRole(role harnessruntime.RunRole, action model.Action) error {
	switch role {
	case harnessruntime.RunRoleSubagent:
		switch action.Action {
		case "tool", "final":
			if action.Action == "tool" && len(action.Calls) == 0 {
				return errors.New("subagent tool action did not include any calls")
			}
			return nil
		case "delegate":
			return errors.New("subagent cannot delegate further work")
		default:
			return fmt.Errorf("subagent returned unsupported action %q", action.Action)
		}
	default:
		switch action.Action {
		case "tool", "final", "delegate":
			if action.Action == "tool" && len(action.Calls) == 0 {
				return errors.New("lead-agent tool action did not include any calls")
			}
			return nil
		default:
			return fmt.Errorf("lead-agent returned unsupported action %q", action.Action)
		}
	}
}
