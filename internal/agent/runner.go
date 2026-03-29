package agent

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type Runner interface {
	ExecuteRun(task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (ExecutionResponse, error)
	ResumeRun(runID string, observer RunObserver) (ExecutionResponse, error)
}

var _ Runner = Executor{}
