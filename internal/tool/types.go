package tool

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

type Result struct {
	Content map[string]any `json:"content"`
}
