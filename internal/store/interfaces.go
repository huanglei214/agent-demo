package store

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type EventStore interface {
	Append(event harnessruntime.Event) error
	ReadAll(runID string) ([]harnessruntime.Event, error)
	ReadAfter(runID string, afterSequence int64) ([]harnessruntime.Event, error)
	NextSequence(runID string) (int64, error)
}

type StateStore interface {
	SaveTask(task harnessruntime.Task) error
	SaveSession(session harnessruntime.Session) error
	SaveRun(run harnessruntime.Run) error
	SaveState(state harnessruntime.RunState) error
	SavePlan(plan harnessruntime.Plan) error
	SaveResult(result harnessruntime.RunResult) error
	SaveSummaries(runID string, summaries []harnessruntime.Summary) error
	SaveRunMemories(memories harnessruntime.RunMemories) error
	AppendSessionMessage(message harnessruntime.SessionMessage) error
	AppendModelCall(call harnessruntime.ModelCall) error
	LoadRun(runID string) (harnessruntime.Run, error)
	LoadTask(taskID string) (harnessruntime.Task, error)
	LoadSession(sessionID string) (harnessruntime.Session, error)
	LoadSessionMessages(sessionID string) ([]harnessruntime.SessionMessage, error)
	LoadRecentSessionMessages(sessionID string, limit int) ([]harnessruntime.SessionMessage, error)
	LoadState(runID string) (harnessruntime.RunState, error)
	LoadPlan(runID string) (harnessruntime.Plan, error)
	LoadResult(runID string) (harnessruntime.RunResult, error)
	LoadSummaries(runID string) ([]harnessruntime.Summary, error)
	LoadRunMemories(runID string) (harnessruntime.RunMemories, error)
	LoadModelCalls(runID string) ([]harnessruntime.ModelCall, error)
}
