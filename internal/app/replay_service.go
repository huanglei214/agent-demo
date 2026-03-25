package app

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

func (s Services) ReplayRun(runID string) ([]harnessruntime.Event, error) {
	return s.EventStore.ReadAll(runID)
}
