package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	arkmodel "github.com/huanglei214/agent-demo/internal/model/ark"
	"github.com/huanglei214/agent-demo/internal/planner"
	promptpkg "github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

type RunRequest struct {
	Instruction string
	Workspace   string
	Provider    string
	Model       string
	MaxTurns    int
	SessionID   string
	Skill       string
}

type RunResponse struct {
	Task   harnessruntime.Task       `json:"task"`
	Run    harnessruntime.Run        `json:"run"`
	Result *harnessruntime.RunResult `json:"result,omitempty"`
}

func (s Services) StartRun(req RunRequest) (RunResponse, error) {
	return s.startRun(req, nil)
}

func (s Services) StartRunStream(req RunRequest, observer RunObserver) (RunResponse, error) {
	return s.startRun(req, observer)
}

func (s Services) appendModelCall(run harnessruntime.Run, sequence int64, phase, toolName string, req model.Request, resp *model.Response, callErr error) error {
	call := harnessruntime.ModelCall{
		ID:       harnessruntime.NewID("modelcall"),
		RunID:    run.ID,
		Sequence: sequence,
		Phase:    phase,
		Tool:     toolName,
		Request: harnessruntime.ModelRequestSnapshot{
			SystemPrompt: req.SystemPrompt,
			Input:        req.Input,
			Provider:     run.Provider,
			Model:        run.Model,
			Messages: []harnessruntime.ModelMessage{
				{
					Role:    "system",
					Content: req.SystemPrompt,
				},
				{
					Role:    "user",
					Content: req.Input,
				},
			},
			Metadata: req.Metadata,
		},
		Timestamp: time.Now(),
	}
	if resp != nil {
		call.Response = &harnessruntime.ModelResponseSnapshot{
			Text:         resp.Text,
			FinishReason: resp.FinishReason,
			Metadata:     resp.Metadata,
		}
	}
	if callErr != nil {
		call.Error = callErr.Error()
	}
	return s.StateStore.AppendModelCall(call)
}

func (s Services) startRun(req RunRequest, observer RunObserver) (RunResponse, error) {
	observer = ensureRunObserver(observer)
	now := time.Now()

	task := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: req.Instruction,
		Workspace:   req.Workspace,
		CreatedAt:   now,
	}

	activeSkill, skillErr := s.resolveActiveSkill(req)
	if skillErr != nil {
		return RunResponse{}, skillErr
	}
	if activeSkill != nil {
		task.Metadata = map[string]string{
			"skill": activeSkill.Name,
			"scope": string(activeSkill.Scope),
		}
	}

	var (
		session        harnessruntime.Session
		err            error
		createdSession bool
	)
	if strings.TrimSpace(req.SessionID) != "" {
		session, err = s.StateStore.LoadSession(req.SessionID)
		if err != nil {
			return RunResponse{}, err
		}
		if session.Workspace != req.Workspace {
			return RunResponse{}, errors.New("session workspace does not match requested workspace")
		}
		session.UpdatedAt = now
	} else {
		session = harnessruntime.Session{
			ID:        harnessruntime.NewID("session"),
			Workspace: req.Workspace,
			CreatedAt: now,
			UpdatedAt: now,
		}
		createdSession = true
	}

	run := harnessruntime.Run{
		ID:        harnessruntime.NewID("run"),
		TaskID:    task.ID,
		SessionID: session.ID,
		Role:      harnessruntime.RunRoleLead,
		Status:    harnessruntime.RunPending,
		Provider:  req.Provider,
		Model:     req.Model,
		MaxTurns:  req.MaxTurns,
		TurnCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	plan, err := s.Planner.CreatePlan(context.Background(), planner.PlanInput{
		RunID:     run.ID,
		Goal:      req.Instruction,
		Workspace: req.Workspace,
	})
	if err != nil {
		return RunResponse{}, err
	}
	if len(plan.Steps) == 0 {
		return RunResponse{}, errors.New("planner returned an empty plan")
	}

	state := harnessruntime.RunState{
		RunID:     run.ID,
		TurnCount: 0,
		UpdatedAt: now,
	}

	if err := s.StateStore.SaveTask(task); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveSession(session); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveRun(run); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SavePlan(plan); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveState(state); err != nil {
		return RunResponse{}, err
	}

	nextSequence, err := s.EventStore.NextSequence(run.ID)
	if err != nil {
		return RunResponse{}, err
	}

	events := []harnessruntime.Event{
		s.newEvent(run, task.ID, session.ID, nextSequence, "task.created", "system", map[string]any{"task_id": task.ID}),
	}
	sequence := nextSequence + 1
	if createdSession {
		events = append(events, s.newEvent(run, task.ID, session.ID, sequence, "session.created", "system", map[string]any{"session_id": session.ID}))
		sequence++
	}
	events = append(events,
		s.newEvent(run, task.ID, session.ID, sequence, "run.created", "system", map[string]any{"status": run.Status}),
		s.newEvent(run, task.ID, session.ID, sequence+1, "run.role_assigned", "runtime", map[string]any{"role": run.Role}),
		s.newEvent(run, task.ID, session.ID, sequence+2, "plan.created", "planner", map[string]any{"plan_id": plan.ID, "version": plan.Version}),
	)

	if err := s.appendEvents(events, observer); err != nil {
		return RunResponse{}, err
	}

	userMessage := harnessruntime.SessionMessage{
		ID:        harnessruntime.NewID("msg"),
		SessionID: session.ID,
		RunID:     run.ID,
		Role:      harnessruntime.MessageRoleUser,
		Content:   req.Instruction,
		CreatedAt: now,
	}
	if err := s.StateStore.AppendSessionMessage(userMessage); err != nil {
		return RunResponse{}, err
	}
	if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+3, "user.message", "user", map[string]any{
		"message_id": userMessage.ID,
		"content":    userMessage.Content,
	}), observer); err != nil {
		return RunResponse{}, err
	}
	if activeSkill != nil {
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+4, "skill.activated", "runtime", map[string]any{
			"name":          activeSkill.Name,
			"scope":         activeSkill.Scope,
			"allowed_tools": activeSkill.AllowedTools,
		}), observer); err != nil {
			return RunResponse{}, err
		}
	}

	return s.executeRun(task, session, run, plan, state, true, observer)
}

