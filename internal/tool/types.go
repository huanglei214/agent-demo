package tool

import (
	"context"
	"encoding/json"
)

type AccessMode string

const (
	AccessReadOnly AccessMode = "read_only"
	AccessWrite    AccessMode = "write"
)

type Descriptor struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Access      AccessMode `json:"access"`
}

type Tool interface {
	Name() string
	Description() string
	AccessMode() AccessMode
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

type Result struct {
	Content map[string]any `json:"content"`
}
