# Prompt Context Trim Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove duplicated and low-value prompt context from run and follow-up prompts while preserving executor behavior and prompt metadata semantics.

**Architecture:** Keep the prompt pipeline intact and make the change at the rendering boundary. Trim duplicated run-context fields in `internal/context` and `internal/prompt`, suppress empty todo prompt sections in `internal/prompt/todo.go`, and make follow-up prompts distinguish current tool results from historical evidence by filtering the current batch out of `workingEvidence` before rendering.

**Tech Stack:** Go, package-level prompt/context helpers, existing executor follow-up flow, Go tests in `internal/context`, `internal/prompt`, `internal/service`, and `internal/agent`.

---

## File map

- Modify: `internal/context/manager.go`
  - Remove pinned workspace from rendered model context.
  - Stop rendering `Recent Events` into model input.
- Modify: `internal/prompt/builder.go`
  - Remove duplicated top-level `Current step` block.
  - Filter current-batch tool results out of `Working evidence`.
- Modify: `internal/prompt/todo.go`
  - Keep todo metadata for todo-mode runs.
  - Skip `Todo snapshot` and `Todo rules` when todo list is empty.
- Modify: `internal/context/manager_test.go`
  - Add coverage for pinned-workspace removal and recent-event omission.
- Modify: `internal/prompt/builder_test.go`
  - Add run-prompt de-duplication coverage.
  - Add follow-up evidence de-duplication coverage.
- Modify: `internal/service/run_test.go`
  - Update todo-mode run assertions so empty todo lists do not inject prompt text.
- Verify: `internal/agent/dispatch.go`
  - No code change expected; used to confirm follow-up behavior still consumes historical evidence.

## Verification note

This branch already has an unrelated baseline failure in `go test ./...`:

- `internal/interfaces/http`: `TestAGUIChatDisconnectDoesNotFailRun`

Do **not** use full-repo `go test ./...` as the success gate for this plan. Use the targeted package commands below instead.

### Task 1: Add failing tests for run-prompt de-duplication and empty todo omission

