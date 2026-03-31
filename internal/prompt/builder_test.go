package prompt

import (
	"strings"
	"testing"
	"testing/fstest"

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

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleLead, task, plan, &step, modelContext, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	for _, fragment := range []string{
		"You are operating inside a local Go-based agent harness.",
		"Role: lead-agent.",
		"Task handling rules:",
		"Tooling rules:",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected system prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}

	if prompt.Metadata["role"] != string(harnessruntime.RunRoleLead) {
		t.Fatalf("unexpected metadata: %#v", prompt.Metadata)
	}
}

func TestBuildRunPromptDoesNotDuplicateWorkspaceOrCurrentStep(t *testing.T) {
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
		ID:          "step_1",
		Title:       "Read README",
		Description: "Open README.md",
		Status:      harnessruntime.StepRunning,
	}
	modelContext := harnesscontext.ModelContext{
		Pinned: []harnesscontext.Item{
			{Title: "Goal", Content: "summarize README"},
		},
		Plan: []harnesscontext.Item{
			{Title: "Plan", Content: "plan_id=plan_1 version=1 goal=summarize README"},
			{Title: "Active Step", Content: "step_id=step_1 title=Read README status=running description=Open README.md"},
		},
	}

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleLead, task, plan, &step, modelContext, nil, nil)

	if strings.Contains(prompt.Input, "Current step:") {
		t.Fatalf("expected duplicated current-step block to be removed, got:\n%s", prompt.Input)
	}
	if !strings.Contains(prompt.Input, "Active Step:") {
		t.Fatalf("expected plan context to keep active step, got:\n%s", prompt.Input)
	}
	if !strings.Contains(prompt.Input, "Workspace:\n/workspace") {
		t.Fatalf("expected top-level workspace block to remain, got:\n%s", prompt.Input)
	}
}

func TestInjectTodoContextAddsSnapshotAndRulesForTodoMode(t *testing.T) {
	t.Parallel()

	base := Prompt{
		System: "system",
		Input:  "User instruction:\nsummarize README",
		Metadata: map[string]any{
			"task_id": "task_1",
		},
	}
	prompt := InjectTodoContext(base, harnessruntime.Run{PlanMode: harnessruntime.PlanModeTodo}, harnessruntime.RunState{Todos: []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending}, {ID: "todo_2", Content: "Summarize docs", Status: harnessruntime.TodoInProgress}}})

	for _, fragment := range []string{
		"Todo snapshot:",
		"Read README",
		"Summarize docs",
		"Use the `todo` action for complex tasks when helpful.",
		"Use `set` with an empty list to clear todos.",
	} {
		if !strings.Contains(prompt.Input, fragment) {
			t.Fatalf("expected todo prompt to contain %q, got:\n%s", fragment, prompt.Input)
		}
	}
	if prompt.Metadata["plan_mode"] != string(harnessruntime.PlanModeTodo) {
		t.Fatalf("expected todo plan_mode metadata, got %#v", prompt.Metadata)
	}
	if prompt.Metadata["todo_count"] != 2 {
		t.Fatalf("expected todo_count metadata, got %#v", prompt.Metadata)
	}
}

func TestInjectTodoContextOmitsSnapshotAndRulesForEmptyTodoMode(t *testing.T) {
	t.Parallel()

	base := Prompt{
		System: "system",
		Input:  "User instruction:\nsummarize README",
		Metadata: map[string]any{
			"task_id": "task_1",
		},
	}
	prompt := InjectTodoContext(base, harnessruntime.Run{PlanMode: harnessruntime.PlanModeTodo}, harnessruntime.RunState{Todos: nil})

	if strings.Contains(prompt.Input, "Todo snapshot:") || strings.Contains(prompt.Input, "Todo rules:") {
		t.Fatalf("expected empty todo-mode prompt to omit todo context, got:\n%s", prompt.Input)
	}
	if prompt.Metadata["plan_mode"] != string(harnessruntime.PlanModeTodo) {
		t.Fatalf("expected todo plan_mode metadata, got %#v", prompt.Metadata)
	}
	if prompt.Metadata["todo_count"] != 0 {
		t.Fatalf("expected zero todo_count metadata, got %#v", prompt.Metadata)
	}
}

func TestInjectTodoContextLeavesNoneModeUnchanged(t *testing.T) {
	t.Parallel()

	base := Prompt{System: "system", Input: "User instruction:\nsummarize README", Metadata: map[string]any{"task_id": "task_1"}}
	prompt := InjectTodoContext(base, harnessruntime.Run{PlanMode: harnessruntime.PlanModeNone}, harnessruntime.RunState{Todos: []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending}}})

	if prompt.Input != base.Input {
		t.Fatalf("expected none mode prompt input to stay unchanged, got:\n%s", prompt.Input)
	}
	if _, ok := prompt.Metadata["plan_mode"]; ok {
		t.Fatalf("expected none mode metadata to stay unchanged, got %#v", prompt.Metadata)
	}
}