func (s Services) promptToolMetadata() []map[string]string {
	return s.promptToolMetadataForSkill(nil)
}

func (s Services) promptToolMetadataForSkill(activeSkill *skill.Definition) []map[string]string {
	descriptors := s.toolDescriptorsForNames(allowedToolSet(activeSkill))
	result := make([]map[string]string, 0, len(descriptors))
	for _, item := range descriptors {
		result = append(result, map[string]string{
			"name":        item.Name,
			"description": item.Description,
			"access":      string(item.Access),
		})
	}
	return result
}

func (s Services) executeRun(task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (RunResponse, error) {
	observer = ensureRunObserver(observer)
	if len(plan.Steps) == 0 {
		return RunResponse{}, errors.New("plan has no steps to execute")
	}
	activeSkill, err := s.activeSkillForTask(task)
	if err != nil {
		return RunResponse{}, err
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

	nextSequence, err := s.EventStore.NextSequence(run.ID)
	if err != nil {
		return RunResponse{}, err
	}

	sequence := nextSequence - 1
	if activate {
		run.Status = harnessruntime.RunRunning
		run.CurrentStepID = currentStep.ID
		run.UpdatedAt = time.Now()
		currentStep.Status = harnessruntime.StepRunning
		plan.UpdatedAt = run.UpdatedAt
		state.CurrentStepID = currentStep.ID
		state.UpdatedAt = run.UpdatedAt

		if err := s.StateStore.SaveRun(run); err != nil {
			return RunResponse{}, err
		}
		if err := s.StateStore.SavePlan(plan); err != nil {
			return RunResponse{}, err
		}
		if err := s.StateStore.SaveState(state); err != nil {
			return RunResponse{}, err
		}

		lifecycleEvents := []harnessruntime.Event{
			s.newEvent(run, task.ID, session.ID, sequence+1, "run.status_changed", "runtime", map[string]any{"from": harnessruntime.RunPending, "to": harnessruntime.RunRunning}),
			s.newEvent(run, task.ID, session.ID, sequence+2, "run.started", "runtime", map[string]any{"run_id": run.ID}),
			s.newEvent(run, task.ID, session.ID, sequence+3, "plan.step.started", "runtime", map[string]any{"step_id": currentStep.ID}),
		}
		if err := s.appendEvents(lifecycleEvents, observer); err != nil {
			return RunResponse{}, err
		}
		sequence += int64(len(lifecycleEvents))
	}

	recentEvents, err := s.EventStore.ReadAll(run.ID)
	if err != nil {
		return RunResponse{}, err
	}
	retrievalProgress := buildRetrievalProgress(recentEvents)
	workingEvidence := promptpkg.BuildWorkingEvidenceForPrompt(collectSuccessfulToolResults(recentEvents))

	recalledMemories, err := s.MemoryManager.Recall(memory.RecallQuery{
		SessionID: session.ID,
		Goal:      task.Instruction,
		Limit:     5,
	})
	if err != nil {
		return RunResponse{}, err
	}
	if run.Role == harnessruntime.RunRoleSubagent {
		recalledMemories = nil
	}

	summaries, err := s.StateStore.LoadSummaries(run.ID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunResponse{}, err
	}

	if err := s.StateStore.SaveRunMemories(harnessruntime.RunMemories{
		RunID:     run.ID,
		Recalled:  recalledMemories,
		UpdatedAt: time.Now(),
	}); err != nil {
		return RunResponse{}, err
	}

	recentMessages := []harnessruntime.SessionMessage{}

	modelContext := s.ContextManager.Build(harnesscontext.BuildInput{
		Task:         task,
		Plan:         plan,
		CurrentStep:  currentStep,
		RecentEvents: recentEvents,
		Summaries:    summaries,
		Memories:     recalledMemories,
		Messages:     recentMessages,
	})

	runPrompt := s.PromptBuilder.BuildRunPrompt(run.Role, task, plan, currentStep, modelContext, s.promptToolMetadataForSkill(activeSkill), activeSkill)
	explicitMemoryCandidates, explicitMemoryAnswer, routedToMemory := s.MemoryManager.DetectExplicitRemember(memory.ExplicitRememberInput{
		SessionID:   session.ID,
		RunID:       run.ID,
		Instruction: task.Instruction,
	})
	preModelEvents := []harnessruntime.Event{
		s.newEvent(run, task.ID, session.ID, sequence+1, "memory.recalled", "memory", map[string]any{
			"count": len(recalledMemories),
		}),
		s.newEvent(run, task.ID, session.ID, sequence+2, "prompt.built", "prompt", runPrompt.Metadata),
		s.newEvent(run, task.ID, session.ID, sequence+3, "context.built", "context", map[string]any{
			"task_id":       task.ID,
			"plan_id":       plan.ID,
			"current_step":  currentStep.ID,
			"message_count": len(recentMessages),
			"memory_count":  len(recalledMemories),
			"summary_count": len(summaries),
			"recent_count":  len(modelContext.Recent),
		}),
	}
	if err := s.appendEvents(preModelEvents, observer); err != nil {
		return RunResponse{}, err
	}
	sequence += int64(len(preModelEvents))

	runCtx := context.Background()

	provider, err := s.ModelFactory()
	if err != nil {
		return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+1, observer)
	}

	finalAnswer := ""
	turnCount := state.TurnCount
	action := model.Action{Action: "final", Answer: explicitMemoryAnswer}

	if !activate && state.ResumePhase == "post_tool" && hasPendingToolResults(state) {
		pendingToolResults := pendingToolResultsFromState(state)
		followUpPrompt := s.PromptBuilder.BuildFollowUpPrompt(run.Role, task, pendingToolResults, workingEvidence, s.promptToolMetadataForSkill(activeSkill), activeSkill)
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
			"role":     run.Role,
			"phase":    "post_tool_resume",
			"tool":     state.PendingToolName,
		}), observer); err != nil {
			return RunResponse{}, err
		}

		followUpRequest := model.Request{
			SystemPrompt: followUpPrompt.System,
			Input:        followUpPrompt.Input,
			Metadata:     followUpPrompt.Metadata,
		}
		followUpResponse, err := s.generateWithModelTimeout(runCtx, provider, followUpRequest)
		if appendErr := s.appendModelCall(run, sequence+1, "post_tool_resume", state.PendingToolName, followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
			return RunResponse{}, appendErr
		}
		if err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+2, observer)
		}

		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
			"finish_reason": followUpResponse.FinishReason,
			"phase":         "post_tool_resume",
		}), observer); err != nil {
			return RunResponse{}, err
		}

		followUpAction := parseAction(followUpResponse.Text)
		if followUpAction.Action == "" {
			return s.failRun(run, plan, task.ID, session.ID, state, errors.New("resumed post-tool model response did not produce a valid action"), sequence+3, observer)
		}
		if err := validateActionForRole(run.Role, followUpAction); err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+3, observer)
		}

		action = followUpAction
		finalAnswer = followUpAction.Answer
		turnCount = state.TurnCount + 1
		state.ResumePhase = ""
		state.PendingToolName = ""
		state.PendingToolResult = nil
		state.UpdatedAt = time.Now()
		if err := s.StateStore.SaveState(state); err != nil {
			return RunResponse{}, err
		}
		sequence += 2
	} else if routedToMemory {
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "memory.routed", "memory", map[string]any{
			"count": len(explicitMemoryCandidates),
		}), observer); err != nil {
			return RunResponse{}, err
		}
		finalAnswer = explicitMemoryAnswer
		sequence++
	} else {
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
			"role":     run.Role,
		}), observer); err != nil {
			return RunResponse{}, err
		}

		modelRequest := model.Request{
			SystemPrompt: runPrompt.System,
			Input:        runPrompt.Input,
			Metadata:     runPrompt.Metadata,
		}
		modelResponse, err := s.generateWithModelTimeout(runCtx, provider, modelRequest)
		if appendErr := s.appendModelCall(run, sequence+1, "", "", modelRequest, responsePtr(modelResponse, err), err); appendErr != nil {
			return RunResponse{}, appendErr
		}
		if err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+1, observer)
		}

		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
			"finish_reason": modelResponse.FinishReason,
		}), observer); err != nil {
			return RunResponse{}, err
		}

		action = parseAction(modelResponse.Text)
		if err := validateActionForRole(run.Role, action); err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+3, observer)
		}
		finalAnswer = action.Answer
		sequence += 2
		turnCount = state.TurnCount + 1
	}

	if action.Action == "delegate" {
		canDelegate, reason := s.DelegationManager.CanDelegate(runCtx, run, *currentStep)
		if !canDelegate {
			if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.rejected", "delegation", map[string]any{
				"step_id": currentStep.ID,
				"reason":  reason,
			}), observer); err != nil {
				return RunResponse{}, err
			}
			return s.failRun(run, plan, task.ID, session.ID, state, errors.New("delegation rejected: "+reason), sequence+2, observer)
		}

		delegationGoal := strings.TrimSpace(action.DelegationGoal)
		if delegationGoal == "" {
			delegationGoal = currentStep.Description
		}
		delegationTask := s.DelegationManager.BuildTask(run, plan, *currentStep, delegationGoal, recalledMemories, summaries)
		childResponse, childResult, err := s.spawnChildRun(task, session, run, delegationTask)
		if err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+1, observer)
		}

		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.spawned", "delegation", map[string]any{
			"child_run_id": childResponse.Run.ID,
			"step_id":      currentStep.ID,
			"role":         childResponse.Run.Role,
		}), observer); err != nil {
			return RunResponse{}, err
		}
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+2, "subagent.completed", "delegation", map[string]any{
			"child_run_id":    childResponse.Run.ID,
			"needs_replan":    childResult.NeedsReplan,
			"summary":         childResult.Summary,
			"recommendations": childResult.Recommendations,
		}), observer); err != nil {
			return RunResponse{}, err
		}
		sequence += 2

		replanDecision := decideChildReplan(childResult)
		if replanDecision.ShouldReplan {
			replanned, err := s.Planner.Replan(runCtx, planner.ReplanInput{
				RunID:    run.ID,
				Goal:     task.Instruction,
				Previous: plan,
				Reason:   replanDecision.Reason,
			})
			if err != nil {
				return RunResponse{}, err
			}
			currentStatus := currentStep.Status
			plan = replanned
			if len(plan.Steps) == 0 {
				return RunResponse{}, errors.New("replan returned an empty plan")
			}
			currentStep = &plan.Steps[0]
			currentStep.Status = currentStatus
			plan.UpdatedAt = time.Now()
			if err := s.StateStore.SavePlan(plan); err != nil {
				return RunResponse{}, err
			}
			if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "plan.updated", "planner", map[string]any{
				"plan_id": plan.ID,
				"version": plan.Version,
				"reason":  replanDecision.Reason,
				"step_id": currentStep.ID,
			}), observer); err != nil {
				return RunResponse{}, err
			}
			sequence++
		}

		delegationEvidence := mergeWorkingEvidence(workingEvidence, promptpkg.BuildWorkingEvidenceForPrompt([]harnessruntime.ToolCallResult{
			{
				ToolCallID: childResponse.Run.ID,
				Tool:       "subagent",
				Input:      map[string]any{"child_run_id": childResponse.Run.ID},
				Result:     delegationResultContent(childResult),
			},
		}))
		followUpPrompt := s.PromptBuilder.BuildFollowUpPrompt(run.Role, task, []harnessruntime.ToolCallResult{
			{
				ToolCallID: childResponse.Run.ID,
				Tool:       "subagent",
				Input:      map[string]any{"child_run_id": childResponse.Run.ID},
				Result:     delegationResultContent(childResult),
			},
		}, delegationEvidence, s.promptToolMetadataForSkill(activeSkill), activeSkill)
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
			"role":     run.Role,
			"phase":    "post_delegation",
			"child":    childResponse.Run.ID,
		}), observer); err != nil {
			return RunResponse{}, err
		}

		followUpRequest := model.Request{
			SystemPrompt: followUpPrompt.System,
			Input:        followUpPrompt.Input,
			Metadata:     followUpPrompt.Metadata,
		}
		followUpResponse, err := s.generateWithModelTimeout(runCtx, provider, followUpRequest)
		if appendErr := s.appendModelCall(run, sequence+1, "post_delegation", "subagent", followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
			return RunResponse{}, appendErr
		}
		if err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+2, observer)
		}
		if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
			"finish_reason": followUpResponse.FinishReason,
			"phase":         "post_delegation",
		}), observer); err != nil {
			return RunResponse{}, err
		}

		followUpAction := parseAction(followUpResponse.Text)
		if err := validateActionForRole(run.Role, followUpAction); err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+3, observer)
		}
		if followUpAction.Action != "final" || strings.TrimSpace(followUpAction.Answer) == "" {
			return s.failRun(run, plan, task.ID, session.ID, state, errors.New("post-delegation model response did not produce a final answer"), sequence+3, observer)
		}

		finalAnswer = followUpAction.Answer
		sequence += 2
		turnCount++
	}

	for action.Action == "tool" {
		calls, err := toolCallsFromAction(action)
		if err != nil {
			return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+1, observer)
		}
		if len(explicitMemoryCandidates) > 0 && containsTool(calls, "fs.write_file") {
			if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "memory.routed", "memory", map[string]any{
				"count":      len(explicitMemoryCandidates),
				"source":     "tool_intercept",
				"tool_batch": extractToolNames(calls),
			}), observer); err != nil {
				return RunResponse{}, err
			}
			finalAnswer = explicitMemoryAnswer
			action = model.Action{
				Action: "final",
				Answer: finalAnswer,
			}
			sequence++
		} else {
			for _, call := range calls {
				if err := ensureSkillAllowsTool(activeSkill, call.Tool); err != nil {
					return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+1, observer)
				}
				if err := s.DelegationManager.ValidateTools(run, call.Tool); err != nil {
					if appendErr := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.rejected", "delegation", map[string]any{
						"tool":   call.Tool,
						"reason": err.Error(),
					}), observer); appendErr != nil {
						return RunResponse{}, appendErr
					}
					return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+2, observer)
				}
			}

			toolCallIDs := make([]string, len(calls))
			for i := range calls {
				toolCallIDs[i] = harnessruntime.NewID("toolcall")
			}
			if len(calls) > 1 {
				if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "tool.batch.started", "runtime", map[string]any{
					"count": len(calls),
					"tools": extractToolNames(calls),
				}), observer); err != nil {
					return RunResponse{}, err
				}
				sequence++
			}
			calledEvents := make([]harnessruntime.Event, 0, len(calls))
			for i, call := range calls {
				calledEvents = append(calledEvents, s.newEvent(run, task.ID, session.ID, sequence+1+int64(i), "tool.called", "runtime", map[string]any{
					"tool_call_id": toolCallIDs[i],
					"tool":         call.Tool,
					"input":        call.Input,
				}))
			}
			if err := s.appendEvents(calledEvents, observer); err != nil {
				return RunResponse{}, err
			}
			sequence += int64(len(calls))

			toolResults, failures := s.executeToolCalls(runCtx, toolCallIDs, calls)
			resultEvents := make([]harnessruntime.Event, 0, len(calls))
			var firstErr error
			for i, call := range calls {
				if i < len(toolResults) && toolResults[i].Tool != "" {
					resultEvents = append(resultEvents, s.newEvent(run, task.ID, session.ID, sequence+1+int64(len(resultEvents)), "tool.succeeded", "tool", map[string]any{
						"tool_call_id": toolResults[i].ToolCallID,
						"tool":         toolResults[i].Tool,
						"input":        toolResults[i].Input,
						"result":       toolResults[i].Result,
					}))
					updateRetrievalProgress(&retrievalProgress, toolResults[i].Tool, toolResults[i].Result)
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
				resultEvents = append(resultEvents, s.newEvent(run, task.ID, session.ID, sequence+1+int64(len(resultEvents)), "tool.failed", "tool", map[string]any{
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
			if err := s.appendEvents(resultEvents, observer); err != nil {
				return RunResponse{}, err
			}
			sequence += int64(len(resultEvents))
			if firstErr != nil {
				return s.failRun(run, plan, task.ID, session.ID, state, firstErr, sequence+1, observer)
			}
			for _, toolResult := range toolResults {
				if toolResult.Tool != "fs.write_file" {
					continue
				}
				eventType := "fs.file_created"
				if mode, ok := toolResult.Result["write_mode"].(string); ok && mode == "updated" {
					eventType = "fs.file_updated"
				}
				if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, eventType, "tool", toolResult.Result), observer); err != nil {
					return RunResponse{}, err
				}
				sequence++
			}

			recentEvents, err = s.EventStore.ReadAll(run.ID)
			if err != nil {
				return RunResponse{}, err
			}
			workingEvidence = promptpkg.BuildWorkingEvidenceForPrompt(collectSuccessfulToolResults(recentEvents))

			shouldCompact, reason := s.ContextManager.ShouldCompact(harnesscontext.CompactionCheckInput{
				TokenUsage:       len(runPrompt.System) + len(runPrompt.Input),
				TokenBudget:      1600,
				RecentEventCount: len(recentEvents),
				LastToolBytes:    totalToolBytes(toolResults),
			})
			if shouldCompact {
				summary, err := s.ContextManager.Compact(harnesscontext.CompactInput{
					RunID:        run.ID,
					Scope:        "run",
					Plan:         plan,
					CurrentStep:  currentStep,
					RecentEvents: recentEvents,
				})
				if err != nil {
					return RunResponse{}, err
				}
				summaries = append(summaries, summary)
				if err := s.StateStore.SaveSummaries(run.ID, summaries); err != nil {
					return RunResponse{}, err
				}
				if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "context.compacted", "context", map[string]any{
					"summary_id": summary.ID,
					"scope":      summary.Scope,
					"reason":     reason,
				}), observer); err != nil {
					return RunResponse{}, err
				}
				sequence++
			}

			state.ResumePhase = "post_tool"
			state.PendingToolName = ""
			state.PendingToolResult = nil
			state.PendingToolResults = toolResults
			state.UpdatedAt = time.Now()
			if err := s.StateStore.SaveState(state); err != nil {
				return RunResponse{}, err
			}

			followUpPrompt := s.PromptBuilder.BuildFollowUpPrompt(run.Role, task, toolResults, workingEvidence, s.promptToolMetadataForSkill(activeSkill), activeSkill)
			if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
				"provider": run.Provider,
				"model":    run.Model,
				"role":     run.Role,
				"phase":    "post_tool",
				"tools":    extractToolNames(calls),
			}), observer); err != nil {
				return RunResponse{}, err
			}

			followUpRequest := model.Request{
				SystemPrompt: followUpPrompt.System,
				Input:        followUpPrompt.Input,
				Metadata:     followUpPrompt.Metadata,
			}
			followUpResponse, err := s.generateWithModelTimeout(runCtx, provider, followUpRequest)
			if appendErr := s.appendModelCall(run, sequence+1, "post_tool", strings.Join(extractToolNames(calls), ","), followUpRequest, responsePtr(followUpResponse, err), err); appendErr != nil {
				return RunResponse{}, appendErr
			}
			if err != nil {
				return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+2, observer)
			}

			if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
				"finish_reason": followUpResponse.FinishReason,
				"phase":         "post_tool",
			}), observer); err != nil {
				return RunResponse{}, err
			}

			followUpAction := parseAction(followUpResponse.Text)
			if followUpAction.Action == "" {
				return s.failRun(run, plan, task.ID, session.ID, state, errors.New("post-tool model response did not produce a valid action"), sequence+3, observer)
			}
			if err := validateActionForRole(run.Role, followUpAction); err != nil {
				return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+3, observer)
			}
			if decision := decideRetrievalProgress(retrievalProgress, followUpAction); decision.ShouldForceFinal {
				forcedPrompt := s.PromptBuilder.BuildForcedFinalPrompt(
					run.Role,
					task,
					decision.Reason,
					buildRetrievalEvidencePayload(retrievalProgress),
					s.promptToolMetadataForSkill(activeSkill),
					activeSkill,
				)
				if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+3, "model.called", "runtime", map[string]any{
					"provider": run.Provider,
					"model":    run.Model,
					"role":     run.Role,
					"phase":    "forced_final",
					"reason":   decision.Reason,
				}), observer); err != nil {
					return RunResponse{}, err
				}

				forcedRequest := model.Request{
					SystemPrompt: forcedPrompt.System,
					Input:        forcedPrompt.Input,
					Metadata:     forcedPrompt.Metadata,
				}
				forcedResponse, err := s.generateWithModelTimeout(runCtx, provider, forcedRequest)
				if appendErr := s.appendModelCall(run, sequence+3, "forced_final", strings.Join(extractToolNames(calls), ","), forcedRequest, responsePtr(forcedResponse, err), err); appendErr != nil {
					return RunResponse{}, appendErr
				}
				if err != nil {
					return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+4, observer)
				}
				if err := s.appendEvent(s.newEvent(run, task.ID, session.ID, sequence+4, "model.responded", "model", map[string]any{
					"finish_reason": forcedResponse.FinishReason,
					"phase":         "forced_final",
				}), observer); err != nil {
					return RunResponse{}, err
				}

				forcedAction := parseAction(forcedResponse.Text)
				if forcedAction.Action == "" {
					return s.failRun(run, plan, task.ID, session.ID, state, errors.New("forced-final model response did not produce a valid action"), sequence+5, observer)
				}
				if err := validateActionForRole(run.Role, forcedAction); err != nil {
					return s.failRun(run, plan, task.ID, session.ID, state, err, sequence+5, observer)
				}
				if forcedAction.Action != "final" || strings.TrimSpace(forcedAction.Answer) == "" {
					return s.failRun(run, plan, task.ID, session.ID, state, errors.New("forced-final model response did not produce a final answer"), sequence+5, observer)
				}
				followUpAction = forcedAction
				sequence += 4
			} else {
				sequence += 2
			}

			action = followUpAction
			finalAnswer = followUpAction.Answer
			state.ResumePhase = ""
			state.PendingToolName = ""
			state.PendingToolResult = nil
			state.PendingToolResults = nil
			state.UpdatedAt = time.Now()
			if err := s.StateStore.SaveState(state); err != nil {
				return RunResponse{}, err
			}
			turnCount++
			if action.Action == "tool" {
				continue
			}
		}
	}

	if action.Action == "final" && strings.TrimSpace(finalAnswer) == "" {
		return s.failRun(run, plan, task.ID, session.ID, state, errors.New("model returned an empty final answer"), sequence+1, observer)
	}

	if action.Action != "final" && action.Action != "delegate" {
		return s.failRun(run, plan, task.ID, session.ID, state, errors.New("model returned an unsupported action"), sequence+1, observer)
	}

	result := harnessruntime.RunResult{
		RunID:       run.ID,
		Status:      harnessruntime.RunCompleted,
		Output:      finalAnswer,
		CompletedAt: time.Now(),
	}

	currentStep.Status = harnessruntime.StepCompleted
	plan.UpdatedAt = result.CompletedAt
	run.Status = harnessruntime.RunCompleted
	run.TurnCount = turnCount
	run.UpdatedAt = result.CompletedAt
	run.CompletedAt = result.CompletedAt
	state.TurnCount = turnCount
	state.UpdatedAt = result.CompletedAt

	assistantMessage := harnessruntime.SessionMessage{
		ID:        harnessruntime.NewID("msg"),
		SessionID: session.ID,
		RunID:     run.ID,
		Role:      harnessruntime.MessageRoleAssistant,
		Content:   finalAnswer,
		CreatedAt: result.CompletedAt,
	}
	if err := s.StateStore.AppendSessionMessage(assistantMessage); err != nil {
		return RunResponse{}, err
	}

	if err := s.StateStore.SaveResult(result); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SavePlan(plan); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveRun(run); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveState(state); err != nil {
		return RunResponse{}, err
	}

	candidates := append([]harnessruntime.MemoryCandidate{}, explicitMemoryCandidates...)
	candidates = append(candidates, s.MemoryManager.ExtractCandidates(memory.ExtractInput{
		SessionID: session.ID,
		RunID:     run.ID,
		Goal:      task.Instruction,
		Result:    result.Output,
		Provider:  run.Provider,
		Model:     run.Model,
	})...)
	committedEntries := []harnessruntime.MemoryEntry{}
	if err := s.StateStore.SaveRunMemories(harnessruntime.RunMemories{
		RunID:      run.ID,
		Recalled:   recalledMemories,
		Candidates: candidates,
		UpdatedAt:  result.CompletedAt,
	}); err != nil {
		return RunResponse{}, err
	}
	if run.ParentRunID == "" {
		committedEntries, err = s.MemoryManager.CommitCandidates(session.ID, candidates)
		if err != nil {
			return RunResponse{}, err
		}
		if err := s.StateStore.SaveRunMemories(harnessruntime.RunMemories{
			RunID:      run.ID,
			Recalled:   recalledMemories,
			Candidates: candidates,
			Committed:  committedEntries,
			UpdatedAt:  result.CompletedAt,
		}); err != nil {
			return RunResponse{}, err
		}
	}

	finalEvents := []harnessruntime.Event{
		s.newEvent(run, task.ID, session.ID, sequence+1, "assistant.message", "assistant", map[string]any{
			"message_id": assistantMessage.ID,
			"content":    assistantMessage.Content,
		}),
		s.newEvent(run, task.ID, session.ID, sequence+2, "result.generated", "runtime", map[string]any{"bytes": len(result.Output)}),
		s.newEvent(run, task.ID, session.ID, sequence+3, "memory.candidate_extracted", "memory", map[string]any{"count": len(candidates)}),
	}
	offset := int64(3)
	if run.ParentRunID == "" {
		finalEvents = append(finalEvents, s.newEvent(run, task.ID, session.ID, sequence+4, "memory.committed", "memory", map[string]any{"count": len(committedEntries)}))
		offset++
	}
	finalEvents = append(finalEvents,
		s.newEvent(run, task.ID, session.ID, sequence+offset+1, "plan.step.completed", "runtime", map[string]any{"step_id": currentStep.ID}),
		s.newEvent(run, task.ID, session.ID, sequence+offset+2, "run.status_changed", "runtime", map[string]any{"from": harnessruntime.RunRunning, "to": harnessruntime.RunCompleted}),
		s.newEvent(run, task.ID, session.ID, sequence+offset+3, "run.completed", "runtime", map[string]any{"run_id": run.ID}),
	)
	if err := s.appendEvents(finalEvents, observer); err != nil {
		return RunResponse{}, err
	}

	return RunResponse{
		Task:   task,
		Run:    run,
		Result: &result,
	}, nil
}

