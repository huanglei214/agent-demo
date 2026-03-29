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
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

type RuntimeServices struct {
	Paths      store.Paths
	EventStore store.EventStore
	StateStore store.StateStore
}

type ModelServices struct {
	ModelFactory  model.Factory
	PromptBuilder prompt.Service
}

type AgentServices struct {
	Planner        planner.Planner
	ContextManager harnesscontext.Service
	MemoryManager  memory.Service
}

type ToolServices struct {
	ToolRegistry *toolruntime.Registry
	ToolExecutor toolruntime.Executor
}

type DelegationServices struct {
	DelegationManager delegation.Manager
	SkillRegistry     skill.Registry
}

type Executor struct {
	Config config.Config
	RuntimeServices
	ModelServices
	AgentServices
	ToolServices
	DelegationServices
}

type ExecutionResponse struct {
	Task   harnessruntime.Task       `json:"task"`
	Run    harnessruntime.Run        `json:"run"`
	Result *harnessruntime.RunResult `json:"result,omitempty"`
}

func NewExecutor(
	cfg config.Config,
	runtime RuntimeServices,
	modelServices ModelServices,
	agentServices AgentServices,
	toolServices ToolServices,
	delegationServices DelegationServices,
) Executor {
	return Executor{
		Config:             cfg,
		RuntimeServices:    runtime,
		ModelServices:      modelServices,
		AgentServices:      agentServices,
		ToolServices:       toolServices,
		DelegationServices: delegationServices,
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
