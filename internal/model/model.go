package model

import "context"

type Request struct {
	SystemPrompt string         `json:"system_prompt"`
	Input        string         `json:"input"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Response struct {
	Text         string         `json:"text"`
	FinishReason string         `json:"finish_reason"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Model interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

type Factory func() (Model, error)

type Action struct {
	Action         string         `json:"action"`
	Answer         string         `json:"answer,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	Input          map[string]any `json:"input,omitempty"`
	DelegationGoal string         `json:"delegation_goal,omitempty"`
}