func (s Services) resolveActiveSkill(req RunRequest) (*skill.Definition, error) {
	if name := strings.TrimSpace(req.Skill); name != "" {
		definition, ok, err := s.SkillRegistry.Resolve(name)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("skill not found: %s", name)
		}
		if err := definition.Metadata.ValidateAllowedTools(s.availableToolSet()); err != nil {
			return nil, err
		}
		return &definition, nil
	}

	definition, ok, err := s.SkillRegistry.Match(req.Instruction)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if err := definition.Metadata.ValidateAllowedTools(s.availableToolSet()); err != nil {
		return nil, err
	}
	return &definition, nil
}

func (s Services) activeSkillForTask(task harnessruntime.Task) (*skill.Definition, error) {
	if task.Metadata == nil {
		return nil, nil
	}
	name := strings.TrimSpace(task.Metadata["skill"])
	if name == "" {
		return nil, nil
	}
	definition, ok, err := s.SkillRegistry.Resolve(name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("skill referenced by task is missing: %s", name)
	}
	if err := definition.Metadata.ValidateAllowedTools(s.availableToolSet()); err != nil {
		return nil, err
	}
	return &definition, nil
}

func allowedToolSet(activeSkill *skill.Definition) map[string]struct{} {
	if activeSkill == nil || len(activeSkill.AllowedTools) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(activeSkill.AllowedTools))
	for _, name := range activeSkill.AllowedTools {
		result[name] = struct{}{}
	}
	return result
}

