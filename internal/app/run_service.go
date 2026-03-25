package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type RunRequest struct {
	Instruction string
	Workspace   string
	Provider    string
	Model       string
	MaxTurns    int
	SessionID   string
}

type RunResponse struct {
	Task   harnessruntime.Task       `json:"task"`
	Run    harnessruntime.Run        `json:"run"`
	Result *harnessruntime.RunResult `json:"result,omitempty"`
}

func (s Services) StartRun(req RunRequest) (RunResponse, error) {
	now := time.Now()

	task := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: req.Instruction,
		Workspace:   req.Workspace,
		CreatedAt:   now,
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
		s.newEvent(run, task.ID, session.ID, sequence+1, "plan.created", "planner", map[string]any{"plan_id": plan.ID, "version": plan.Version}),
	)

	for _, event := range events {
		if err := s.EventStore.Append(event); err != nil {
			return RunResponse{}, err
		}
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
	if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "user.message", "user", map[string]any{
		"message_id": userMessage.ID,
		"content":    userMessage.Content,
	})); err != nil {
		return RunResponse{}, err
	}

	return s.executeRun(task, session, run, plan, state, true)
}

func (s Services) promptToolMetadata() []map[string]string {
	descriptors := s.toolDescriptors()
	result := make([]map[string]string, 0, len(descriptors))
	for _, item := range descriptors {
		result = append(result, map[string]string{
			"name":        item.Name,
			"description": item.Description,
		})
	}
	return result
}

