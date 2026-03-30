package planner

import (
	"strings"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

const replanReasonChildRequested = "child_result_requested_replan"

type ReplanDecision struct {
	ShouldReplan bool
	Reason       string
}

func DecideChildReplan(result harnessruntime.DelegationResult) ReplanDecision {
	if !result.NeedsReplan {
		return ReplanDecision{}
	}

	if !hasChildReplanEvidence(result) {
		return ReplanDecision{}
	}

	return ReplanDecision{
		ShouldReplan: true,
		Reason:       replanReasonChildRequested,
	}
}

func hasChildReplanEvidence(result harnessruntime.DelegationResult) bool {
	return strings.TrimSpace(result.Summary) != "" ||
		len(result.Findings) > 0 ||
		len(result.Risks) > 0 ||
		len(result.Recommendations) > 0
}
