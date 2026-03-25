package app

import "fmt"

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

	switch run.Status {
	case "completed", "failed", "cancelled":
		return RunResponse{}, fmt.Errorf("run %s is not resumable from status %s", runID, run.Status)
	case "pending":
		return s.executeRun(task, session, run, plan, state, true)
	default:
		return s.executeRun(task, session, run, plan, state, false)
	}
}