func (s Services) executeRun(task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool) (RunResponse, error) {
	if len(plan.Steps) == 0 {
		return RunResponse{}, errors.New("plan has no steps to execute")
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
		for _, event := range lifecycleEvents {
			if err := s.EventStore.Append(event); err != nil {
				return RunResponse{}, err
			}
		}
		sequence += int64(len(lifecycleEvents))
	}

	recentEvents, err := s.EventStore.ReadAll(run.ID)
	if err != nil {
		return RunResponse{}, err
	}

	recalledMemories, err := s.MemoryManager.Recall(memory.RecallQuery{
		SessionID: session.ID,
		Goal:      task.Instruction,
		Limit:     5,
	})
	if err != nil {
		return RunResponse{}, err
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

	recentMessages, err := s.StateStore.LoadRecentSessionMessages(session.ID, 6)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunResponse{}, err
	}

	modelContext := s.ContextManager.Build(harnesscontext.BuildInput{
		Task:         task,
		Plan:         plan,
		CurrentStep:  currentStep,
		RecentEvents: recentEvents,
		Summaries:    summaries,
		Memories:     recalledMemories,
		Messages:     recentMessages,
	})

	runPrompt := s.PromptBuilder.BuildRunPrompt(task, plan, currentStep, modelContext, s.promptToolMetadata())
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
	for _, event := range preModelEvents {
		if err := s.EventStore.Append(event); err != nil {
			return RunResponse{}, err
		}
	}
	sequence += int64(len(preModelEvents))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	provider, err := s.ModelFactory()
	if err != nil {
		return s.failRun(run, task.ID, session.ID, state, err, sequence+1)
	}

	finalAnswer := ""
	turnCount := state.TurnCount
	action := model.Action{Action: "final", Answer: explicitMemoryAnswer}

	if routedToMemory {
		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "memory.routed", "memory", map[string]any{
			"count": len(explicitMemoryCandidates),
		})); err != nil {
			return RunResponse{}, err
		}
		finalAnswer = explicitMemoryAnswer
		sequence++
	} else {
		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
		})); err != nil {
			return RunResponse{}, err
		}

		modelResponse, err := provider.Generate(ctx, model.Request{
			SystemPrompt: runPrompt.System,
			Input:        runPrompt.Input,
			Metadata:     runPrompt.Metadata,
		})
		if err != nil {
			return s.failRun(run, task.ID, session.ID, state, err, sequence+1)
		}

		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
			"finish_reason": modelResponse.FinishReason,
		})); err != nil {
			return RunResponse{}, err
		}

		action = parseAction(modelResponse.Text)
		finalAnswer = action.Answer
		sequence += 2
		turnCount = state.TurnCount + 1
	}

	if action.Action == "delegate" {
		canDelegate, reason := s.DelegationManager.CanDelegate(ctx, run, *currentStep)
		if !canDelegate {
			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.rejected", "delegation", map[string]any{
				"step_id": currentStep.ID,
				"reason":  reason,
			})); err != nil {
				return RunResponse{}, err
			}
			return s.failRun(run, task.ID, session.ID, state, errors.New("delegation rejected: "+reason), sequence+2)
		}

		delegationGoal := strings.TrimSpace(action.DelegationGoal)
		if delegationGoal == "" {
			delegationGoal = currentStep.Description
		}
		delegationTask := s.DelegationManager.BuildTask(run, plan, *currentStep, delegationGoal, recalledMemories, summaries)
		childResponse, childResult, err := s.spawnChildRun(task, session, run, delegationTask)
		if err != nil {
			return s.failRun(run, task.ID, session.ID, state, err, sequence+1)
		}

		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.spawned", "delegation", map[string]any{
			"child_run_id": childResponse.Run.ID,
			"step_id":      currentStep.ID,
		})); err != nil {
			return RunResponse{}, err
		}
		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "subagent.completed", "delegation", map[string]any{
			"child_run_id":    childResponse.Run.ID,
			"needs_replan":    childResult.NeedsReplan,
			"summary":         childResult.Summary,
			"recommendations": childResult.Recommendations,
		})); err != nil {
			return RunResponse{}, err
		}
		sequence += 2

		if childResult.NeedsReplan {
			replanned, err := s.Planner.Replan(ctx, planner.ReplanInput{
				RunID:    run.ID,
				Goal:     task.Instruction,
				Previous: plan,
				Reason:   "child_result_requested_replan",
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
			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "plan.updated", "planner", map[string]any{
				"plan_id": plan.ID,
				"version": plan.Version,
				"reason":  "child_result_requested_replan",
				"step_id": currentStep.ID,
			})); err != nil {
				return RunResponse{}, err
			}
			sequence++
		}

		followUpPrompt := s.PromptBuilder.BuildFollowUpPrompt(task, "subagent", delegationResultContent(childResult), summaries)
		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
			"provider": run.Provider,
			"model":    run.Model,
			"phase":    "post_delegation",
			"child":    childResponse.Run.ID,
		})); err != nil {
			return RunResponse{}, err
		}

		followUpResponse, err := provider.Generate(ctx, model.Request{
			SystemPrompt: followUpPrompt.System,
			Input:        followUpPrompt.Input,
			Metadata:     followUpPrompt.Metadata,
		})
		if err != nil {
			return s.failRun(run, task.ID, session.ID, state, err, sequence+2)
		}
		if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
			"finish_reason": followUpResponse.FinishReason,
			"phase":         "post_delegation",
		})); err != nil {
			return RunResponse{}, err
		}

		followUpAction := parseAction(followUpResponse.Text)
		if followUpAction.Action != "final" || strings.TrimSpace(followUpAction.Answer) == "" {
			return s.failRun(run, task.ID, session.ID, state, errors.New("post-delegation model response did not produce a final answer"), sequence+3)
		}

		finalAnswer = followUpAction.Answer
		sequence += 2
		turnCount++
	}

	if action.Action == "tool" {
		if action.Tool == "" {
			return s.failRun(run, task.ID, session.ID, state, errors.New("model requested tool execution without tool name"), sequence+1)
		}
		if action.Tool == "fs.write_file" && len(explicitMemoryCandidates) > 0 {
			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "memory.routed", "memory", map[string]any{
				"count":  len(explicitMemoryCandidates),
				"source": "tool_intercept",
				"tool":   action.Tool,
			})); err != nil {
				return RunResponse{}, err
			}
			finalAnswer = explicitMemoryAnswer
			action = model.Action{
				Action: "final",
				Answer: finalAnswer,
			}
			sequence++
		} else {
			if err := s.DelegationManager.ValidateTools(run, action.Tool); err != nil {
				if appendErr := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "subagent.rejected", "delegation", map[string]any{
					"tool":   action.Tool,
					"reason": err.Error(),
				})); appendErr != nil {
					return RunResponse{}, appendErr
				}
				return s.failRun(run, task.ID, session.ID, state, err, sequence+2)
			}

			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "tool.called", "runtime", map[string]any{
				"tool":  action.Tool,
				"input": action.Input,
			})); err != nil {
				return RunResponse{}, err
			}

			toolResult, err := s.ToolExecutor.Execute(ctx, action.Tool, action.Input)
			if err != nil {
				if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "tool.failed", "tool", map[string]any{
					"tool":  action.Tool,
					"error": err.Error(),
				})); err != nil {
					return RunResponse{}, err
				}
				return s.failRun(run, task.ID, session.ID, state, err, sequence+3)
			}

			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "tool.succeeded", "tool", map[string]any{
				"tool":   action.Tool,
				"result": toolResult.Content,
			})); err != nil {
				return RunResponse{}, err
			}

			sequence += 2

			if action.Tool == "fs.write_file" {
				eventType := "fs.file_created"
				if mode, ok := toolResult.Content["write_mode"].(string); ok && mode == "updated" {
					eventType = "fs.file_updated"
				}
				if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, eventType, "tool", toolResult.Content)); err != nil {
					return RunResponse{}, err
				}
				sequence++
			}

			recentEvents, err = s.EventStore.ReadAll(run.ID)
			if err != nil {
				return RunResponse{}, err
			}

			shouldCompact, reason := s.ContextManager.ShouldCompact(harnesscontext.CompactionCheckInput{
				TokenUsage:       len(runPrompt.System) + len(runPrompt.Input),
				TokenBudget:      1600,
				RecentEventCount: len(recentEvents),
				LastToolBytes:    toolBytes(toolResult.Content),
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
				if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "context.compacted", "context", map[string]any{
					"summary_id": summary.ID,
					"scope":      summary.Scope,
					"reason":     reason,
				})); err != nil {
					return RunResponse{}, err
				}
				sequence++
			}

			followUpPrompt := s.PromptBuilder.BuildFollowUpPrompt(task, action.Tool, toolResult.Content, summaries)
			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+1, "model.called", "runtime", map[string]any{
				"provider": run.Provider,
				"model":    run.Model,
				"phase":    "post_tool",
				"tool":     action.Tool,
			})); err != nil {
				return RunResponse{}, err
			}

			followUpResponse, err := provider.Generate(ctx, model.Request{
				SystemPrompt: followUpPrompt.System,
				Input:        followUpPrompt.Input,
				Metadata:     followUpPrompt.Metadata,
			})
			if err != nil {
				return s.failRun(run, task.ID, session.ID, state, err, sequence+2)
			}

			if err := s.EventStore.Append(s.newEvent(run, task.ID, session.ID, sequence+2, "model.responded", "model", map[string]any{
				"finish_reason": followUpResponse.FinishReason,
				"phase":         "post_tool",
			})); err != nil {
				return RunResponse{}, err
			}

			followUpAction := parseAction(followUpResponse.Text)
			if followUpAction.Action != "final" || strings.TrimSpace(followUpAction.Answer) == "" {
				return s.failRun(run, task.ID, session.ID, state, errors.New("post-tool model response did not produce a final answer"), sequence+3)
			}

			finalAnswer = followUpAction.Answer
			sequence += 2
			turnCount++
		}
	}

	if strings.TrimSpace(finalAnswer) == "" {
		return s.failRun(run, task.ID, session.ID, state, errors.New("model returned an empty final answer"), sequence+1)
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
	for _, event := range finalEvents {
		if err := s.EventStore.Append(event); err != nil {
			return RunResponse{}, err
		}
	}

	return RunResponse{
		Task:   task,
		Run:    run,
		Result: &result,
	}, nil
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

func (s Services) failRun(run harnessruntime.Run, taskID, sessionID string, state harnessruntime.RunState, cause error, sequence int64) (RunResponse, error) {
	now := time.Now()
	run.Status = harnessruntime.RunFailed
	run.UpdatedAt = now
	run.CompletedAt = now
	state.UpdatedAt = now

	if err := s.StateStore.SaveRun(run); err != nil {
		return RunResponse{}, err
	}
	if err := s.StateStore.SaveState(state); err != nil {
		return RunResponse{}, err
	}

	events := []harnessruntime.Event{
		s.newEvent(run, taskID, sessionID, sequence, "run.status_changed", "runtime", map[string]any{"to": harnessruntime.RunFailed}),
		s.newEvent(run, taskID, sessionID, sequence+1, "run.failed", "runtime", map[string]any{"error": cause.Error()}),
	}
	for _, event := range events {
		if err := s.EventStore.Append(event); err != nil {
			return RunResponse{}, err
		}
	}

	return RunResponse{}, cause
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

func (s Services) spawnChildRun(parentTask harnessruntime.Task, session harnessruntime.Session, parentRun harnessruntime.Run, task harnessruntime.DelegationTask) (RunResponse, harnessruntime.DelegationResult, error) {
	childTask := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: buildChildInstruction(task),
		Workspace:   parentTask.Workspace,
		CreatedAt:   time.Now(),
	}
	childRun := harnessruntime.Run{
		ID:          harnessruntime.NewID("run"),
		TaskID:      childTask.ID,
		SessionID:   session.ID,
		ParentRunID: parentRun.ID,
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
		s.newEvent(childRun, childTask.ID, session.ID, nextSequence+2, "plan.created", "planner", map[string]any{"plan_id": childPlan.ID, "version": childPlan.Version}),
	}
	for _, event := range events {
		if err := s.EventStore.Append(event); err != nil {
			return RunResponse{}, harnessruntime.DelegationResult{}, err
		}
	}

	initialRecord := delegation.ChildRecord{
		Task: task,
		Run:  childRun,
		Result: harnessruntime.DelegationResult{
			ChildRunID:      childRun.ID,
			Summary:         "",
			Artifacts:       []string{},
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

	response, err := s.executeRun(childTask, session, childRun, childPlan, state, true)
	if err != nil {
		return RunResponse{}, harnessruntime.DelegationResult{}, err
	}
	result := buildDelegationResult(response)
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
	parts := []string{
		"Parent goal:\n" + task.ParentGoal,
		"Child goal:\n" + task.Goal,
		"Plan step:\n" + task.StepTitle + "\n" + task.StepDesc,
	}
	if len(task.Constraints) > 0 {
		parts = append(parts, "Constraints:\n- "+strings.Join(task.Constraints, "\n- "))
	}
	if len(task.ContextMemory) > 0 {
		parts = append(parts, "Selected context:\n- "+strings.Join(task.ContextMemory, "\n- "))
	}
	return strings.Join(parts, "\n\n")
}

func buildDelegationResult(response RunResponse) harnessruntime.DelegationResult {
	summary := ""
	if response.Result != nil {
		summary = strings.TrimSpace(response.Result.Output)
	}
	result := harnessruntime.DelegationResult{
		ChildRunID:      response.Run.ID,
		Summary:         summary,
		Artifacts:       []string{},
		Findings:        []string{},
		Risks:           []string{},
		Recommendations: []string{},
		NeedsReplan:     false,
	}
	if summary != "" {
		var decoded struct {
			Summary         string   `json:"summary"`
			Artifacts       []string `json:"artifacts"`
			Findings        []string `json:"findings"`
			Risks           []string `json:"risks"`
			Recommendations []string `json:"recommendations"`
			NeedsReplan     bool     `json:"needs_replan"`
		}
		if err := json.Unmarshal([]byte(summary), &decoded); err == nil && decoded.Summary != "" {
			result.Summary = decoded.Summary
			result.Artifacts = ensureStringSlice(decoded.Artifacts)
			result.Findings = ensureStringSlice(decoded.Findings)
			result.Risks = ensureStringSlice(decoded.Risks)
			result.Recommendations = ensureStringSlice(decoded.Recommendations)
			result.NeedsReplan = decoded.NeedsReplan
		}
	}
	return result
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
