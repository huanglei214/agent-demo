package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

type Executor struct {
	registry *Registry
}

func NewExecutor(registry *Registry) Executor {
	return Executor{registry: registry}
}

func (e Executor) Execute(ctx context.Context, name string, input map[string]any) (Result, error) {
	tool, ok := e.registry.Get(name)
	if !ok {
		return Result{}, fmt.Errorf("tool not found: %s", name)
	}

	data, err := json.Marshal(input)
	if err != nil {
		return Result{}, err
	}

	return tool.Execute(ctx, data)
}
