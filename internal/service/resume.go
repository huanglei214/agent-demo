package service

func (s Services) ResumeRun(runID string) (RunResponse, error) {
	runner, err := s.runner()
	if err != nil {
		return RunResponse{}, err
	}
	return runner.ResumeRun(runID, nil)
}
