package policy

func Continue() *PolicyOutcome {
	return &PolicyOutcome{}
}

func HasEffect(outcome *PolicyOutcome) bool {
	if outcome == nil {
		return false
	}

	if outcome.Decision != "" && outcome.Decision != DecisionContinue {
		return true
	}

	return outcome.Reason != "" ||
		outcome.UpdatedAction != nil ||
		outcome.UserMessage != "" ||
		len(outcome.Metadata) > 0
}
