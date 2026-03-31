package policy

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/model"
)

type DelegationPolicy struct {
	Manager delegation.DelegateChecker
}

func (DelegationPolicy) Name() string { return string(PolicyNameDelegation) }

func (DelegationPolicy) BeforeRun(context.Context, *ExecutionContext) (*PolicyOutcome, error) {
	return Continue(), nil
}

func (p DelegationPolicy) AfterModel(ctx context.Context, exec *ExecutionContext, action *model.Action) (*PolicyOutcome, error) {
	if exec == nil || action == nil || action.Action != "delegate" || exec.CurrentStep == nil || p.Manager == nil {
		return Continue(), nil
	}

	ok, reason := p.Manager.CanDelegate(ctx, exec.Run, *exec.CurrentStep)
	if ok {
		return Continue(), nil
	}

	return &PolicyOutcome{
		Decision: DecisionBlock,
		Reason:   reason,
	}, nil
}

func (DelegationPolicy) AfterAction(context.Context, *ExecutionContext, model.Action, ActionResult) (*PolicyOutcome, error) {
	return Continue(), nil
}
