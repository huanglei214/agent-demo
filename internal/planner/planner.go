package planner

import (
	"context"
	"regexp"
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
	goal := strings.TrimSpace(input.Goal)

	subgoals := splitGoalIntoSteps(goal)
	steps := make([]harnessruntime.PlanStep, len(subgoals))
	for i, sg := range subgoals {
		schema := "step-result"
		if i == len(subgoals)-1 {
			schema = "final-answer"
		}
		var deps []string
		if i > 0 {
			deps = []string{steps[i-1].ID}
		}
		steps[i] = harnessruntime.PlanStep{
			ID:              harnessruntime.NewID("step"),
			Title:           stepTitle(sg),
			Description:     sg,
			Status:          harnessruntime.StepPending,
			Delegatable:     isDelegatableGoal(sg),
			EstimatedEffort: "small",
			OutputSchema:    schema,
			Dependencies:    deps,
		}
	}

	return harnessruntime.Plan{
		ID:        harnessruntime.NewID("plan"),
		RunID:     input.RunID,
		Goal:      goal,
		Steps:     steps,
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

	// Update remaining pending steps based on new goal context.
	for i := range plan.Steps {
		if plan.Steps[i].Status == harnessruntime.StepPending || plan.Steps[i].Status == "" {
			plan.Steps[i].Title = stepTitle(plan.Goal)
			plan.Steps[i].Description = plan.Goal
			if plan.Steps[i].Status == "" {
				plan.Steps[i].Status = harnessruntime.StepPending
			}
			break
		}
	}

	return plan, nil
}

// splitGoalIntoSteps decomposes a goal into sub-goals using structural cues.
// Falls back to a single step if no multi-step pattern is detected.
func splitGoalIntoSteps(goal string) []string {
	// Pattern 1: Numbered list "1. ... 2. ... 3. ..."
	if steps := splitByNumberedList(goal); len(steps) > 1 {
		return steps
	}
	// Pattern 2: Chinese sequence words "先...然后/接着...最后..."
	if steps := splitByChineseSequence(goal); len(steps) > 1 {
		return steps
	}
	// Pattern 3: English connectors "first...then...finally..."
	if steps := splitByEnglishSequence(goal); len(steps) > 1 {
		return steps
	}
	return []string{goal}
}

var numberedListPattern = regexp.MustCompile(`(?m)^\s*(\d+)\.\s+`)

func splitByNumberedList(goal string) []string {
	indices := numberedListPattern.FindAllStringIndex(goal, -1)
	if len(indices) < 2 {
		return nil
	}
	steps := make([]string, 0, len(indices))
	for i, loc := range indices {
		start := loc[1] // after "N. "
		var end int
		if i+1 < len(indices) {
			end = indices[i+1][0]
		} else {
			end = len(goal)
		}
		text := strings.TrimSpace(goal[start:end])
		if text != "" {
			steps = append(steps, text)
		}
	}
	return steps
}

func splitByChineseSequence(goal string) []string {
	// Split on sequence markers: 先, 然后, 接着, 最后, 再
	markers := []string{"然后", "接着", "最后", "再"}
	if !strings.Contains(goal, "先") {
		return nil
	}
	parts := []string{}
	remaining := goal
	// Extract the part after "先"
	idx := strings.Index(remaining, "先")
	if idx >= 0 {
		remaining = remaining[idx+len("先"):]
	}
	for {
		bestIdx := -1
		bestMarker := ""
		for _, m := range markers {
			if i := strings.Index(remaining, m); i >= 0 && (bestIdx < 0 || i < bestIdx) {
				bestIdx = i
				bestMarker = m
			}
		}
		if bestIdx < 0 {
			text := strings.TrimSpace(remaining)
			if text != "" {
				parts = append(parts, text)
			}
			break
		}
		text := strings.TrimSpace(remaining[:bestIdx])
		if text != "" {
			parts = append(parts, text)
		}
		remaining = remaining[bestIdx+len(bestMarker):]
	}
	if len(parts) < 2 {
		return nil
	}
	return parts
}

func splitByEnglishSequence(goal string) []string {
	lower := strings.ToLower(goal)
	markers := []string{" then ", " and then ", " finally ", " after that "}
	hasFirst := strings.Contains(lower, "first ")
	hasStepByStep := strings.Contains(lower, "step by step")
	if !hasFirst && !hasStepByStep {
		return nil
	}
	parts := []string{}
	remaining := goal
	if hasFirst {
		idx := strings.Index(lower, "first ")
		if idx >= 0 {
			remaining = remaining[idx+len("first "):]
		}
	}
	for {
		bestIdx := -1
		bestLen := 0
		for _, m := range markers {
			if i := strings.Index(strings.ToLower(remaining), m); i >= 0 && (bestIdx < 0 || i < bestIdx) {
				bestIdx = i
				bestLen = len(m)
			}
		}
		if bestIdx < 0 {
			text := strings.TrimSpace(remaining)
			if text != "" {
				parts = append(parts, text)
			}
			break
		}
		text := strings.TrimSpace(remaining[:bestIdx])
		if text != "" {
			parts = append(parts, text)
		}
		remaining = remaining[bestIdx+bestLen:]
	}
	if len(parts) < 2 {
		return nil
	}
	return parts
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
	case strings.Contains(lower, "summarize") || strings.Contains(goal, "总结") || strings.Contains(goal, "摘要"):
		return "Summarize findings"
	case strings.Contains(lower, "analyze") || strings.Contains(goal, "分析"):
		return "Analyze and assess"
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