func ensureSkillAllowsTool(activeSkill *skill.Definition, toolName string) error {
	if activeSkill == nil || len(activeSkill.AllowedTools) == 0 {
		return nil
	}
	for _, allowed := range activeSkill.AllowedTools {
		if allowed == toolName {
			return nil
		}
	}
	return fmt.Errorf("tool %s is not allowed by active skill %s", toolName, activeSkill.Name)
}

func (s Services) appendEvent(event harnessruntime.Event, observer RunObserver) error {
	if err := s.EventStore.Append(event); err != nil {
		return err
	}
	ensureRunObserver(observer).OnRuntimeEvent(event)
	return nil
}

func (s Services) appendEvents(events []harnessruntime.Event, observer RunObserver) error {
	for _, event := range events {
		if err := s.appendEvent(event, observer); err != nil {
			return err
		}
	}
	return nil
}

func (s Services) newEvent(run harnessruntime.Run, taskID, sessionID string, sequence int64, eventType, actor string, payload map[string]any) harnessruntime.Event {
	return harnessruntime.Event{
		ID:        harnessruntime.NewID("evt"),
		RunID:     run.ID,
		SessionID: sessionID,
		TaskID:    taskID,
		Sequence:  sequence,
		Type:      eventType,
		Timestamp: time.Now(),
		Actor:     actor,
		Payload:   payload,
	}
}

