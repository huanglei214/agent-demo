package policy_test

import (
	"context"
	"testing"

	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestDelegationPolicyBlocksNestedDelegationFromSubagent(t *testing.T) {
	t.Parallel()

	p := policy.DelegationPolicy{
		Manager: delegation.NewManager(store.NewPaths(t.TempDir())),
	}

	outcome, err := p.AfterModel(context.Background(), &policy.ExecutionContext{
		Run: harnessruntime.Run{
			ID:          "run_child",
			ParentRunID: "run_parent",
			Role:        harnessruntime.RunRoleSubagent,
		},
		CurrentStep: &harnessruntime.PlanStep{
			ID:          "step_1",
			Delegatable: true,
		},
	}, actionPtr("delegate"))
	if err != nil {
		t.Fatalf("after model: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected policy outcome, got nil")
	}
	if outcome.Decision != policy.DecisionBlock {
		t.Fatalf("expected block decision, got %#v", outcome)
	}
	if outcome.Reason != "subagent_cannot_delegate" {
		t.Fatalf("expected subagent_cannot_delegate, got %q", outcome.Reason)
	}
}

func TestDelegationPolicyNoopsForLeadRunOnDelegatableStep(t *testing.T) {
	t.Parallel()

	p := policy.DelegationPolicy{
		Manager: delegation.NewManager(store.NewPaths(t.TempDir())),
	}

	outcome, err := p.AfterModel(context.Background(), &policy.ExecutionContext{
		Run: harnessruntime.Run{
			ID:   "run_lead",
			Role: harnessruntime.RunRoleLead,
		},
		CurrentStep: &harnessruntime.PlanStep{
			ID:          "step_1",
			Delegatable: true,
		},
	}, actionPtr("delegate"))
	if err != nil {
		t.Fatalf("after model: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected continue outcome, got nil")
	}
	if policy.HasEffect(outcome) {
		t.Fatalf("expected no-op outcome, got %#v", outcome)
	}
}

func actionPtr(name string) *model.Action {
	return &model.Action{Action: name}
}
