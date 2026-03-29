package service

import (
	"sort"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/model/ark"
	"github.com/huanglei214/agent-demo/internal/model/mock"
	"github.com/huanglei214/agent-demo/internal/planner"
	"github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	"github.com/huanglei214/agent-demo/internal/store"
	"github.com/huanglei214/agent-demo/internal/store/filesystem"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
	bashtool "github.com/huanglei214/agent-demo/internal/tool/bash"
	fstool "github.com/huanglei214/agent-demo/internal/tool/filesystem"
	webtool "github.com/huanglei214/agent-demo/internal/tool/web"
)

type Services struct {
	Config            config.Config
	Paths             store.Paths
	EventStore        store.EventStore
	StateStore        store.StateStore
	Planner           planner.Planner
	DelegationManager delegation.Manager
	SkillRegistry     skill.Registry
	ToolRegistry      *toolruntime.Registry
	Runner            agent.Runner
}

func NewServices(cfg config.Config) Services {
	paths := store.NewPaths(cfg.Runtime.Root)
	toolServices := newToolServices(cfg.Workspace)
	return NewServicesFromParts(
		cfg,
		newRuntimeServices(paths),
		newModelServices(cfg),
		newAgentServices(cfg, paths),
		toolServices,
		newDelegationServices(cfg, paths, toolServices),
	)
}

func NewServicesFromParts(
	cfg config.Config,
	runtimeServices agent.RuntimeServices,
	modelServices agent.ModelServices,
	agentServices agent.AgentServices,
	toolServices agent.ToolServices,
	delegationServices agent.DelegationServices,
) Services {
	executor := agent.NewExecutor(cfg, runtimeServices, modelServices, agentServices, toolServices, delegationServices)
	return Services{
		Config:            cfg,
		Paths:             runtimeServices.Paths,
		EventStore:        runtimeServices.EventStore,
		StateStore:        runtimeServices.StateStore,
		Planner:           agentServices.Planner,
		DelegationManager: delegationServices.DelegationManager,
		SkillRegistry:     delegationServices.SkillRegistry,
		ToolRegistry:      toolServices.ToolRegistry,
		Runner:            executor,
	}
}

func newRuntimeServices(paths store.Paths) agent.RuntimeServices {
	return agent.RuntimeServices{
		Paths:      paths,
		EventStore: filesystem.NewEventStore(paths),
		StateStore: filesystem.NewStateStore(paths),
	}
}

func newModelServices(cfg config.Config) agent.ModelServices {
	return agent.ModelServices{
		ModelFactory:  newModelFactory(cfg),
		PromptBuilder: prompt.NewBuilder(),
	}
}

func newAgentServices(cfg config.Config, paths store.Paths) agent.AgentServices {
	return agent.AgentServices{
		Planner:        planner.New(),
		ContextManager: harnesscontext.NewManager(),
		MemoryManager:  memory.NewManager(paths),
	}
}

func newToolServices(workspace string) agent.ToolServices {
	registry := newToolRegistry(workspace)
	return agent.ToolServices{
		ToolRegistry: registry,
		ToolExecutor: toolruntime.NewExecutor(registry),
	}
}

func newDelegationServices(cfg config.Config, paths store.Paths, toolServices agent.ToolServices) agent.DelegationServices {
	return agent.DelegationServices{
		DelegationManager: delegation.NewManager(paths, delegation.WithAllowedTools(readOnlyToolNames(toolServices.ToolRegistry))),
		SkillRegistry:     skill.NewRegistry(cfg.Workspace),
	}
}

func newModelFactory(cfg config.Config) model.Factory {
	return func() (model.Model, error) {
		switch cfg.Model.Provider {
		case "", "ark":
			provider := ark.New(cfg.Model)
			return provider, nil
		case "mock":
			provider := mock.New()
			return provider, nil
		default:
			return nil, harnessruntime.NewUnsupportedProviderError(cfg.Model.Provider)
		}
	}
}

func newToolRegistry(workspace string) *toolruntime.Registry {
	registry := toolruntime.NewRegistry()
	registry.Register(fstool.NewReadFileTool(workspace))
	registry.Register(fstool.NewWriteFileTool(workspace))
	registry.Register(fstool.NewStrReplaceTool(workspace))
	registry.Register(fstool.NewListDirTool(workspace))
	registry.Register(fstool.NewSearchTool(workspace))
	registry.Register(webtool.NewSearchTool())
	registry.Register(webtool.NewFetchTool())
	registry.Register(bashtool.NewExecTool(workspace))
	return registry
}

func readOnlyToolNames(registry *toolruntime.Registry) []string {
	descriptors := registry.Descriptors()
	result := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.Access == toolruntime.AccessReadOnly {
			result = append(result, descriptor.Name)
		}
	}
	sort.Strings(result)
	return result
}