func (s Services) failRun(run harnessruntime.Run, plan harnessruntime.Plan, taskID, sessionID string, state harnessruntime.RunState, cause error, sequence int64, observer RunObserver) (RunResponse, error) {
	now := time.Now()
	previousStatus := run.Status
	run.Status = harnessruntime.RunFailed
	run.UpdatedAt = now
	run.CompletedAt = now
	state.UpdatedAt = now
	planUpdated := false
	for i := range plan.Steps {
		if plan.Steps[i].ID == state.CurrentStepID || (state.CurrentStepID == "" && plan.Steps[i].ID == run.CurrentStepID) {
			plan.Steps[i].Status = harnessruntime.StepFailed
			planUpdated = true
			break
		}
	}
	if planUpdated {
		plan.UpdatedAt = now
	}

	if err := s.StateStore.SaveRun(run); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveState(state); err != nil {
		return RunResponse{}, err
	}
	if planUpdated {
		if err := s.StateStore.SavePlan(plan); err != nil {
			return RunResponse{}, err
		}
	}

	failureKind, retryable := classifyRunFailure(cause)

	events := []harnessruntime.Event{
		s.newEvent(run, taskID, sessionID, sequence, "run.status_changed", "runtime", map[string]any{
			"from": previousStatus,
			"to":   harnessruntime.RunFailed,
		}),
		s.newEvent(run, taskID, sessionID, sequence+1, "run.failed", "runtime", map[string]any{
			"error":        cause.Error(),
			"failure_kind": failureKind,
			"retryable":    retryable,
			"step_id":      state.CurrentStepID,
		}),
	}
	if err := s.appendEvents(events, observer); err != nil {
		return RunResponse{}, err
	}

	return RunResponse{}, cause
}

