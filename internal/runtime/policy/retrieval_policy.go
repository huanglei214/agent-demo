package policy

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/retrieval"
)

type RetrievalPolicy struct{}

func (RetrievalPolicy) Name() string { return string(PolicyNameRetrieval) }

func (RetrievalPolicy) BeforeRun(context.Context, *ExecutionContext) (*PolicyOutcome, error) {
	return Continue(), nil
}

func (RetrievalPolicy) AfterModel(context.Context, *ExecutionContext, *model.Action) (*PolicyOutcome, error) {
	return Continue(), nil
}

func (RetrievalPolicy) AfterAction(_ context.Context, exec *ExecutionContext, action model.Action, result ActionResult) (*PolicyOutcome, error) {
	if exec == nil || result.Kind != ActionResultToolBatch {
		return Continue(), nil
	}

	decision := retrieval.DecideProgress(exec.RetrievalProgress, action)
	if !decision.ShouldForceFinal {
		return Continue(), nil
	}

	return &PolicyOutcome{
		Decision: DecisionForceFinal,
		Reason:   decision.Reason,
	}, nil
}
