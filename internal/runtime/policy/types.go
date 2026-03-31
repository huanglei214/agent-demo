package policy

import (
	"context"

	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/retrieval"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type ExecutionMode string

const (
	ExecutionModeStructured ExecutionMode = "structured"
	ExecutionModeResume     ExecutionMode = "resume"
	ExecutionModeDelegated  ExecutionMode = "delegated"
)

type ExecutionFlags struct {
	AllowDelegation     bool
	RequireApproval     bool
	EnableMemoryRecall  bool
	EnableClarification bool
	EnableCompaction    bool
}

type ExecutionContext struct {
	Task    harnessruntime.Task
	Session harnessruntime.Session
	Run     harnessruntime.Run
	State   harnessruntime.RunState

	Plan        *harnessruntime.Plan
	CurrentStep *harnessruntime.PlanStep

	Mode     ExecutionMode
	Flags    ExecutionFlags
	Metadata map[string]any

	Memories           []harnessruntime.MemoryEntry
	Summaries          []harnessruntime.Summary
	RetrievalProgress  retrieval.RetrievalProgress
	WorkingEvidence    map[string]any
	ExplicitCandidates []harnessruntime.MemoryCandidate
	FinalAnswer        string
	TurnCount          int
}

type ActionResultKind string

type PolicyName string

const (
	ActionResultModel      ActionResultKind = "model"
	ActionResultToolBatch  ActionResultKind = "tool_batch"
	ActionResultDelegation ActionResultKind = "delegation"
	ActionResultFinal      ActionResultKind = "final"
)

const (
	PolicyNameDelegation PolicyName = "delegation"
	PolicyNameReplan     PolicyName = "replan"
	PolicyNameRetrieval  PolicyName = "retrieval"
)

type ActionResult struct {
	Kind    ActionResultKind
	Success bool
	Error   error

	ToolCalls   []model.ToolCall
	ToolResults []harnessruntime.ToolCallResult
	Delegation  *harnessruntime.DelegationResult
	Metadata    map[string]any
}

type PolicyDecision string

const (
	DecisionContinue   PolicyDecision = "continue"
	DecisionBlock      PolicyDecision = "block"
	DecisionReplan     PolicyDecision = "replan"
	DecisionForceFinal PolicyDecision = "force_final"
)

type PolicyOutcome struct {
	Decision      PolicyDecision
	Reason        string
	UpdatedAction *model.Action
	UserMessage   string
	Metadata      map[string]any
}

type RuntimePolicy interface {
	Name() string
	BeforeRun(ctx context.Context, exec *ExecutionContext) (*PolicyOutcome, error)
	AfterModel(ctx context.Context, exec *ExecutionContext, action *model.Action) (*PolicyOutcome, error)
	AfterAction(ctx context.Context, exec *ExecutionContext, action model.Action, result ActionResult) (*PolicyOutcome, error)
}
