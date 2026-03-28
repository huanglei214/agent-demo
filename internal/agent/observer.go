package agent

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type RunObserver interface {
	OnRuntimeEvent(event harnessruntime.Event)
}

type noopRunObserver struct{}

func (noopRunObserver) OnRuntimeEvent(harnessruntime.Event) {}

func ensureRunObserver(observer RunObserver) RunObserver {
	if observer == nil {
		return noopRunObserver{}
	}
	return observer
}