func classifyRunFailure(cause error) (string, bool) {
	var arkErr *arkmodel.Error
	if errors.As(cause, &arkErr) {
		return arkErr.FailureKind(), arkErr.Retryable()
	}
	var unsupportedErr *unsupportedProviderError
	switch {
	case errors.Is(cause, context.DeadlineExceeded):
		return "timeout", true
	case errors.Is(cause, context.Canceled):
		return "canceled", true
	case errors.As(cause, &unsupportedErr):
		return "unsupported_provider", false
	default:
		return "runtime_error", false
	}
}

func toolBytes(content map[string]any) int {
	switch value := content["bytes"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return len(harnessruntime.MustJSON(content))
	}
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

func (s Services) executeToolCalls(ctx context.Context, toolCallIDs []string, calls []model.ToolCall) ([]harnessruntime.ToolCallResult, []toolExecutionError) {
	results := make([]harnessruntime.ToolCallResult, len(calls))
	failures := make([]toolExecutionError, 0)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for i, call := range calls {
		wg.Add(1)
		go func(i int, call model.ToolCall) {
			defer wg.Done()
			result, err := s.ToolExecutor.Execute(ctx, call.Tool, call.Input)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, toolExecutionError{Index: i, Call: call, Err: err})
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
	sort.Slice(failures, func(i, j int) bool { return failures[i].Index < failures[j].Index })
	return results, failures
}

func (s Services) spawnChildRun(parentTask harnessruntime.Task, session harnessruntime.Session, parentRun harnessruntime.Run, task harnessruntime.DelegationTask) (RunResponse, harnessruntime.DelegationResult, error) {
	if parentRun.Role != harnessruntime.RunRoleLead {
		return RunResponse{}, harnessruntime.DelegationResult{}, errors.New("only lead-agent can delegate child runs")
	}
	childTask := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: buildChildInstruction(task),
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
	childPlan, err := s.Planner.CreatePlan(context.Background(), planner.PlanInput{
		RunID:     childRun.ID,
		Goal:      task.Goal,
		Workspace: parentTask.Workspace,
	})
	if err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	state := harnessruntime.RunState{
		RunID:     childRun.ID,
		TurnCount: 0,
		UpdatedAt: time.Now(),
	}

	if err := s.StateStore.SaveTask(childTask); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := s.StateStore.SaveRun(childRun); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := s.StateStore.SavePlan(childPlan); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := s.StateStore.SaveState(state); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}

	nextSequence, err := s.EventStore.NextSequence(childRun.ID)
	if err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	events := []harnessruntime.Event{
		s.newEvent(childRun, childTask.ID, session.ID, nextSequence, "task.created", "system", map[string]any{"task_id": childTask.ID}),
		s.newEvent(childRun, childTask.ID, session.ID, nextSequence+1, "run.created", "system", map[string]any{"status": childRun.Status}),
		s.newEvent(childRun, childTask.ID, session.ID, nextSequence+2, "run.role_assigned", "runtime", map[string]any{"role": childRun.Role}),
		s.newEvent(childRun, childTask.ID, session.ID, nextSequence+3, "plan.created", "planner", map[string]any{"plan_id": childPlan.ID, "version": childPlan.Version}),
	}
	if err := s.appendEvents(events, nil); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
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
	if err := s.DelegationManager.SaveChild(parentRun.ID, initialRecord); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}

	response, err := s.executeRun(childTask, session, childRun, childPlan, state, true, nil)
	if err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	result, err := buildDelegationResult(response)
	if err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	if err := s.DelegationManager.SaveChild(parentRun.ID, delegation.ChildRecord{
		Task:      task,
		Run:       response.Run,
		Result:    result,
		UpdatedAt: time.Now(),
	}); err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	return response, result, nil
}

