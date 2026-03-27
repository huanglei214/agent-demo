package prompt

import (
	"strings"
	"testing"

	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
)

func TestBuildRunPromptIncludesFourLayers(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize README",
		Workspace:   "/workspace",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    "summarize README",
		Version: 1,
	}
	step := harnessruntime.PlanStep{
		ID:     "step_1",
		Title:  "Read README",
		Status: harnessruntime.StepRunning,
	}
	modelContext := harnesscontext.ModelContext{
		Pinned: []harnesscontext.Item{{Title: "Goal", Content: "summarize README"}},
	}

	prompt := builder.BuildRunPrompt(task, plan, &step, modelContext, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	for _, fragment := range []string{
		"You are operating inside a local Go-based agent harness.",
		"Role: default-agent.",
		"Task handling rules:",
		"Tooling rules:",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected system prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}

	if prompt.Metadata["role"] != "default-agent" {
		t.Fatalf("unexpected metadata: %#v", prompt.Metadata)
	}
}

func TestBuildFollowUpPromptIncludesSummaries(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize README",
	}

	prompt := builder.BuildFollowUpPrompt(task, "fs.read_file", map[string]any{
		"path": "README.md",
	}, []harnessruntime.Summary{
		{Scope: "run", Content: "summary content"},
	}, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	if !strings.Contains(prompt.Input, "Available summaries:") {
		t.Fatalf("expected follow-up prompt to include summaries, got:\n%s", prompt.Input)
	}
	if prompt.Metadata["summary_count"] != 1 {
		t.Fatalf("unexpected metadata: %#v", prompt.Metadata)
	}
	for _, fragment := range []string{
		"prefer giving the best sourced answer you can instead of searching again",
		"Do not repeat the same search/fetch loop",
		"respond with the best supported answer and note uncertainty",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected follow-up system prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}
}

func TestBuildRunPromptIncludesActiveSkillLayerAndFilteredTools(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "武汉天气怎么样",
		Workspace:   "/workspace",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    "武汉天气怎么样",
		Version: 1,
	}

	activeSkill := &skill.Definition{
		Metadata: skill.Metadata{
			Name:         "weather-lookup",
			Description:  "查询城市实时天气并给出来源",
			AllowedTools: []string{"web.search", "web.fetch"},
		},
		Instructions: "先搜索再读取页面，不要只返回链接。",
	}

	prompt := builder.BuildRunPrompt(task, plan, nil, harnesscontext.ModelContext{}, []map[string]string{
		{"name": "web.search", "description": "Search"},
		{"name": "web.fetch", "description": "Fetch"},
	}, activeSkill)

	for _, fragment := range []string{
		"Active skill: weather-lookup",
		"Skill instructions:",
		"先搜索再读取页面",
		"- web.search: Search",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected system prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}
	if prompt.Metadata["skill"] != "weather-lookup" {
		t.Fatalf("unexpected skill metadata: %#v", prompt.Metadata)
	}
}
