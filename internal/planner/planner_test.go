package planner

import (
	"context"
	"testing"
)

func TestCreatePlanReturnsStructuredPlan(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID:     "run_1",
		Goal:      "请读取 README.md 并总结当前项目状态",
		Workspace: "/workspace",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if plan.RunID != "run_1" || plan.Version != 1 {
		t.Fatalf("unexpected plan header: %#v", plan)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Title == "" || plan.Steps[0].OutputSchema == "" {
		t.Fatalf("expected structured step fields, got %#v", plan.Steps[0])
	}
}

func TestReplanIncrementsVersion(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "first goal",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	replanned, err := planner.Replan(context.Background(), ReplanInput{
		RunID:    "run_1",
		Goal:     "updated goal",
		Previous: plan,
		Reason:   "new input",
	})
	if err != nil {
		t.Fatalf("replan: %v", err)
	}

	if replanned.Version != 2 || replanned.Goal != "updated goal" {
		t.Fatalf("unexpected replanned plan: %#v", replanned)
	}
}
