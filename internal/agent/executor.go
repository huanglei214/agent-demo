package agent

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	"github.com/huanglei214/agent-demo/internal/prompt"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	"github.com/huanglei214/agent-demo/internal/store"
	"github.com/huanglei214/agent-demo/internal/store/filesystem"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

type Dependencies struct {
	Config            config.Config
	Paths             store.Paths
	EventStore        filesystem.EventStore
	StateStore        filesystem.StateStore
	ModelFactory      model.Factory
	Planner           planner.Planner
	ContextManager    harnesscontext.Service
	MemoryManager     memory.Service
	DelegationManager delegation.Manager
	PromptBuilder     prompt.Service
	SkillRegistry     skill.Registry
	ToolRegistry      *toolruntime.Registry
	ToolExecutor      toolruntime.Executor
}

type Executor struct {
	Config            config.Config
	Paths             store.Paths
	EventStore        filesystem.EventStore
	StateStore        filesystem.StateStore
	ModelFactory      model.Factory
	Planner           planner.Planner
	ContextManager    harnesscontext.Service
	MemoryManager     memory.Service
	DelegationManager delegation.Manager
	PromptBuilder     prompt.Service
	SkillRegistry     skill.Registry
	ToolRegistry      *toolruntime.Registry
	ToolExecutor      toolruntime.Executor
}

type ExecutionResponse struct {
	Task   harnessruntime.Task       `json:"task"`
	Run    harnessruntime.Run        `json:"run"`
	Result *harnessruntime.RunResult `json:"result,omitempty"`
}

func NewExecutor(deps Dependencies) Executor {
	return Executor{
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
		SkillRegistry:     deps.SkillRegistry,
		ToolRegistry:      deps.ToolRegistry,
		ToolExecutor:      deps.ToolExecutor,
	}
}

func (e Executor) executeRun(
	task harnessruntime.Task,
	session harnessruntime.Session,
	run harnessruntime.Run,
	plan harnessruntime.Plan,
	state harnessruntime.RunState,
	appendUserMessage bool,
	observer RunObserver,
) (ExecutionResponse, error) {
	return e.ExecuteRun(task, session, run, plan, state, appendUserMessage, observer)
}

func (e Executor) HandleDelegationAction(
	ctx context.Context,
	exec *runExecution,
	action model.Action,
) (model.Action, error) {
	return e.handleDelegationAction(ctx, exec, action)
}
