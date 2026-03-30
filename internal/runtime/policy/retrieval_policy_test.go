package policy_test

import (
	"context"
	"testing"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/retrieval"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
)

func TestRetrievalPolicyRequestsForcedFinalWhenNextFetchRepeatsUsefulURL(t *testing.T) {
	t.Parallel()

	execCtx := &policy.ExecutionContext{
		RetrievalProgress: retrieval.RetrievalProgress{
			FetchedURLs:       []string{"https://example.com/weather"},
			SuccessfulFetches: 1,
			Evidence: []retrieval.RetrievalEvidence{{
				URL:            "https://example.com/weather",
				Title:          "Weather",
				ContentExcerpt: "Today is cloudy and 22C.",
			}},
			DistinctEvidence: 1,
		},
	}

	action := model.Action{
		Action: "tool",
		Calls: []model.ToolCall{{
			Tool:  "web.fetch",
			Input: map[string]any{"url": "https://example.com/weather"},
		}},
	}
	result := policy.ActionResult{
		Kind:    policy.ActionResultToolBatch,
		Success: true,
	}

	outcome, err := policy.RetrievalPolicy{}.AfterAction(context.Background(), execCtx, action, result)
	if err != nil {
		t.Fatalf("after action: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected policy outcome, got nil")
	}
	if outcome.Decision != policy.DecisionForceFinal {
		t.Fatalf("expected force-final decision, got %#v", outcome)
	}

	expected := retrieval.DecideProgress(execCtx.RetrievalProgress, action)
	if outcome.Reason != expected.Reason {
		t.Fatalf("expected reason %q, got %q", expected.Reason, outcome.Reason)
	}
}

func TestRetrievalPolicyNoopsOutsideToolBatch(t *testing.T) {
	t.Parallel()

	outcome, err := policy.RetrievalPolicy{}.AfterAction(context.Background(), &policy.ExecutionContext{
		RetrievalProgress: retrieval.RetrievalProgress{
			FetchedURLs:       []string{"https://example.com/weather"},
			SuccessfulFetches: 1,
		},
	}, model.Action{
		Action: "tool",
		Calls: []model.ToolCall{{
			Tool:  "web.fetch",
			Input: map[string]any{"url": "https://example.com/weather"},
		}},
	}, policy.ActionResult{
		Kind:    policy.ActionResultFinal,
		Success: true,
	})
	if err != nil {
		t.Fatalf("after action: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected continue outcome, got nil")
	}
	if policy.HasEffect(outcome) {
		t.Fatalf("expected no-op outcome, got %#v", outcome)
	}
}

func TestRetrievalPolicyNoopsForNonWebFollowUpAction(t *testing.T) {
	t.Parallel()

	outcome, err := policy.RetrievalPolicy{}.AfterAction(context.Background(), &policy.ExecutionContext{
		RetrievalProgress: retrieval.RetrievalProgress{
			FetchedURLs:       []string{"https://example.com/weather"},
			SuccessfulFetches: 2,
		},
	}, model.Action{
		Action: "final",
		Answer: "武汉今天多云，22C。",
	}, policy.ActionResult{
		Kind:    policy.ActionResultToolBatch,
		Success: true,
	})
	if err != nil {
		t.Fatalf("after action: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected continue outcome, got nil")
	}
	if policy.HasEffect(outcome) {
		t.Fatalf("expected no-op outcome, got %#v", outcome)
	}
}
