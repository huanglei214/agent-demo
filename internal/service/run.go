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
}

type RunResponse = agent.ExecutionResponse
type RunObserver = agent.RunObserver

func (s Services) StartRun(req RunRequest) (RunResponse, error) {
	return s.startRun(req, nil)
}

func (s Services) StartRunStream(req RunRequest, observer RunObserver) (RunResponse, error) {
	return s.startRun(req, observer)
}

func (s Services) ExecuteRun(task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (RunResponse, error) {
	runner, err := s.runner()
	if err != nil {
		return RunResponse{}, err
	}
	return runner.ExecuteRun(task, session, run, plan, state, activate, observer)
}

func (s Services) startRun(req RunRequest, observer RunObserver) (RunResponse, error) {
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
	return runner.ExecuteRun(task, session, run, plan, state, true, observer)
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
