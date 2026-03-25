package app

import (
	"errors"
	"os"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type InspectResponse struct {
	Run    harnessruntime.Run        `json:"run"`
	Plan   harnessruntime.Plan       `json:"plan"`
	State  harnessruntime.RunState   `json:"state"`
	Result *harnessruntime.RunResult `json:"result,omitempty"`
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

	return InspectResponse{
		Run:    run,
		Plan:   plan,
		State:  state,
		Result: result,
	}, nil
}
