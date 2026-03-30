package service

import "context"

func (s Services) ResumeRun(ctx context.Context, runID string) (RunResponse, error) {
	runner, err := s.runner()
	if err != nil {
		return RunResponse{}, err
	}
	return runner.ResumeRun(ctx, runID, nil)
}
