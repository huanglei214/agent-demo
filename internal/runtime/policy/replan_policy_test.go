package policy_test

import (
	"context"
	"testing"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
)

func TestReplanPolicyRequestsReplanForChildResult(t *testing.T) {
	t.Parallel()

	childResult := harnessruntime.DelegationResult{
		ChildRunID:  "run_child",
		Summary:     "Child inspected the repo and found the original plan is missing a validation step.",
		NeedsReplan: true,
	}

	outcome, err := policy.ReplanPolicy{}.AfterAction(context.Background(), &policy.ExecutionContext{}, model.Action{Action: "delegate"}, policy.ActionResult{
		Kind:       policy.ActionResultDelegation,
		Success:    true,
		Delegation: &childResult,
	})
	if err != nil {
		t.Fatalf("after action: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected policy outcome, got nil")
	}
	if outcome.Decision != policy.DecisionReplan {
		t.Fatalf("expected replan decision, got %#v", outcome)
	}

	expected := planner.DecideChildReplan(childResult)
	if outcome.Reason != expected.Reason {
		t.Fatalf("expected reason %q, got %q", expected.Reason, outcome.Reason)
	}
}