func buildChildInstruction(task harnessruntime.DelegationTask) string {
	return strings.TrimSpace(task.Goal)
}

func buildDelegationResult(response RunResponse) (harnessruntime.DelegationResult, error) {
	summary := ""
	if response.Result != nil {
		summary = strings.TrimSpace(response.Result.Output)
	}
	if action := parseAction(summary); action.Action == "final" && strings.TrimSpace(action.Answer) != "" {
		summary = strings.TrimSpace(action.Answer)
	}
	result := harnessruntime.DelegationResult{
		ChildRunID:      response.Run.ID,
		Summary:         summary,
		Artifacts:       []harnessruntime.DelegationArtifact{},
		Findings:        []string{},
		Risks:           []string{},
		Recommendations: []string{},
		NeedsReplan:     false,
	}
	if summary == "" {
		return result, errors.New("child run did not return a structured result")
	}
	var decoded struct {
		Summary         string          `json:"summary"`
		Artifacts       json.RawMessage `json:"artifacts"`
		Findings        []string        `json:"findings"`
		Risks           []string        `json:"risks"`
		Recommendations []string        `json:"recommendations"`
		NeedsReplan     bool            `json:"needs_replan"`
	}
	if err := json.Unmarshal([]byte(summary), &decoded); err != nil {
		return result, fmt.Errorf("child run did not return valid structured result json: %w", err)
	}
	if strings.TrimSpace(decoded.Summary) == "" {
		return result, errors.New("child run structured result is missing summary")
	}
	result.Summary = strings.TrimSpace(decoded.Summary)
	artifacts, err := decodeDelegationArtifacts(decoded.Artifacts)
	if err != nil {
		return result, fmt.Errorf("child run returned invalid artifacts: %w", err)
	}
	result.Artifacts = artifacts
	result.Findings = ensureStringSlice(decoded.Findings)
	result.Risks = ensureStringSlice(decoded.Risks)
	result.Recommendations = ensureStringSlice(decoded.Recommendations)
	result.NeedsReplan = decoded.NeedsReplan
	return result, nil
}

