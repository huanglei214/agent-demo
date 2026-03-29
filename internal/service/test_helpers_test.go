package service

import (
	"testing"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/store"
)

type testServicesMutator func(
	*agent.RuntimeServices,
	*agent.ModelServices,
	*agent.AgentServices,
	*agent.ToolServices,
	*agent.DelegationServices,
)

func newTestServices(t *testing.T, cfg config.Config, mutate testServicesMutator) Services {
	t.Helper()

	paths := store.NewPaths(cfg.Runtime.Root)
	runtimeServices := newRuntimeServices(paths)
	modelServices := newModelServices(cfg)
	agentServices := newAgentServices(cfg, paths)
	toolServices := newToolServices(cfg.Workspace)
	delegationServices := newDelegationServices(cfg, paths, toolServices)
	if mutate != nil {
		mutate(&runtimeServices, &modelServices, &agentServices, &toolServices, &delegationServices)
	}
	return NewServicesFromParts(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)
}
