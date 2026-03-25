package app

import (
	"errors"
	"fmt"
	"os"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func (s Services) ResumeRun(runID string) (RunResponse, error) {
	run, err := s.StateStore.LoadRun(runID)
	if err != nil {
		return RunResponse{}, err
	}
	plan, err := s.StateStore.LoadPlan(runID)
	if err != nil {
		return RunResponse{}, err
	}
	state, err := s.StateStore.LoadState(runID)
	if err != nil {
		return RunResponse{}, err
	}
	task, err := s.StateStore.LoadTask(run.TaskID)
	if err != nil {
		return RunResponse{}, err
	}
	session, err := s.StateStore.LoadSession(run.SessionID)
	if err != nil {
		return RunResponse{}, err
	}
	if _, err := s.StateStore.LoadResult(runID); err == nil {
		return RunResponse{}, fmt.Errorf("run %s already has a persisted result and cannot be resumed", runID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RunResponse{}, err
	}

	if state.CurrentStepID == "" && run.CurrentStepID != "" {
		state.CurrentStepID = run.CurrentStepID
	}

	switch run.Status {
	case harnessruntime.RunCompleted, harnessruntime.RunFailed, harnessruntime.RunCancelled:
		return RunResponse{}, fmt.Errorf("run %s is not resumable from terminal status %s", runID, run.Status)
	case harnessruntime.RunBlocked:
		return RunResponse{}, fmt.Errorf("run %s is blocked and requires manual intervention before resume", runID)
	case harnessruntime.RunPending:
		return s.executeRun(task, session, run, plan, state, true)
	case harnessruntime.RunRunning:
		return s.executeRun(task, session, run, plan, state, false)
	default:
		return RunResponse{}, fmt.Errorf("run %s is not resumable from status %s", runID, run.Status)
	}
}
