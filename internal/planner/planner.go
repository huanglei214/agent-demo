package planner

import (
	"context"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type Planner interface {
	CreatePlan(ctx context.Context, input PlanInput) (harnessruntime.Plan, error)
	Replan(ctx context.Context, input ReplanInput) (harnessruntime.Plan, error)
}

type PlanInput struct {
	RunID        string
	Goal         string
	Workspace    string
	ExistingPlan *harnessruntime.Plan
}

type ReplanInput struct {
	RunID    string
	Goal     string
	Previous harnessruntime.Plan
	Reason   string
}

type DeterministicPlanner struct{}

func New() DeterministicPlanner {
	return DeterministicPlanner{}
}

func (p DeterministicPlanner) CreatePlan(ctx context.Context, input PlanInput) (harnessruntime.Plan, error) {
	_ = ctx

	now := time.Now()
	step := harnessruntime.PlanStep{
		ID:              harnessruntime.NewID("step"),
		Title:           stepTitle(input.Goal),
		Description:     strings.TrimSpace(input.Goal),
		Status:          harnessruntime.StepPending,
		Delegatable:     isDelegatableGoal(input.Goal),
		EstimatedCost:   "",
		EstimatedEffort: "small",
		OutputSchema:    "final-answer",
	}

	return harnessruntime.Plan{
		ID:        harnessruntime.NewID("plan"),
		RunID:     input.RunID,
		Goal:      strings.TrimSpace(input.Goal),
		Steps:     []harnessruntime.PlanStep{step},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (p DeterministicPlanner) Replan(ctx context.Context, input ReplanInput) (harnessruntime.Plan, error) {
	_ = ctx

	plan := input.Previous
	plan.Version++
	plan.UpdatedAt = time.Now()
	if strings.TrimSpace(input.Goal) != "" {
		plan.Goal = strings.TrimSpace(input.Goal)
	}

	if len(plan.Steps) == 0 {
		plan.Steps = []harnessruntime.PlanStep{
			{
				ID:              harnessruntime.NewID("step"),
				Title:           stepTitle(plan.Goal),
				Description:     plan.Goal,
				Status:          harnessruntime.StepPending,
				EstimatedEffort: "small",
				OutputSchema:    "final-answer",
			},
		}
		return plan, nil
	}

	plan.Steps[0].Title = stepTitle(plan.Goal)
	plan.Steps[0].Description = plan.Goal
	if plan.Steps[0].Status == "" {
		plan.Steps[0].Status = harnessruntime.StepPending
	}

	return plan, nil
}

func stepTitle(goal string) string {
	lower := strings.ToLower(goal)
	switch {
	case strings.Contains(lower, "delegate") || strings.Contains(goal, "委派") || strings.Contains(goal, "子任务"):
		return "Delegate a bounded child task"
	case strings.Contains(lower, "read") || strings.Contains(goal, "读取"):
		return "Read relevant workspace files"
	case strings.Contains(lower, "list") || strings.Contains(goal, "列出"):
		return "Inspect workspace entries"
	case strings.Contains(lower, "write") || strings.Contains(goal, "写入"):
		return "Write requested workspace artifact"
	default:
		return "Complete the requested task"
	}
}

func isDelegatableGoal(goal string) bool {
	lower := strings.ToLower(goal)
	return strings.Contains(lower, "delegate") ||
		strings.Contains(goal, "委派") ||
		strings.Contains(goal, "子任务") ||
		strings.Contains(goal, "架构") ||
		strings.Contains(goal, "方案") ||
		strings.Contains(lower, "analyze")
}
