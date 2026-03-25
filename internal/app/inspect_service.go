package app

import (
	"errors"
	"os"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type InspectChildRunSummary struct {
	RunID       string                   `json:"run_id"`
	Status      harnessruntime.RunStatus `json:"status"`
	Summary     string                   `json:"summary"`
	NeedsReplan bool                     `json:"needs_replan"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type InspectResponse struct {
	Run           harnessruntime.Run        `json:"run"`
	Plan          harnessruntime.Plan       `json:"plan"`
	State         harnessruntime.RunState   `json:"state"`
	Result        *harnessruntime.RunResult `json:"result,omitempty"`
	CurrentStep   *harnessruntime.PlanStep  `json:"current_step,omitempty"`
	RecentFailure *harnessruntime.Event     `json:"recent_failure,omitempty"`
	ChildRuns     []InspectChildRunSummary  `json:"child_runs"`
	EventCount    int                       `json:"event_count"`
}

func (s Services) InspectRun(runID string) (InspectResponse, error) {
	run, err := s.StateStore.LoadRun(runID)
	if err != nil {
		return InspectResponse{}, err
	}

	plan, err := s.StateStore.LoadPlan(runID)
	if err != nil {
		return InspectResponse{}, err
	}

	state, err := s.StateStore.LoadState(runID)
	if err != nil {
		return InspectResponse{}, err
	}

	var result *harnessruntime.RunResult
	loadedResult, err := s.StateStore.LoadResult(runID)
	if err == nil {
		result = &loadedResult
	} else if !errors.Is(err, os.ErrNotExist) {
		return InspectResponse{}, err
	}

	events, err := s.EventStore.ReadAll(runID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return InspectResponse{}, err
	}
	currentStep := findCurrentStep(plan, state.CurrentStepID, run.CurrentStepID)
	recentFailure := lastFailureEvent(events)
	childRuns, err := s.DelegationManager.ListChildren(runID)
	if err != nil {
		return InspectResponse{}, err
	}
	childSummaries := make([]InspectChildRunSummary, 0, len(childRuns))
	for _, child := range childRuns {
		childSummaries = append(childSummaries, InspectChildRunSummary{
			RunID:       child.Run.ID,
			Status:      child.Run.Status,
			Summary:     child.Result.Summary,
			NeedsReplan: child.Result.NeedsReplan,
			UpdatedAt:   child.UpdatedAt,
		})
	}

	return InspectResponse{
		Run:           run,
		Plan:          plan,
		State:         state,
		Result:        result,
		CurrentStep:   currentStep,
		RecentFailure: recentFailure,
		ChildRuns:     childSummaries,
		EventCount:    len(events),
	}, nil
}

func findCurrentStep(plan harnessruntime.Plan, stateStepID, runStepID string) *harnessruntime.PlanStep {
	targetID := strings.TrimSpace(stateStepID)
	if targetID == "" {
		targetID = strings.TrimSpace(runStepID)
	}
	if targetID == "" {
		return nil
	}
	for i := range plan.Steps {
		if plan.Steps[i].ID == targetID {
			step := plan.Steps[i]
			return &step
		}
	}
	return nil
}

func lastFailureEvent(events []harnessruntime.Event) *harnessruntime.Event {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Type {
		case "run.failed", "tool.failed", "subagent.rejected":
			event := events[i]
			return &event
		}
	}
	return nil
}
