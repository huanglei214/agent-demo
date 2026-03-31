package policy

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
)

type ReplanPolicy struct{}

func (ReplanPolicy) Name() string { return string(PolicyNameReplan) }

func (ReplanPolicy) BeforeRun(context.Context, *ExecutionContext) (*PolicyOutcome, error) {
	return Continue(), nil
}

func (ReplanPolicy) AfterModel(context.Context, *ExecutionContext, *model.Action) (*PolicyOutcome, error) {
	return Continue(), nil
}

func (ReplanPolicy) AfterAction(_ context.Context, exec *ExecutionContext, action model.Action, result ActionResult) (*PolicyOutcome, error) {
	if exec == nil {
		return Continue(), nil
	}
	if action.Action != "delegate" || result.Kind != ActionResultDelegation || !result.Success || result.Delegation == nil {
		return Continue(), nil
	}

	decision := planner.DecideChildReplan(*result.Delegation)
	if !decision.ShouldReplan {
		return Continue(), nil
	}

	return &PolicyOutcome{
		Decision: DecisionReplan,
		Reason:   decision.Reason,
	}, nil
}