func delegationResultContent(result harnessruntime.DelegationResult) map[string]any {
	return map[string]any{
		"summary":         result.Summary,
		"artifacts":       result.Artifacts,
		"findings":        result.Findings,
		"risks":           result.Risks,
		"recommendations": result.Recommendations,
		"needs_replan":    result.NeedsReplan,
		"child_run_id":    result.ChildRunID,
	}
}

func ensureStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func decodeDelegationArtifacts(raw json.RawMessage) ([]harnessruntime.DelegationArtifact, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []harnessruntime.DelegationArtifact{}, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	artifacts := make([]harnessruntime.DelegationArtifact, 0, len(items))
	for _, item := range items {
		var text string
		if err := json.Unmarshal(item, &text); err == nil {
			artifacts = append(artifacts, harnessruntime.DelegationArtifact{Value: strings.TrimSpace(text)})
			continue
		}

		var object map[string]any
		if err := json.Unmarshal(item, &object); err != nil {
			return nil, err
		}
		artifact := harnessruntime.DelegationArtifact{
			Name: asString(object, "name"),
			Path: asString(object, "path"),
			URL:  asString(object, "url"),
		}
		delete(object, "name")
		delete(object, "path")
		delete(object, "url")
		if len(object) > 0 {
			artifact.Extra = object
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func asString(values map[string]any, key string) string {
	raw, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := raw.(string)
	return strings.TrimSpace(text)
}

func mustJSONString(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return harnessruntime.MustJSON(values)
}

func (s Services) generateWithModelTimeout(parent context.Context, provider model.Model, req model.Request) (model.Response, error) {
	timeoutSeconds := s.Config.Model.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 90
	}
	callCtx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	return provider.Generate(callCtx, req)
}

func responsePtr(resp model.Response, err error) *model.Response {
	if err != nil {
		return nil
	}
	return &resp
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

func buildRetrievalEvidencePayload(progress RetrievalProgress) map[string]any {
	payload := map[string]any{
		"search_queries":     progress.SearchQueries,
		"fetched_urls":       progress.FetchedURLs,
		"successful_fetches": progress.SuccessfulFetches,
		"empty_fetches":      progress.EmptyFetches,
		"duplicate_fetches":  progress.DuplicateFetches,
		"distinct_evidence":  progress.DistinctEvidence,
		"retrieved_evidence": progress.Evidence,
	}
	return payload
}
