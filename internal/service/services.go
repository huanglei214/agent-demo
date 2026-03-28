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

type Dependencies = agent.Dependencies

type Services struct {
	agent.Executor
}

func NewServices(cfg config.Config) Services {
	return NewServicesFromDependencies(NewDependencies(cfg))
}

func NewDependencies(cfg config.Config) Dependencies {
	paths := store.NewPaths(cfg.Runtime.Root)
	registry := newToolRegistry(cfg.Workspace)

	return Dependencies{
		Config:            cfg,
		Paths:             paths,
		EventStore:        filesystem.NewEventStore(paths),
		StateStore:        filesystem.NewStateStore(paths),
		ModelFactory:      newModelFactory(cfg),
		Planner:           planner.New(),
		ContextManager:    harnesscontext.NewManager(),
		MemoryManager:     memory.NewManager(paths),
		DelegationManager: delegation.NewManager(paths, delegation.WithAllowedTools(readOnlyToolNames(registry))),
		PromptBuilder:     prompt.NewBuilder(),
		SkillRegistry:     skill.NewRegistry(cfg.Workspace),
		ToolRegistry:      registry,
		ToolExecutor:      toolruntime.NewExecutor(registry),
	}
}

func NewServicesFromDependencies(deps Dependencies) Services {
	return Services{
		Executor: agent.NewExecutor(agent.Dependencies(deps)),
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
