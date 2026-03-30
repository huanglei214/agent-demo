# Plan Mode Todo Implementation Status

> Status: Implemented in the current worktree on 2026-03-30.
> Verification: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/runtime ./internal/model ./internal/prompt ./internal/agent ./internal/service` and `GOCACHE=/tmp/agent-demo-gocache go test ./...` both pass.
> Notes:
> - `Run.PlanMode` and `RunState.Todos` are persisted, and legacy runs default `plan_mode` to `none` during JSON load.
> - `todo` actions are only accepted in `plan_mode=todo`, replace the full todo snapshot, emit `todo.updated`, and continue the loop with updated prompt context.
> - `none` mode keeps a compatibility planner path; compatibility steps preserve delegatability heuristics so delegation and replan behavior still work.
> - The checklist below is kept as a historical execution record. Commit steps were not executed because this work continued in the shared dirty worktree.

# Historical Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add first-version `none | todo` planning modes so runs can optionally maintain a persistent todo list through an explicit `todo` action without making planning the default control path.

**Architecture:** Store the selected `PlanMode` on `Run`, store current todo items on `RunState`, teach the executor to accept a dedicated `todo` action only in `todo` mode, and emit `todo.updated` events whenever the todo snapshot changes. Keep the existing deterministic planner on a compatibility path while moving default start/resume behavior toward `execution-first`.

**Tech Stack:** Go, existing `internal/service`, `internal/agent`, `internal/runtime`, filesystem-backed state/event stores, Go test

---

## File Map

### New or expanded responsibilities

- `internal/runtime/runtime.go` or the runtime type file that defines `Run` / `RunState`
  - Add `PlanMode`, `TodoStatus`, `TodoItem`
  - Extend `Run` and `RunState`
- `internal/model/model.go`
  - Extend `Action` with todo payload
- `internal/service/run.go`
  - Accept `PlanMode` from request, derive default when omitted
  - Stop forcing deterministic planner onto all runs
- `internal/service/resume.go`
  - Preserve `PlanMode` / todo-aware resume behavior
- `internal/agent/loop.go`
  - Add todo-action branch to the main execution loop
- `internal/agent/dispatch.go`
  - Keep action validation consistent with new todo action path
- `internal/prompt/...`
  - Inject todo snapshot and usage instructions when `plan_mode=todo`
- `internal/store/...`
  - Ensure run/state persistence handles new fields

### Likely tests

- `internal/service/run_test.go`
- `internal/service/resume_test.go`
- `internal/agent/loop_test.go`
- `internal/interfaces/http/...` if request payloads expose `plan_mode`

---

### Task 1: Add runtime types for `PlanMode` and todo state

**Files:**
- Modify: `internal/runtime/*.go` where `Run` and `RunState` are defined
- Test: `internal/runtime` test file if one exists, otherwise `internal/service/run_test.go`

- [ ] **Step 1: Write the failing test for persisted plan mode and todos**

Add a test that saves and reloads a `Run` and `RunState` with:
- `PlanModeTodo`
- one todo item with `pending` status

Assert the loaded objects preserve:
- `run.PlanMode == PlanModeTodo`
- `len(state.Todos) == 1`
- `state.Todos[0].Content == "inspect README"`

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'Test.*PlanMode.*|Test.*Todos.*' -v`

Expected: FAIL because the new fields/types do not exist yet.

- [ ] **Step 3: Add the minimal runtime types**

Implement:

```go
type PlanMode string

const (
    PlanModeNone PlanMode = "none"
    PlanModeTodo PlanMode = "todo"
)

type TodoStatus string

const (
    TodoPending    TodoStatus = "pending"
    TodoInProgress TodoStatus = "in_progress"
    TodoDone       TodoStatus = "done"
)

type TodoItem struct {
    ID        string    `json:"id"`
    Content   string    `json:"content"`
    Status    TodoStatus `json:"status"`
    Priority  int       `json:"priority"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

Extend:

```go
type Run struct {
    ...
    PlanMode PlanMode `json:"plan_mode"`
}

type RunState struct {
    ...
    Todos []TodoItem `json:"todos,omitempty"`
}
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'Test.*PlanMode.*|Test.*Todos.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime internal/service
git commit -m "feat: add runtime plan mode and todo state types"
```

### Task 2: Add `plan_mode` to `RunRequest` and derive default mode

**Files:**
- Modify: `internal/service/run.go`
- Modify: `internal/service/*.go` files that define request decoding if needed
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write failing tests for explicit and derived plan mode**

Add tests that verify:
- explicit `RunRequest{PlanMode: PlanModeTodo}` produces `run.PlanMode == PlanModeTodo`
- missing plan mode on a simple instruction falls back to `PlanModeNone`
- missing plan mode on a clearly multi-step instruction falls back to `PlanModeTodo`

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'TestStartRun.*PlanMode' -v`

Expected: FAIL because `RunRequest` does not carry `PlanMode` yet.

- [ ] **Step 3: Implement `RunRequest.PlanMode` and mode derivation**

Add:

```go
type RunRequest struct {
    ...
    PlanMode harnessruntime.PlanMode
}
```

Implement a helper similar to:

```go
func derivePlanMode(req RunRequest, activeSkill *skill.Definition) harnessruntime.PlanMode
```

Rules:
- explicit request value wins
- skill metadata may provide a default if available
- otherwise apply a small heuristic
- fallback to `PlanModeNone`

The heuristic should only return `PlanModeTodo` for clearly multi-step / analysis-style prompts.

- [ ] **Step 4: Stop forcing deterministic planner for `none` runs**

In `startRun(...)`:
- set `run.PlanMode`
- only call `Planner.CreatePlan(...)` on compatibility paths that still require it
- do not make `CreatePlan(...)` a mandatory path for `PlanModeNone`

If the current executor still requires a plan object, create the smallest compatibility placeholder and clearly isolate that path so the next refactor can remove it.

- [ ] **Step 5: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'TestStartRun.*PlanMode' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service
git commit -m "feat: derive and persist run plan mode"
```

### Task 3: Add todo action payload to model actions

**Files:**
- Modify: `internal/model/*.go` where `Action` is defined
- Modify: JSON parsing tests for model actions if they exist
- Test: `internal/agent/loop_test.go` or model parsing tests

- [ ] **Step 1: Write the failing action parsing test**

Add a test that unmarshals:

```json
{
  "action": "todo",
  "todo": {
    "operation": "set",
    "items": [
      {
        "id": "todo_1",
        "content": "Read README",
        "status": "pending",
        "priority": 1
      }
    ]
  }
}
```

Assert:
- `action.Action == "todo"`
- `action.Todo != nil`
- `action.Todo.Operation == "set"`
- `len(action.Todo.Items) == 1`

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent -run 'Test.*TodoAction.*|Test.*Parse.*Todo.*' -v`

Expected: FAIL because `Action.Todo` does not exist.

- [ ] **Step 3: Add the minimal action fields**

Extend the action model with:

```go
type TodoAction struct {
    Operation string                  `json:"operation"`
    Items     []harnessruntime.TodoItem `json:"items"`
}

type Action struct {
    ...
    Todo *TodoAction `json:"todo,omitempty"`
}
```

Do not add patch operations yet.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent -run 'Test.*TodoAction.*|Test.*Parse.*Todo.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/model internal/agent
git commit -m "feat: add todo action payload to model actions"
```

### Task 4: Teach the executor to handle `todo` action

**Files:**
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/dispatch.go` if shared action validation lives there
- Test: `internal/agent/loop_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write failing executor tests**

Add tests for:
- `plan_mode=none` + `todo action` -> executor returns a clear error
- `plan_mode=todo` + valid `todo set` -> state is updated and loop continues
- `plan_mode=todo` + `set []` -> todos are cleared
- todo action increments turn count

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent -run 'Test.*Todo.*' -v`

Expected: FAIL because the loop does not recognize `todo`.

- [ ] **Step 3: Implement the todo-action branch**

Add a dedicated path in `ExecuteRun(...)` / supporting helpers:

```text
if action.Action == "todo":
  validate mode == todo
  validate action.Todo != nil
  validate operation == set
  replace exec.state.Todos
  save state
  append todo.updated event
  increment turn count
  continue to next model turn
```

Validation rules:
- `todo` only allowed when `run.PlanMode == PlanModeTodo`
- operation must be `set`
- items may be empty

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent -run 'Test.*Todo.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent internal/service
git commit -m "feat: handle todo action in executor loop"
```

### Task 5: Inject todo context into prompts

**Files:**
- Modify: `internal/prompt/...`
- Modify: prompt builder tests
- Test: `internal/service/run_test.go` or prompt tests

- [ ] **Step 1: Write the failing prompt test**

Add a test that builds a prompt for:
- `PlanModeTodo`
- a `RunState` with at least two todo items

Assert the rendered prompt contains:
- the current todo snapshot
- brief instructions to use full-list replacement

Also add a test that `PlanModeNone` does not include todo instructions.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/prompt ./internal/service -run 'Test.*Todo.*Prompt.*|Test.*PlanMode.*Prompt.*' -v`

Expected: FAIL because todo context is not injected yet.

- [ ] **Step 3: Implement minimal todo prompt injection**

Add prompt context only when `run.PlanMode == PlanModeTodo`:
- current todo items
- short rules:
  - use `todo` action for complex tasks when helpful
  - submit the complete latest list
  - `set []` clears the list
  - do not overuse todo for trivial tasks

Do not inject todo change history.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/prompt ./internal/service -run 'Test.*Todo.*Prompt.*|Test.*PlanMode.*Prompt.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/prompt internal/service
git commit -m "feat: inject todo mode context into prompts"
```

### Task 6: Persist and replay `todo.updated` events

**Files:**
- Modify: `internal/service/run.go`
- Modify: `internal/agent/...` event append helpers as needed
- Test: `internal/service/run_test.go`
- Test: `internal/interfaces/http/...` if stream payloads need assertions

- [ ] **Step 1: Write the failing event test**

Add a test that runs a scripted model producing a todo update and then asserts replay contains:
- one `todo.updated` event
- `payload.operation == "set"`
- `payload.count == len(items)`

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'Test.*TodoUpdated.*' -v`

Expected: FAIL because no such event is emitted yet.

- [ ] **Step 3: Emit todo events from executor**

When todo state is updated, append:

```go
newEvent(..., "todo.updated", "planner", map[string]any{
    "operation": "set",
    "count": len(items),
    "items": items,
})
```

Reuse existing event append path so replay/SSE picks it up automatically.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'Test.*TodoUpdated.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent internal/service
git commit -m "feat: emit todo updated events"
```

### Task 7: Verify resume behavior and compatibility path

**Files:**
- Modify: `internal/service/resume.go`
- Test: `internal/service/resume_test.go`

- [ ] **Step 1: Write the failing resume test**

Add a test that:
- starts a `todo` mode run
- performs one `todo set`
- resumes the run

Assert resume sees the stored todo snapshot and the prompt/input path can still observe it.

Add a second test that a `none` mode run continues to work without todo state.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'TestResume.*Todo.*|TestResume.*PlanModeNone.*' -v`

Expected: FAIL because resume does not yet cover the new mode/state path.

- [ ] **Step 3: Implement minimal resume compatibility**

Ensure resume:
- reads `run.PlanMode`
- keeps existing deterministic-planner compatibility path working if still needed
- passes todo-aware state forward without mutation or reset

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/service -run 'TestResume.*Todo.*|TestResume.*PlanModeNone.*' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service
git commit -m "feat: preserve todo state across resume"
```

### Task 8: Full regression and documentation sync

**Files:**
- Modify: `docs/superpowers/specs/2026-03-30-plan-mode-todo-design.md`
- Create or modify: `docs/superpowers/plans/2026-03-30-plan-mode-todo-implementation.md`

- [ ] **Step 1: Run targeted regression**

Run:

```bash
GOCACHE=/tmp/agent-demo-gocache go test ./internal/runtime ./internal/model ./internal/prompt ./internal/agent ./internal/service
```

Expected: PASS.

- [ ] **Step 2: Run full regression**

Run:

```bash
GOCACHE=/tmp/agent-demo-gocache go test ./...
```

Expected: PASS.

- [ ] **Step 3: Sync docs to actual landed behavior**

Update the spec and this plan/status doc so they reflect:
- actual field names
- actual error behavior
- whether deterministic planner compatibility remains temporarily in place

Do not leave future-tense placeholders once implementation is done.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-03-30-plan-mode-todo-design.md docs/superpowers/plans/2026-03-30-plan-mode-todo-implementation.md
git commit -m "docs: sync todo plan mode design and implementation notes"
```

## Self-Review

Spec coverage:
- `none | todo` modes: Tasks 1-2
- `PlanMode` on `Run`: Tasks 1-2
- `Todos` on `RunState`: Task 1
- dedicated `todo` action with `set`: Task 3
- executor todo path and turn counting: Task 4
- todo prompt injection: Task 5
- `todo.updated` event: Task 6
- resume/replay compatibility: Tasks 6-7

Placeholder scan:
- No `TBD` / `TODO` placeholders remain
- Each task names exact files, commands, and expected outcomes

Type consistency:
- `PlanMode`, `TodoItem`, `TodoAction`, `todo.updated` event naming is consistent across tasks
