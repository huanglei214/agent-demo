package app

import (
	"sort"

	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/model/ark"
	"github.com/huanglei214/agent-demo/internal/model/mock"
	"github.com/huanglei214/agent-demo/internal/planner"
	"github.com/huanglei214/agent-demo/internal/prompt"
	"github.com/huanglei214/agent-demo/internal/store"
	"github.com/huanglei214/agent-demo/internal/store/filesystem"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
	fstool "github.com/huanglei214/agent-demo/internal/tool/filesystem"
)

type Dependencies struct {
	Config            config.Config
	Paths             store.Paths
	EventStore        filesystem.EventStore
	StateStore        filesystem.StateStore
	ModelFactory      model.Factory
	Planner           planner.Planner
	ContextManager    harnesscontext.Manager
	MemoryManager     memory.Manager
	DelegationManager delegation.Manager
	PromptBuilder     prompt.Builder
	ToolRegistry      *toolruntime.Registry
	ToolExecutor      toolruntime.Executor
}

type Services struct {
	Config            config.Config
	Paths             store.Paths
	EventStore        filesystem.EventStore
	StateStore        filesystem.StateStore
	ModelFactory      model.Factory
	Planner           planner.Planner
	ContextManager    harnesscontext.Manager
	MemoryManager     memory.Manager
	DelegationManager delegation.Manager
	PromptBuilder     prompt.Builder
	ToolRegistry      *toolruntime.Registry
	ToolExecutor      toolruntime.Executor
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
		ToolRegistry:      registry,
		ToolExecutor:      toolruntime.NewExecutor(registry),
	}
}

func NewServicesFromDependencies(deps Dependencies) Services {
	return Services{
		Config:            deps.Config,
		Paths:             deps.Paths,
		EventStore:        deps.EventStore,
		StateStore:        deps.StateStore,
		ModelFactory:      deps.ModelFactory,
		Planner:           deps.Planner,
		ContextManager:    deps.ContextManager,
		MemoryManager:     deps.MemoryManager,
		DelegationManager: deps.DelegationManager,
		PromptBuilder:     deps.PromptBuilder,
		ToolRegistry:      deps.ToolRegistry,
		ToolExecutor:      deps.ToolExecutor,
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
			return nil, errUnsupportedProvider(cfg.Model.Provider)
		}
	}
}

func newToolRegistry(workspace string) *toolruntime.Registry {
	registry := toolruntime.NewRegistry()
	registry.Register(fstool.NewReadFileTool(workspace))
	registry.Register(fstool.NewWriteFileTool(workspace))
	registry.Register(fstool.NewListDirTool(workspace))
	registry.Register(fstool.NewSearchTool(workspace))
	registry.Register(fstool.NewStatTool(workspace))
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

func errUnsupportedProvider(name string) error {
	return &unsupportedProviderError{name: name}
}

type unsupportedProviderError struct {
	name string
}

func (e *unsupportedProviderError) Error() string {
	return "unsupported model provider: " + e.name
}

func (s Services) toolDescriptors() []ToolDescriptor {
	descriptors := s.ToolRegistry.Descriptors()
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Name < descriptors[j].Name
	})

	result := make([]ToolDescriptor, 0, len(descriptors))
	for _, item := range descriptors {
		result = append(result, ToolDescriptor{
			Name:        item.Name,
			Description: item.Description,
			Access:      item.Access,
		})
	}

	return result
}
