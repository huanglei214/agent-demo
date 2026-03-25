package app

import (
	"encoding/json"
	"strings"

	"github.com/huanglei214/agent-demo/internal/model"
)

func parseAction(text string) model.Action {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var action model.Action
	if err := json.Unmarshal([]byte(cleaned), &action); err == nil && action.Action != "" {
		return action
	}

	return model.Action{
		Action: "final",
		Answer: text,
	}
}
