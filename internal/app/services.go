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
	paths := store.NewPaths(cfg.Runtime.Root)
	registry := toolruntime.NewRegistry()
	registry.Register(fstool.NewReadFileTool(cfg.Workspace))
	registry.Register(fstool.NewWriteFileTool(cfg.Workspace))
	registry.Register(fstool.NewListDirTool(cfg.Workspace))
	registry.Register(fstool.NewSearchTool(cfg.Workspace))
	registry.Register(fstool.NewStatTool(cfg.Workspace))

	return Services{
		Config:     cfg,
		Paths:      paths,
		EventStore: filesystem.NewEventStore(paths),
		StateStore: filesystem.NewStateStore(paths),
		ModelFactory: func() (model.Model, error) {
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
		},
		Planner:           planner.New(),
		ContextManager:    harnesscontext.NewManager(),
		MemoryManager:     memory.NewManager(paths),
		DelegationManager: delegation.NewManager(paths),
		PromptBuilder:     prompt.NewBuilder(),
		ToolRegistry:      registry,
		ToolExecutor:      toolruntime.NewExecutor(registry),
	}
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
	tools := s.ToolRegistry.List()
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})

	result := make([]ToolDescriptor, 0, len(tools))
	for _, item := range tools {
		result = append(result, ToolDescriptor{
			Name:        item.Name(),
			Description: item.Description(),
		})
	}

	return result
}
