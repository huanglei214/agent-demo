package agent

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type AnswerStreamEventType string

const (
	AnswerStreamEventStart     AnswerStreamEventType = "start"
	AnswerStreamEventDelta     AnswerStreamEventType = "delta"
	AnswerStreamEventCompleted AnswerStreamEventType = "completed"
	AnswerStreamEventFailed    AnswerStreamEventType = "failed"
)

type AnswerStreamEvent struct {
	RunID      string
	SessionID  string
	MessageID  string
	Type       AnswerStreamEventType
	Delta      string
	ErrMessage string
}

type RunObserver interface {
	OnRuntimeEvent(event harnessruntime.Event)
	OnAnswerStreamEvent(event AnswerStreamEvent)
}

type noopRunObserver struct{}

func (noopRunObserver) OnRuntimeEvent(harnessruntime.Event) {}

func (noopRunObserver) OnAnswerStreamEvent(AnswerStreamEvent) {}

func ensureRunObserver(observer RunObserver) RunObserver {
	if observer == nil {
		return noopRunObserver{}
	}
	return observer
}
