package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/planner"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	"github.com/huanglei214/agent-demo/internal/store"
)

type RunRequest struct {
	Instruction string
	Workspace   string
	Provider    string
	Model       string
	MaxTurns    int
	SessionID   string
	Skill       string
	PlanMode    harnessruntime.PlanMode
}

type RunResponse = agent.ExecutionResponse
type RunObserver = agent.RunObserver

func (s Services) StartRun(ctx context.Context, req RunRequest) (RunResponse, error) {
	return s.startRun(ctx, req, nil)
}

func (s Services) StartRunStream(ctx context.Context, req RunRequest, observer RunObserver) (RunResponse, error) {
	return s.startRun(ctx, req, observer)
}

func (s Services) ExecuteRun(ctx context.Context, task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (RunResponse, error) {
	runner, err := s.runner()
	if err != nil {
		return RunResponse{}, err
	}
	return runner.ExecuteRun(ctx, task, session, run, plan, state, activate, observer)
}

func (s Services) startRun(ctx context.Context, req RunRequest, observer RunObserver) (RunResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
		PlanMode:  derivePlanMode(req),
		Status:    harnessruntime.RunPending,
		Provider:  req.Provider,
		Model:     req.Model,
		MaxTurns:  req.MaxTurns,
		TurnCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	plan, err := s.buildStartRunPlan(ctx, req, run, now)
	if err != nil {
		return RunResponse{}, err
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
	sequence := harnessruntime.NewSequenceCursor(nextSequence)

	events := []harnessruntime.Event{
		newEvent(run, task.ID, session.ID, sequence.Next(), "task.created", "system", map[string]any{"task_id": task.ID}),
	}
	if createdSession {
		events = append(events, newEvent(run, task.ID, session.ID, sequence.Next(), "session.created", "system", map[string]any{"session_id": session.ID}))
	}
	events = append(events,
		newEvent(run, task.ID, session.ID, sequence.Next(), "run.created", "system", map[string]any{"status": run.Status}),
		newEvent(run, task.ID, session.ID, sequence.Next(), "run.role_assigned", "runtime", map[string]any{"role": run.Role}),
		newEvent(run, task.ID, session.ID, sequence.Next(), "plan.created", "planner", map[string]any{"plan_id": plan.ID, "version": plan.Version}),
	)
	if err := appendEvents(s.EventStore, events, observer); err != nil {
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
	if err := appendEvent(s.EventStore, newEvent(run, task.ID, session.ID, sequence.Next(), "user.message", "user", map[string]any{
		"message_id": userMessage.ID,
		"content":    userMessage.Content,
	}), observer); err != nil {
		return RunResponse{}, err
	}
	if activeSkill != nil {
		if err := appendEvent(s.EventStore, newEvent(run, task.ID, session.ID, sequence.Next(), "skill.activated", "runtime", map[string]any{
			"name":          activeSkill.Name,
			"scope":         activeSkill.Scope,
			"allowed_tools": activeSkill.AllowedTools,
		}), observer); err != nil {
			return RunResponse{}, err
		}
	}

	runner, err := s.runner()
	if err != nil {
		return RunResponse{}, err
	}
	return runner.ExecuteRun(ctx, task, session, run, plan, state, true, observer)
}

func derivePlanMode(req RunRequest) harnessruntime.PlanMode {
	if mode := normalizePlanMode(req.PlanMode); mode != "" {
		return mode
	}
	if looksLikeTodoModeInstruction(req.Instruction) {
		return harnessruntime.PlanModeTodo
	}
	return harnessruntime.PlanModeNone
}

func normalizePlanMode(mode harnessruntime.PlanMode) harnessruntime.PlanMode {
	switch harnessruntime.PlanMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case harnessruntime.PlanModeNone:
		return harnessruntime.PlanModeNone
	case harnessruntime.PlanModeTodo:
		return harnessruntime.PlanModeTodo
	default:
		return ""
	}
}

func looksLikeTodoModeInstruction(instruction string) bool {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "\n") {
		return true
	}
	if strings.Contains(lower, "step by step") || strings.Contains(lower, "first") && strings.Contains(lower, "then") {
		return true
	}
	if strings.Contains(trimmed, "先") && strings.Contains(trimmed, "再") {
		return true
	}
	if strings.Contains(trimmed, "然后") || strings.Contains(trimmed, "接着") || strings.Contains(trimmed, "最后") {
		return true
	}
	if strings.Contains(trimmed, "1.") && strings.Contains(trimmed, "2.") {
		return true
	}
	return false
}

func (s Services) buildStartRunPlan(ctx context.Context, req RunRequest, run harnessruntime.Run, now time.Time) (harnessruntime.Plan, error) {
	if run.PlanMode != harnessruntime.PlanModeTodo {
		return compatibilityStartRunPlan(req.Instruction, run.ID, now), nil
	}

	if s.Planner == nil {
		return harnessruntime.Plan{}, errors.New("planner not configured")
	}

	plan, err := s.Planner.CreatePlan(ctx, planner.PlanInput{
		RunID:     run.ID,
		Goal:      req.Instruction,
		Workspace: req.Workspace,
	})
	if err != nil {
		return harnessruntime.Plan{}, err
	}
	if len(plan.Steps) == 0 {
		return harnessruntime.Plan{}, errors.New("planner returned an empty plan")
	}
	return plan, nil
}

func compatibilityStartRunPlan(instruction, runID string, now time.Time) harnessruntime.Plan {
	goal := strings.TrimSpace(instruction)
	step := harnessruntime.PlanStep{
		ID:              harnessruntime.NewID("step"),
		Title:           compatibilityStepTitle(goal),
		Description:     goal,
		Status:          harnessruntime.StepPending,
		Delegatable:     compatibilityStepDelegatable(goal),
		EstimatedEffort: "small",
		OutputSchema:    "final-answer",
	}
	return harnessruntime.Plan{
		ID:        harnessruntime.NewID("plan"),
		RunID:     runID,
		Goal:      goal,
		Steps:     []harnessruntime.PlanStep{step},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func compatibilityStepDelegatable(goal string) bool {
	lower := strings.ToLower(goal)
	return strings.Contains(lower, "delegate") ||
		strings.Contains(goal, "委派") ||
		strings.Contains(goal, "子任务") ||
		strings.Contains(goal, "架构") ||
		strings.Contains(goal, "方案") ||
		strings.Contains(lower, "analyze")
}

func compatibilityStepTitle(goal string) string {
	lower := strings.ToLower(goal)
	switch {
	case strings.Contains(lower, "read") || strings.Contains(goal, "读取"):
		return "Read relevant workspace files"
	case strings.Contains(lower, "write") || strings.Contains(goal, "写入"):
		return "Write requested workspace artifact"
	case strings.Contains(lower, "analyze") || strings.Contains(goal, "分析"):
		return "Complete the requested analysis"
	default:
		return "Complete the requested task"
	}
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

func (s Services) availableToolSet() map[string]struct{} {
	descriptors := s.ToolRegistry.Descriptors()
	result := make(map[string]struct{}, len(descriptors))
	for _, item := range descriptors {
		result[item.Name] = struct{}{}
	}
	return result
}

func newEvent(run harnessruntime.Run, taskID, sessionID string, sequence int64, eventType, actor string, payload map[string]any) harnessruntime.Event {
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

func appendEvent(eventStore store.EventStore, event harnessruntime.Event, observer RunObserver) error {
	if err := eventStore.Append(event); err != nil {
		return err
	}
	if observer != nil {
		observer.OnRuntimeEvent(event)
	}
	return nil
}

func appendEvents(eventStore store.EventStore, events []harnessruntime.Event, observer RunObserver) error {
	for _, event := range events {
		if err := appendEvent(eventStore, event, observer); err != nil {
			return err
		}
	}
	return nil
}