func TestLoadTemplatesReadsEmbeddedFiles(t *testing.T) {
	t.Parallel()

	loaded, err := loadTemplates(fstest.MapFS{
		"templates/base.txt":                       {Data: []byte("base")},
		"templates/lead_role.txt":                  {Data: []byte("lead")},
		"templates/subagent_role.txt":              {Data: []byte("sub")},
		"templates/lead_task_guidance.txt":         {Data: []byte("lead task")},
		"templates/subagent_task_guidance.txt":     {Data: []byte("sub task")},
		"templates/lead_follow_up_rule.txt":        {Data: []byte("lead follow")},
		"templates/subagent_follow_up_rule.txt":    {Data: []byte("sub follow")},
		"templates/lead_forced_final_rule.txt":     {Data: []byte("lead force")},
		"templates/subagent_forced_final_rule.txt": {Data: []byte("sub force")},
	})
	if err != nil {
		t.Fatalf("loadTemplates returned error: %v", err)
	}

	if loaded.base != "base" || loaded.leadRole != "lead" || loaded.subagentForcedFinalRule != "sub force" {
		t.Fatalf("unexpected loaded templates: %#v", loaded)
	}
}

func TestNewBuilderLoadsExternalizedTemplates(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	if !strings.Contains(builder.templates.base, "local Go-based agent harness") {
		t.Fatalf("expected embedded base template to be loaded, got %#v", builder.templates)
	}
	if !strings.Contains(builder.templates.leadRole, "Role: lead-agent.") {
		t.Fatalf("expected embedded lead role template to be loaded, got %#v", builder.templates)
	}
}

func TestBuildFollowUpPromptIncludesWorkingEvidence(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize README",
	}

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result:     map[string]any{"path": "README.md"},
	}}, map[string]any{
		"fs.read_file": []map[string]any{{"tool_call_id": "toolcall_1", "result": map[string]any{"path": "README.md"}}},
	}, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	if !strings.Contains(prompt.Input, "Working evidence:") {
		t.Fatalf("expected follow-up prompt to include working evidence, got:\n%s", prompt.Input)
	}
	if prompt.Metadata["new_tool_count"] != 1 {
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

func TestBuildFollowUpPromptSummarizesReadFileContent(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "summarize README"}
	largeContent := strings.Repeat("A", 1500) + strings.Repeat("B", 900)

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result: map[string]any{
			"path":      "README.md",
			"bytes":     len(largeContent),
			"content":   largeContent,
			"truncated": false,
		},
	}}, nil, []map[string]string{{"name": "fs.read_file", "description": "Read a file"}}, nil)

	if strings.Contains(prompt.Input, "\"content\":\"") {
		t.Fatalf("expected raw content to be summarized, got:\n%s", prompt.Input)
	}
	for _, fragment := range []string{
		"content_excerpt",
		"\"path\":\"README.md\"",
		"\"truncated\":true",
	} {
		if !strings.Contains(prompt.Input, fragment) {
			t.Fatalf("expected follow-up prompt to contain %q, got:\n%s", fragment, prompt.Input)
		}
	}
	if !strings.Contains(prompt.Input, "...") {
		t.Fatalf("expected follow-up prompt to include condensed excerpt marker, got:\n%s", prompt.Input)
	}
}

func TestBuildFollowUpPromptSummarizesBashOutput(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "run tests"}
	largeStdout := strings.Repeat("line\n", 600)

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "bash.exec",
		Input:      map[string]any{"command": "go test ./..."},
		Result: map[string]any{
			"command":   "go test ./...",
			"workdir":   ".",
			"exit_code": 1,
			"stdout":    largeStdout,
			"stderr":    "",
			"truncated": false,
			"timed_out": false,
		},
	}}, nil, []map[string]string{{"name": "bash.exec", "description": "Execute a command"}}, nil)

	for _, fragment := range []string{
		"stdout_excerpt",
		"stderr_excerpt",
		"\"command\":\"go test ./...\"",
		"\"truncated\":true",
	} {
		if !strings.Contains(prompt.Input, fragment) {
			t.Fatalf("expected follow-up prompt to contain %q, got:\n%s", fragment, prompt.Input)
		}
	}
	if strings.Contains(prompt.Input, "\"stdout\":\"") {
		t.Fatalf("expected raw stdout to be summarized, got:\n%s", prompt.Input)
	}
}

