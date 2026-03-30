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

func (s Services) modelCaller() (agent.ModelCaller, error) {
	if s.ModelCaller == nil {
		return nil, errors.New("service model caller not configured")
	}
	return s.ModelCaller, nil
}

func (s Services) GenerateWithModelTimeout(parent context.Context, provider model.Model, req model.Request) (model.Response, error) {
	caller, err := s.modelCaller()
	if err != nil {
		return model.Response{}, err
	}
	return caller.GenerateWithModelTimeout(parent, provider, req)
}
