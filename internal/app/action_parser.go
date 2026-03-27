package app

import (
	"encoding/json"
	"strings"

	"github.com/huanglei214/agent-demo/internal/model"
)

func parseAction(text string) model.Action {
	cleaned := cleanActionText(text)
	cleaned = sanitizeActionJSON(cleaned)

	var action model.Action
	if err := json.Unmarshal([]byte(cleaned), &action); err == nil && action.Action != "" {
		return normalizeAction(action)
	}

	return model.Action{
		Action: "final",
		Answer: text,
	}
}

func cleanActionText(text string) string {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func sanitizeActionJSON(text string) string {
	if text == "" {
		return text
	}

	var builder strings.Builder
	builder.Grow(len(text))

	inString := false
	escaped := false
	for _, r := range text {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if inString {
			switch r {
			case '\\':
				builder.WriteRune(r)
				escaped = true
				continue
			case '\n':
				builder.WriteString(`\n`)
				continue
			case '\r':
				builder.WriteString(`\r`)
				continue
			case '\t':
				builder.WriteString(`\t`)
				continue
			}
		}
		if r == '"' {
			inString = !inString
		}
		builder.WriteRune(r)
	}

	return builder.String()
}

func normalizeAction(action model.Action) model.Action {
	switch action.Action {
	case "delegate":
		if strings.TrimSpace(action.DelegationGoal) == "" && action.Subtask != nil {
			action.DelegationGoal = strings.TrimSpace(action.Subtask.Goal)
		}
		return action
	case "final":
		return normalizeFinalAction(action)
	default:
		return action
	}
}

func normalizeFinalAction(action model.Action) model.Action {
	nestedText := cleanActionText(action.Answer)
	if nestedText == "" {
		return action
	}

	var nested model.Action
	if err := json.Unmarshal([]byte(nestedText), &nested); err != nil {
		return action
	}
	if nested.Action != "final" || strings.TrimSpace(nested.Answer) == "" {
		return action
	}

	action.Answer = nested.Answer
	return action
}
