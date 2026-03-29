package service

import (
	"context"
	"errors"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/model"
)

func (s Services) runner() (agent.Runner, error) {
	if s.Runner == nil {
		return nil, errors.New("service runner not configured")
	}
	return s.Runner, nil
}

func (s Services) GenerateWithModelTimeout(parent context.Context, provider model.Model, req model.Request) (model.Response, error) {
	runner, err := s.runner()
	if err != nil {
		return model.Response{}, err
	}
	executor, ok := runner.(interface {
		GenerateWithModelTimeout(context.Context, model.Model, model.Request) (model.Response, error)
	})
	if !ok {
		return model.Response{}, errors.New("configured runner does not support model timeout generation")
	}
	return executor.GenerateWithModelTimeout(parent, provider, req)
}
