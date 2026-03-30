package agent

import (
	"context"
	"errors"
	"fmt"
	"os"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func (e *Executor) ResumeRun(ctx context.Context, runID string, observer RunObserver) (ExecutionResponse, error) {
	run, err := e.StateStore.LoadRun(runID)
	if err != nil {
		return ExecutionResponse{}, err
	}
	plan, err := e.StateStore.LoadPlan(runID)
	if err != nil {
		return ExecutionResponse{}, err
	}
	state, err := e.StateStore.LoadState(runID)
	if err != nil {
		return ExecutionResponse{}, err
	}
	task, err := e.StateStore.LoadTask(run.TaskID)
	if err != nil {
		return ExecutionResponse{}, err
	}
	session, err := e.StateStore.LoadSession(run.SessionID)
	if err != nil {
		return ExecutionResponse{}, err
	}
	if _, err := e.StateStore.LoadResult(runID); err == nil {
		return ExecutionResponse{}, fmt.Errorf("run %s already has a persisted result and cannot be resumed", runID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return ExecutionResponse{}, err
	}

	if state.CurrentStepID == "" && run.CurrentStepID != "" {
		state.CurrentStepID = run.CurrentStepID
	}

	switch run.Status {
	case harnessruntime.RunCompleted, harnessruntime.RunFailed, harnessruntime.RunCancelled:
		return ExecutionResponse{}, fmt.Errorf("run %s is not resumable from terminal status %s", runID, run.Status)
	case harnessruntime.RunBlocked:
		return ExecutionResponse{}, fmt.Errorf("run %s is blocked and requires manual intervention before resume", runID)
	case harnessruntime.RunPending:
		return e.ExecuteRun(ctx, task, session, run, plan, state, true, observer)
	case harnessruntime.RunRunning:
		return e.ExecuteRun(ctx, task, session, run, plan, state, false, observer)
	default:
		return ExecutionResponse{}, fmt.Errorf("run %s is not resumable from status %s", runID, run.Status)
	}
}
