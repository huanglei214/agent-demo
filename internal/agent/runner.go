package agent

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type Runner interface {
	ExecuteRun(ctx context.Context, task harnessruntime.Task, session harnessruntime.Session, run harnessruntime.Run, plan harnessruntime.Plan, state harnessruntime.RunState, activate bool, observer RunObserver) (ExecutionResponse, error)
	ResumeRun(ctx context.Context, runID string, observer RunObserver) (ExecutionResponse, error)
}

type ModelCaller interface {
	GenerateWithModelTimeout(context.Context, model.Model, model.Request) (model.Response, error)
}

var _ Runner = (*Executor)(nil)
var _ ModelCaller = (*Executor)(nil)