**Files:**
- Modify: `internal/context/manager_test.go`
- Modify: `internal/prompt/builder_test.go`
- Modify: `internal/service/run_test.go`
- Test: `internal/context/manager_test.go`
- Test: `internal/prompt/builder_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write the failing context render test**

Add this test to `internal/context/manager_test.go`:

```go
func TestBuildOmitsWorkspaceFromPinnedContextAndRecentEvents(t *testing.T) {
	t.Parallel()

	manager := NewManager()
	task := harnessruntime.Task{
		ID:          "task_1",
		Instruction: "summarize the repo",
		Workspace:   "/workspace",
	}
	plan := harnessruntime.Plan{
		ID:      "plan_1",
		Goal:    "summarize the repo",
		Version: 1,
		Steps: []harnessruntime.PlanStep{{
			ID:          "step_1",
			Title:       "Read files",
			Description: "Read important files",
			Status:      harnessruntime.StepRunning,
		}},
	}

	rendered := manager.Build(BuildInput{
		Task:        task,
		Plan:        plan,
		CurrentStep: &plan.Steps[0],
		RecentEvents: []harnessruntime.Event{
			{Type: "tool.succeeded", Actor: "tool", Sequence: 3},
		},
	}).Render()

	if !strings.Contains(rendered, "Pinned Context:") {
		t.Fatalf("expected pinned context to remain, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "- Goal: summarize the repo") {
		t.Fatalf("expected goal to remain pinned, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "- Workspace: /workspace") {
		t.Fatalf("expected workspace to be omitted from pinned context, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "Recent Events:") {
		t.Fatalf("expected recent events to be omitted from rendered model context, got:\n%s", rendered)
	}
}
```

- [ ] **Step 2: Write the failing run-prompt de-duplication test**

Add this test to `internal/prompt/builder_test.go`:

```go
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
		Steps: []harnessruntime.PlanStep{{
			ID:          "step_1",
			Title:       "Read README",
			Description: "Open README.md",
			Status:      harnessruntime.StepRunning,
		}},
	}

	manager := harnesscontext.NewManager()
	modelContext := manager.Build(harnesscontext.BuildInput{
		Task:        task,
		Plan:        plan,
		CurrentStep: &plan.Steps[0],
	})

	prompt := builder.BuildRunPrompt(harnessruntime.RunRoleLead, task, plan, &plan.Steps[0], modelContext, nil, nil)

	if got := strings.Count(prompt.Input, "/workspace"); got != 1 {
		t.Fatalf("expected workspace to appear once, got %d in:\n%s", got, prompt.Input)
	}
	if strings.Contains(prompt.Input, "Current step:") {
		t.Fatalf("expected duplicated current-step block to be removed, got:\n%s", prompt.Input)
	}
	if !strings.Contains(prompt.Input, "Active Step:") {
		t.Fatalf("expected plan context to keep active step, got:\n%s", prompt.Input)
	}
}
```

- [ ] **Step 3: Write the failing empty-todo prompt test**

Replace the current todo-mode start-run assertion in `internal/service/run_test.go` with this test:

```go
func TestStartRunOmitsEmptyTodoPromptContextForTodoMode(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	captured := &capturedModelRequest{}
	services := newTestServices(t, config.Load(workspace), func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
		modelServices.ModelFactory = func() (model.Model, error) {
			return &inspectingModel{
				captured: captured,
				response: model.Action{Action: "final", Answer: "done"},
			}, nil
		}
	})

	response, err := services.StartRun(context.Background(), RunRequest{
		Instruction: "先阅读 README.md，再总结项目状态",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    4,
		PlanMode:    harnessruntime.PlanModeTodo,
	})
	if err != nil {
		t.Fatalf("start todo-mode run: %v", err)
	}
	if response.Run.PlanMode != harnessruntime.PlanModeTodo {
		t.Fatalf("expected todo plan mode, got %#v", response.Run)
	}
	if strings.Contains(captured.Input, "Todo snapshot:") || strings.Contains(captured.Input, "Todo rules:") {
		t.Fatalf("expected empty todo-mode prompt to omit todo context, got:\n%s", captured.Input)
	}
	if captured.Metadata["plan_mode"] != string(harnessruntime.PlanModeTodo) {
		t.Fatalf("expected todo plan_mode metadata, got %#v", captured.Metadata)
	}
	if captured.Metadata["todo_count"] != 0 {
		t.Fatalf("expected zero todo_count metadata, got %#v", captured.Metadata)
	}
}
```

- [ ] **Step 4: Run the targeted red tests**

Run:

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/context ./internal/prompt ./internal/service -run 'Test(BuildOmitsWorkspaceFromPinnedContextAndRecentEvents|BuildRunPromptDoesNotDuplicateWorkspaceOrCurrentStep|StartRunOmitsEmptyTodoPromptContextForTodoMode)' -count=1
```

Expected:
- FAIL because pinned workspace still renders
- FAIL because `BuildRunPrompt` still appends `Current step:`
- FAIL because todo-mode runs still inject `Todo snapshot` and `Todo rules` even when the todo list is empty

- [ ] **Step 5: Commit the red tests**

```bash
git add internal/context/manager_test.go internal/prompt/builder_test.go internal/service/run_test.go
git commit -m "test: cover prompt context trimming"
```

### Task 2: Implement run-prompt trimming and empty-todo suppression

**Files:**
- Modify: `internal/context/manager.go`
- Modify: `internal/prompt/builder.go`
- Modify: `internal/prompt/todo.go`
- Test: `internal/context/manager_test.go`
- Test: `internal/prompt/builder_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Remove pinned workspace and hide recent events from rendered model context**

Update `internal/context/manager.go` so `pinned` contains only the goal, and `Render()` does not render `Recent Events`:

```go
func (m Manager) Build(input BuildInput) ModelContext {
	pinned := []Item{{
		Kind:    "instruction",
		Title:   "Goal",
		Content: input.Task.Instruction,
	}}

	planItems := []Item{{
		Kind:    "plan",
		Title:   "Plan",
		Content: fmt.Sprintf("plan_id=%s version=%d goal=%s", input.Plan.ID, input.Plan.Version, input.Plan.Goal),
	}}
	if input.CurrentStep != nil {
		planItems = append(planItems, Item{
			Kind:    "step",
			Title:   "Active Step",
			Content: fmt.Sprintf("step_id=%s title=%s status=%s description=%s", input.CurrentStep.ID, input.CurrentStep.Title, input.CurrentStep.Status, input.CurrentStep.Description),
		})
	}

	// messageItems / memoryItems / summaryItems / recentItems stay unchanged.

	return ModelContext{
		Pinned:    pinned,
		Messages:  messageItems,
		Plan:      planItems,
		Memories:  memoryItems,
		Summaries: summaryItems,
		Recent:    recentItems,
		Metadata: map[string]any{
			"pinned_count":         len(pinned),
			"message_count":        len(messageItems),
			"message_source_count": len(input.Messages),
			"conversation_omitted": messagesOmitted,
			"plan_count":           len(planItems),
			"memory_count":         len(memoryItems),
			"summary_count":        len(summaryItems),
			"recent_count":         len(recentItems),
		},
	}
}