func TestBuildForcedFinalPromptIncludesRetrievedEvidence(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "比较两个网页里的信息"}

	prompt := builder.BuildForcedFinalPrompt(harnessruntime.RunRoleLead, task, "already have enough retrieved evidence", map[string]any{
		"fetched_urls":       []string{"https://example.com/a", "https://example.com/b"},
		"successful_fetches": 2,
		"retrieved_evidence": []map[string]any{
			{"url": "https://example.com/a", "title": "A"},
			{"url": "https://example.com/b", "title": "B"},
		},
	}, []map[string]string{{"name": "web.fetch", "description": "Fetch a web page"}}, nil)

	for _, fragment := range []string{
		"Forced-final rule:",
		"Do not call any more tools.",
		"Retrieved evidence:",
		"successful_fetches",
	} {
		if !strings.Contains(prompt.System+"\n"+prompt.Input, fragment) {
			t.Fatalf("expected forced-final prompt to contain %q, got system:\n%s\ninput:\n%s", fragment, prompt.System, prompt.Input)
		}
	}
	if prompt.Metadata["forced_final"] != true {
		t.Fatalf("expected forced_final metadata, got %#v", prompt.Metadata)
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

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleLead, task, plan, nil, harnesscontext.ModelContext{}, []map[string]string{
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

func TestBuildRunPromptUsesSubagentRoleTemplate(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_child",
		Instruction: "只做受限子任务",
		Metadata: map[string]string{
			"delegated":                     "true",
			"delegated_allowed_tools":       `["fs.read_file"]`,
			"delegated_constraints":         `["不要继续委派","只返回结构化结果"]`,
			"delegated_completion_criteria": `["返回 summary","返回 needs_replan"]`,
		},
	}

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleSubagent, task, harnessruntime.Plan{}, nil, harnesscontext.ModelContext{}, []map[string]string{
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	for _, fragment := range []string{
		"Role: subagent.",
		"You are not speaking to the end user.",
		"final answer must be a JSON object string",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected subagent prompt to contain %q, got system:\n%s\ninput:\n%s", fragment, prompt.System, prompt.Input)
		}
	}
	if strings.Contains(prompt.System, `prefer {"action":"delegate", ...}`) {
		t.Fatalf("expected subagent system prompt to exclude lead delegation preference, got:\n%s", prompt.System)
	}
	if prompt.Metadata["role"] != string(harnessruntime.RunRoleSubagent) {
		t.Fatalf("unexpected role metadata: %#v", prompt.Metadata)
	}
	for _, fragment := range []string{
		"Delegated task:",
		"- goal: 只做受限子任务",
		"- allowed_tools:",
		"- completion_criteria:",
	} {
		if !strings.Contains(prompt.Input, fragment) {
			t.Fatalf("expected subagent input to contain %q, got:\n%s", fragment, prompt.Input)
		}
	}
	if strings.Contains(prompt.Input, "Conversation History:") {
		t.Fatalf("expected subagent input to exclude conversation history, got:\n%s", prompt.Input)
	}
}

func TestBuildRunPromptIncludesOnlyExplicitTaskLocalContext(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_child",
		Instruction: "分析仓库结构并返回结构化总结",
		Metadata: map[string]string{
			"delegated":                     "true",
			"delegated_allowed_tools":       `["fs.list_dir","fs.read_file"]`,
			"delegated_constraints":         `["不要继续委派","只返回结构化结果"]`,
			"delegated_completion_criteria": `["指出入口文件","总结核心目录"]`,
			"delegated_task_local_context":  `["重点查看 README.md","只分析仓库根目录与 internal/"]`,
		},
	}

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleSubagent, task, harnessruntime.Plan{}, nil, harnesscontext.ModelContext{}, []map[string]string{
		{"name": "fs.list_dir", "description": "List a directory"},
		{"name": "fs.read_file", "description": "Read a file"},
	}, nil)

	for _, fragment := range []string{
		"Relevant task context:",
		"重点查看 README.md",
		"只分析仓库根目录与 internal/",
	} {
		if !strings.Contains(prompt.Input, fragment) {
			t.Fatalf("expected subagent input to contain %q, got:\n%s", fragment, prompt.Input)
		}
	}
}

func TestBuildRunPromptTellsLeadToPreferDelegationWhenUserExplicitlyRequestsIt(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    task.Instruction,
		Version: 1,
	}
	step := harnessruntime.PlanStep{
		ID:          "step_1",
		Title:       "Delegate a bounded child task",
		Description: "分析仓库并返回结构化结论",
		Status:      harnessruntime.StepRunning,
		Delegatable: true,
	}

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleLead, task, plan, &step, harnesscontext.ModelContext{}, []map[string]string{
		{"name": "fs.list_dir", "description": "List a directory"},
	}, nil)

	if !strings.Contains(prompt.System, `If the user explicitly asks you to delegate`) {
		t.Fatalf("expected lead system prompt to mention explicit delegation preference, got:\n%s", prompt.System)
	}
}

func TestBuildFollowUpPromptKeepsSubagentRoleSemantics(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_child", Instruction: "受限子任务"}
	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleSubagent, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result:     map[string]any{"path": "README.md"},
	}}, nil, []map[string]string{{"name": "fs.read_file", "description": "Read a file"}}, nil)

	for _, fragment := range []string{
		"Role: subagent.",
		"Stay within the delegated scope",
		"Do not switch into a user-facing answer style.",
	} {
		if !strings.Contains(prompt.System, fragment) {
			t.Fatalf("expected subagent follow-up prompt to contain %q, got:\n%s", fragment, prompt.System)
		}
	}
	if prompt.Metadata["role"] != string(harnessruntime.RunRoleSubagent) {
		t.Fatalf("unexpected role metadata: %#v", prompt.Metadata)
	}
}
