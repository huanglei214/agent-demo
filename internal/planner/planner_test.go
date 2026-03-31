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

func TestCreatePlanGeneratesMultipleStepsForNumberedGoal(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "1. Read the README file\n2. Analyze the code structure\n3. Summarize the project",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d: %#v", len(plan.Steps), plan.Steps)
	}
	if plan.Steps[0].OutputSchema != "step-result" {
		t.Fatalf("expected first step schema=step-result, got %q", plan.Steps[0].OutputSchema)
	}
	if plan.Steps[2].OutputSchema != "final-answer" {
		t.Fatalf("expected last step schema=final-answer, got %q", plan.Steps[2].OutputSchema)
	}
}

func TestCreatePlanGeneratesMultipleStepsForChineseSequence(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "先读取 README 文件，然后分析代码结构，最后总结项目状态",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if len(plan.Steps) < 2 {
		t.Fatalf("expected multiple steps for Chinese sequence, got %d: %#v", len(plan.Steps), plan.Steps)
	}
}

func TestCreatePlanFallsToSingleStepForSimpleGoal(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "hello",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step for simple goal, got %d", len(plan.Steps))
	}
}

func TestCreatePlanSetsDependenciesLinearly(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "1. Read files\n2. Analyze them\n3. Write summary",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if len(plan.Steps[0].Dependencies) != 0 {
		t.Fatalf("expected first step to have no dependencies, got %v", plan.Steps[0].Dependencies)
	}
	if len(plan.Steps[1].Dependencies) != 1 || plan.Steps[1].Dependencies[0] != plan.Steps[0].ID {
		t.Fatalf("expected step 2 to depend on step 1, got %v", plan.Steps[1].Dependencies)
	}
	if len(plan.Steps[2].Dependencies) != 1 || plan.Steps[2].Dependencies[0] != plan.Steps[1].ID {
		t.Fatalf("expected step 3 to depend on step 2, got %v", plan.Steps[2].Dependencies)
	}
}

func TestCreatePlanLastStepHasFinalAnswerSchema(t *testing.T) {
	t.Parallel()

	planner := New()
	plan, err := planner.CreatePlan(context.Background(), PlanInput{
		RunID: "run_1",
		Goal:  "First read the code, then summarize the findings",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if len(plan.Steps) < 2 {
		t.Fatalf("expected multiple steps, got %d", len(plan.Steps))
	}
	last := plan.Steps[len(plan.Steps)-1]
	if last.OutputSchema != "final-answer" {
		t.Fatalf("expected last step schema=final-answer, got %q", last.OutputSchema)
	}
	for _, step := range plan.Steps[:len(plan.Steps)-1] {
		if step.OutputSchema != "step-result" {
			t.Fatalf("expected intermediate step schema=step-result, got %q for step %s", step.OutputSchema, step.ID)
		}
	}
}