func (m ModelContext) Render() string {
	sections := []struct {
		title string
		items []Item
	}{
		{title: "Pinned Context", items: m.Pinned},
		{title: "Conversation History", items: m.Messages},
		{title: "Plan Context", items: m.Plan},
		{title: "Recalled Memories", items: m.Memories},
		{title: "Summaries", items: m.Summaries},
	}

	var builder strings.Builder
	for _, section := range sections {
		if len(section.items) == 0 {
			continue
		}
		builder.WriteString(section.title)
		builder.WriteString(":\n")
		for _, item := range section.items {
			builder.WriteString("- ")
			builder.WriteString(item.Title)
			builder.WriteString(": ")
			builder.WriteString(item.Content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}
```

- [ ] **Step 2: Remove duplicated current-step block from the run prompt**

Update `internal/prompt/builder.go` so `BuildRunPrompt()` stops appending a top-level `Current step:` block:

```go
func (b Builder) BuildRunPrompt(role harnessruntime.RunRole, task harnessruntime.Task, plan harnessruntime.Plan, currentStep *harnessruntime.PlanStep, modelContext harnesscontext.ModelContext, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.roleLayer(role),
		b.taskGuidance(role),
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))

	inputParts := b.runInputParts(role, task, modelContext)

	return Prompt{
		System: strings.Join(sections, "\n\n"),
		Input:  strings.TrimSpace(strings.Join(inputParts, "\n\n")),
		Metadata: map[string]any{
			"task_id":      task.ID,
			"plan_id":      plan.ID,
			"plan_version": plan.Version,
			"role":         string(normalizeRole(role)),
			"layers":       promptLayers(activeSkill != nil),
			"tool_count":   len(tools),
			"skill":        activeSkillName(activeSkill),
		},
	}
}
```

- [ ] **Step 3: Preserve todo metadata but skip empty todo text blocks**

Update `internal/prompt/todo.go`:

```go
func InjectTodoContext(base Prompt, run harnessruntime.Run, state harnessruntime.RunState) Prompt {
	if run.PlanMode != harnessruntime.PlanModeTodo {
		return base
	}

	if base.Metadata == nil {
		base.Metadata = map[string]any{}
	}
	base.Metadata["plan_mode"] = string(run.PlanMode)
	base.Metadata["todo_count"] = len(state.Todos)

	if len(state.Todos) == 0 {
		return base
	}

	section := strings.Join([]string{
		"Todo snapshot:\n" + harnessruntime.MustJSON(state.Todos),
		"Todo rules:\n- Use the `todo` action for complex tasks when helpful.\n- When updating todos, send the complete latest list with operation `set`.\n- Use `set` with an empty list to clear todos.\n- Do not overuse todos for trivial tasks.",
	}, "\n\n")

	base.Input = strings.TrimSpace(strings.Join([]string{base.Input, section}, "\n\n"))
	return base
}
```

- [ ] **Step 4: Run the affected packages and verify green**

Run:

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/context ./internal/prompt ./internal/service -count=1
```

Expected:
- PASS

- [ ] **Step 5: Commit the implementation**

```bash
git add internal/context/manager.go internal/prompt/builder.go internal/prompt/todo.go internal/context/manager_test.go internal/prompt/builder_test.go internal/service/run_test.go
git commit -m "feat: trim duplicated run prompt context"
```

### Task 3: Add failing tests for follow-up evidence de-duplication

**Files:**
- Modify: `internal/prompt/builder_test.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write the failing “omit duplicated working evidence” test**

Add this test to `internal/prompt/builder_test.go`:

```go
func TestBuildFollowUpPromptOmitsWorkingEvidenceWhenItOnlyRepeatsCurrentBatch(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "summarize README"}

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result:     map[string]any{"path": "README.md"},
	}}, map[string]any{
		"fs.read_file": []map[string]any{{
			"tool_call_id": "toolcall_1",
			"result":       map[string]any{"path": "README.md"},
		}},
	}, nil, nil)

	if !strings.Contains(prompt.Input, "New tool results:") {
		t.Fatalf("expected prompt to keep current tool results, got:\n%s", prompt.Input)
	}
	if strings.Contains(prompt.Input, "Working evidence:") {
		t.Fatalf("expected duplicated working evidence block to be omitted, got:\n%s", prompt.Input)
	}
}
```

- [ ] **Step 2: Write the failing “keep only historical evidence” test**

Add this test to `internal/prompt/builder_test.go`:

```go
func TestBuildFollowUpPromptKeepsOnlyHistoricalWorkingEvidence(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "summarize README"}

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_current",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result:     map[string]any{"path": "README.md"},
	}}, map[string]any{
		"fs.read_file": []map[string]any{{
			"tool_call_id": "toolcall_current",
			"result":       map[string]any{"path": "README.md"},
		}},
		"bash.exec": []map[string]any{{
			"tool_call_id": "toolcall_old",
			"result":       map[string]any{"command": "go test ./...", "exit_code": 0},
		}},
	}, nil, nil)

	if !strings.Contains(prompt.Input, "Working evidence:") {
		t.Fatalf("expected historical evidence to remain, got:\n%s", prompt.Input)
	}

	parts := strings.SplitN(prompt.Input, "Working evidence:\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected working evidence section, got:\n%s", prompt.Input)
	}
	workingSection := parts[1]
	if strings.Contains(workingSection, "toolcall_current") {
		t.Fatalf("expected current batch to be removed from working evidence, got:\n%s", workingSection)
	}
	if !strings.Contains(workingSection, "toolcall_old") {
		t.Fatalf("expected historical evidence to remain, got:\n%s", workingSection)
	}
}
```

- [ ] **Step 3: Write the failing malformed-evidence preservation test**

Add this test to `internal/prompt/builder_test.go`:

```go
func TestBuildFollowUpPromptPreservesMalformedWorkingEvidence(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	task := harnessruntime.Task{ID: "task_1", Instruction: "summarize README"}

	prompt := builder.BuildFollowUpPrompt(harnessruntime.RunRoleLead, task, []harnessruntime.ToolCallResult{{
		ToolCallID: "toolcall_1",
		Tool:       "fs.read_file",
		Input:      map[string]any{"path": "README.md"},
		Result:     map[string]any{"path": "README.md"},
	}}, map[string]any{
		"fs.read_file": "unexpected-shape",
	}, nil, nil)

	if !strings.Contains(prompt.Input, "Working evidence:") {
		t.Fatalf("expected malformed evidence to be preserved instead of dropped, got:\n%s", prompt.Input)
	}
	if !strings.Contains(prompt.Input, "unexpected-shape") {
		t.Fatalf("expected malformed evidence payload to survive unchanged, got:\n%s", prompt.Input)
	}
}
```

- [ ] **Step 4: Run the targeted red tests**

Run:

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/prompt -run 'TestBuildFollowUpPrompt(OmitsWorkingEvidenceWhenItOnlyRepeatsCurrentBatch|KeepsOnlyHistoricalWorkingEvidence|PreservesMalformedWorkingEvidence)' -count=1
```

