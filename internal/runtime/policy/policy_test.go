package policy_test

import (
	"testing"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
)

func TestPolicyOutcomeDefaultsToContinueShape(t *testing.T) {
	var outcome policy.PolicyOutcome
	if outcome.Decision != "" {
		t.Fatalf("expected zero-value decision, got %q", outcome.Decision)
	}
}

func TestActionResultKindConstantsAreStable(t *testing.T) {
	if policy.ActionResultToolBatch != "tool_batch" {
		t.Fatalf("unexpected tool batch kind: %q", policy.ActionResultToolBatch)
	}
}

func TestContinueReturnsNoOpOutcome(t *testing.T) {
	outcome := policy.Continue()
	if outcome == nil {
		t.Fatal("expected continue outcome, got nil")
	}
	if policy.HasEffect(outcome) {
		t.Fatalf("expected continue outcome to have no effect, got %#v", outcome)
	}
	if outcome.Decision != "" {
		t.Fatalf("expected continue outcome to keep zero-value decision, got %q", outcome.Decision)
	}
}

func TestHasEffectTreatsPayloadWithEmptyDecisionAsEffect(t *testing.T) {
	outcome := &policy.PolicyOutcome{
		UpdatedAction: &model.Action{Action: "final"},
	}
	if !policy.HasEffect(outcome) {
		t.Fatalf("expected payload-only outcome to count as effect, got %#v", outcome)
	}
}