Expected:
- FAIL because `BuildFollowUpPrompt()` still renders the full `workingEvidence` map unchanged

- [ ] **Step 5: Commit the red tests**

```bash
git add internal/prompt/builder_test.go
git commit -m "test: cover follow-up evidence de-duplication"
```

### Task 4: Implement historical-evidence filtering for follow-up prompts

**Files:**
- Modify: `internal/prompt/builder.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Add a helper that removes current-batch entries from working evidence**

Add these helpers to `internal/prompt/builder.go`:

```go
func historicalWorkingEvidenceForFollowUp(toolResults []harnessruntime.ToolCallResult, workingEvidence map[string]any) map[string]any {
	if len(workingEvidence) == 0 || len(toolResults) == 0 {
		return workingEvidence
	}

	currentIDs := map[string]struct{}{}
	for _, toolResult := range toolResults {
		if id := strings.TrimSpace(toolResult.ToolCallID); id != "" {
			currentIDs[id] = struct{}{}
		}
	}
	if len(currentIDs) == 0 {
		return workingEvidence
	}

	filtered := map[string]any{}
	changed := false
	for toolName, rawEntries := range workingEvidence {
		entries, ok := normalizeWorkingEvidenceEntries(rawEntries)
		if !ok {
			return workingEvidence
		}

		kept := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			toolCallID, _ := entry["tool_call_id"].(string)
			if _, duplicated := currentIDs[strings.TrimSpace(toolCallID)]; duplicated {
				changed = true
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) > 0 {
			filtered[toolName] = kept
		}
	}

	if !changed {
		return workingEvidence
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func normalizeWorkingEvidenceEntries(raw any) ([]map[string]any, bool) {
	switch entries := raw.(type) {
	case []map[string]any:
		return entries, true
	case []any:
		normalized := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			item, ok := entry.(map[string]any)
			if !ok {
				return nil, false
			}
			normalized = append(normalized, item)
		}
		return normalized, true
	default:
		return nil, false
	}
}
```

- [ ] **Step 2: Render only historical evidence in follow-up prompts**

Update `BuildFollowUpPrompt()` in `internal/prompt/builder.go`:

```go
func (b Builder) BuildFollowUpPrompt(role harnessruntime.RunRole, task harnessruntime.Task, toolResults []harnessruntime.ToolCallResult, workingEvidence map[string]any, tools []map[string]string, activeSkill *skill.Definition) Prompt {
	sections := []string{
		b.templates.base,
		b.roleLayer(role),
		b.followUpRule(role),
	}
	if skillLayer := renderSkillLayer(activeSkill); skillLayer != "" {
		sections = append(sections, skillLayer)
	}
	sections = append(sections, renderToolingLayer(tools))
	systemPrompt := strings.Join(sections, "\n\n")

	inputParts := []string{}
	switch normalizeRole(role) {
	case harnessruntime.RunRoleSubagent:
		inputParts = append(inputParts, renderDelegatedTaskInput(task))
	default:
		inputParts = append(inputParts, "Original instruction:\n"+task.Instruction)
	}
	inputParts = append(inputParts, "New tool results:\n"+harnessruntime.MustJSON(summarizeToolResultsForPrompt(toolResults)))

	historicalEvidence := historicalWorkingEvidenceForFollowUp(toolResults, workingEvidence)
	if len(historicalEvidence) > 0 {
		inputParts = append(inputParts, "Working evidence:\n"+harnessruntime.MustJSON(historicalEvidence))
	}

	return Prompt{
		System: systemPrompt,
		Input:  strings.Join(inputParts, "\n\n"),
		Metadata: map[string]any{
			"task_id":       task.ID,
			"role":          string(normalizeRole(role)),
			"layers":        promptLayers(activeSkill != nil),
			"skill":         activeSkillName(activeSkill),
			"tool_count":    len(tools),
			"new_tool_count": len(toolResults),
		},
	}
}
```

- [ ] **Step 3: Run prompt-package tests and the executor-facing packages**

Run:

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/prompt ./internal/agent ./internal/service -count=1
```

Expected:
- PASS

- [ ] **Step 4: Run the full targeted verification set for this feature**

Run:

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/context ./internal/prompt ./internal/service ./internal/agent -count=1
```

Expected:
- PASS

- [ ] **Step 5: Commit the implementation**

```bash
git add internal/prompt/builder.go internal/prompt/builder_test.go internal/context/manager.go internal/context/manager_test.go internal/prompt/todo.go internal/service/run_test.go
git commit -m "feat: de-duplicate follow-up prompt evidence"
```

## Self-review

- Spec coverage:
  - Duplicate `Workspace`: Task 1 and Task 2.
  - Duplicate `Current step`: Task 1 and Task 2.
  - Empty todo prompt omission: Task 1 and Task 2.
  - `Recent Events` removal from rendered prompt: Task 1 and Task 2.
  - `New tool results` / `Working evidence` split: Task 3 and Task 4.
  - Malformed working-evidence preservation: Task 3 and Task 4.
- Placeholder scan:
  - No `TODO`, `TBD`, or deferred “write tests later” instructions remain.
  - Each code-changing step includes exact code or exact commands.
- Type consistency:
  - Follow-up de-duplication uses `historicalWorkingEvidenceForFollowUp` consistently from `BuildFollowUpPrompt`.
  - `normalizeWorkingEvidenceEntries` returns `[]map[string]any` and a boolean to support the “preserve malformed payloads” rule.
  - Todo metadata remains in `Prompt.Metadata` even when prompt text blocks are omitted.
