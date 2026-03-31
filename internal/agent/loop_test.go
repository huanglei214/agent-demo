package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	harnesscontext "github.com/huanglei214/agent-demo/internal/context"
	"github.com/huanglei214/agent-demo/internal/delegation"
	"github.com/huanglei214/agent-demo/internal/memory"
	"github.com/huanglei214/agent-demo/internal/model"
	"github.com/huanglei214/agent-demo/internal/planner"
	"github.com/huanglei214/agent-demo/internal/prompt"
	"github.com/huanglei214/agent-demo/internal/retrieval"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/runtime/policy"
	"github.com/huanglei214/agent-demo/internal/skill"
	"github.com/huanglei214/agent-demo/internal/store"
	filesystemstore "github.com/huanglei214/agent-demo/internal/store/filesystem"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

func TestParseActionPreservesTodoPayload(t *testing.T) {
	t.Parallel()

	action := parseAction(`{
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
	}`)

	if action.Action != "todo" {
		t.Fatalf("expected todo action, got %#v", action)
	}
	if action.Todo == nil {
		t.Fatalf("expected todo payload, got %#v", action)
	}
	if action.Todo.Operation != "set" {
		t.Fatalf("expected set operation, got %#v", action.Todo)
	}
	if len(action.Todo.Items) != 1 {
		t.Fatalf("expected one todo item, got %#v", action.Todo.Items)
	}
}

func TestValidateActionForRoleAllowsTodoForLeadAgent(t *testing.T) {
	t.Parallel()

	if err := validateActionForRole(harnessruntime.RunRoleLead, model.Action{
		Action: "todo",
		Todo: &model.TodoAction{
			Operation: "set",
			Items:     []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending}},
		},
	}); err != nil {
		t.Fatalf("expected lead-agent todo action to pass role validation, got %v", err)
	}
}

func TestExecuteRunCapturesAssistantMessageWithoutAnswerStreamEvents(t *testing.T) {
	t.Parallel()

	response, observer, services := executeTodoScenarioWithObserver(t, harnessruntime.PlanModeTodo, []model.Action{{Action: "final", Answer: "done"}})

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if got := countEventTypeLoop(observer.runtimeEvents, "assistant.message"); got != 1 {
		t.Fatalf("expected exactly one assistant.message event, got %d in %#v", got, eventTypesLoop(observer.runtimeEvents))
	}
	if len(observer.answerStreamEvents) != 0 {
		t.Fatalf("expected no answer stream events at this stage, got %#v", observer.answerStreamEvents)
	}
	state, err := services.StateStore.LoadState(response.Run.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.TurnCount != 1 {
		t.Fatalf("expected one turn recorded, got %#v", state)
	}
}

func TestExecuteRunStreamedFinalAnswerPersistsOnlyFinalAssistantMessage(t *testing.T) {
	t.Parallel()

	response, observer, services := executeStreamingFinalAnswerScenario(t, []string{"mock response: Hello", ", ", "world"})

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if got := countEventTypeLoop(observer.runtimeEvents, "assistant.message"); got != 1 {
		t.Fatalf("expected exactly one assistant.message event, got %d in %#v", got, eventTypesLoop(observer.runtimeEvents))
	}
	if got := len(observer.answerStreamEvents); got != 5 {
		t.Fatalf("expected start, 3 deltas, and completed answer stream events, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[0].Type != AnswerStreamEventStart {
		t.Fatalf("expected first stream event to be start, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[1].Type != AnswerStreamEventDelta || observer.answerStreamEvents[1].Delta != "mock response: Hello" {
		t.Fatalf("expected first delta event to match first chunk, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[2].Type != AnswerStreamEventDelta || observer.answerStreamEvents[2].Delta != ", " {
		t.Fatalf("expected second delta event to match second chunk, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[3].Type != AnswerStreamEventDelta || observer.answerStreamEvents[3].Delta != "world" {
		t.Fatalf("expected third delta event to match third chunk, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[4].Type != AnswerStreamEventCompleted {
		t.Fatalf("expected final stream event to be completed, got %#v", observer.answerStreamEvents)
	}

	events, err := services.EventStore.ReadAll(response.Run.ID)
	if err != nil {
		t.Fatalf("load persisted events: %v", err)
	}
	if got := countEventTypeLoop(events, "assistant.message"); got != 1 {
		t.Fatalf("expected one persisted assistant.message, got %d in %#v", got, eventTypesLoop(events))
	}
	if got := countEventTypeLoop(events, "answer.delta"); got != 0 {
		t.Fatalf("expected no persisted answer.delta events, got %d in %#v", got, eventTypesLoop(events))
	}
	if len(events) == 0 {
		t.Fatal("expected persisted runtime events")
	}

	state, err := services.StateStore.LoadState(response.Run.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	messages, err := services.StateStore.LoadSessionMessages(response.Run.SessionID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected exactly one persisted assistant message, got %#v", messages)
	}
	if got := messages[0].Content; got != "mock response: Hello, world" {
		t.Fatalf("expected persisted assistant message to equal concatenated deltas, got %q", got)
	}
	if state.TurnCount != 1 {
		t.Fatalf("expected one turn recorded, got %#v", state)
	}
}

func TestExecuteRunDoesNotStreamAnswerEventsForNonFinalAction(t *testing.T) {
	t.Parallel()

	response, observer, services := executeStreamingActionScenario(t, harnessruntime.PlanModeTodo, []model.Action{
		{
			Action: "todo",
			Todo: &model.TodoAction{
				Operation: "set",
				Items: []harnessruntime.TodoItem{{
					ID: "todo_1",
					Content: "Read README",
					Status: harnessruntime.TodoPending,
				}},
			},
		},
		{Action: "final", Answer: "mock response: Hello, world"},
	})

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if got := countEventTypeLoop(observer.runtimeEvents, "assistant.message"); got != 1 {
		t.Fatalf("expected exactly one assistant.message event, got %d in %#v", got, eventTypesLoop(observer.runtimeEvents))
	}
	if got := len(observer.answerStreamEvents); got != 3 {
		t.Fatalf("expected only the final answer to stream, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[0].Type != AnswerStreamEventStart {
		t.Fatalf("expected first stream event to be start, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[2].Type != AnswerStreamEventCompleted {
		t.Fatalf("expected final stream event to be completed, got %#v", observer.answerStreamEvents)
	}

	events, err := services.EventStore.ReadAll(response.Run.ID)
	if err != nil {
		t.Fatalf("load persisted events: %v", err)
	}
	if got := countEventTypeLoop(events, "assistant.message"); got != 1 {
		t.Fatalf("expected one persisted assistant.message, got %d in %#v", got, eventTypesLoop(events))
	}
	if got := countEventTypeLoop(events, "answer.delta"); got != 0 {
		t.Fatalf("expected no persisted answer.delta events, got %d in %#v", got, eventTypesLoop(events))
	}
	messages, err := services.StateStore.LoadSessionMessages(response.Run.SessionID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected exactly one persisted assistant message, got %#v", messages)
	}
	if got := messages[0].Content; got != "mock response: Hello, world" {
		t.Fatalf("expected persisted assistant message to equal concatenated deltas, got %q", got)
	}
}

func TestExecuteRunStreamFailureEmitsFailedAnswerStreamEvent(t *testing.T) {
	t.Parallel()

	response, observer, services := executeStreamingFailureScenario(t)
	_ = response

	if got := len(observer.answerStreamEvents); got != 3 {
		t.Fatalf("expected start, delta, and failed answer stream events, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[0].Type != AnswerStreamEventStart {
		t.Fatalf("expected first stream event to be start, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[1].Type != AnswerStreamEventDelta || observer.answerStreamEvents[1].Delta != "partial answer" {
		t.Fatalf("expected second stream event to be delta, got %#v", observer.answerStreamEvents)
	}
	if observer.answerStreamEvents[2].Type != AnswerStreamEventFailed {
		t.Fatalf("expected final stream event to be failed, got %#v", observer.answerStreamEvents)
	}
	if got := countAnswerStreamEventType(observer.answerStreamEvents, AnswerStreamEventFailed); got != 1 {
		t.Fatalf("expected exactly one failed answer stream event, got %#v", observer.answerStreamEvents)
	}
	if got := countEventTypeLoop(observer.runtimeEvents, "assistant.message"); got != 0 {
		t.Fatalf("expected no assistant.message event on stream failure, got %d in %#v", got, eventTypesLoop(observer.runtimeEvents))
	}

	run, err := services.LoadRun("run_stream_failure")
	if err != nil {
		t.Fatalf("load failed run: %v", err)
	}
	if run.Status != harnessruntime.RunFailed {
		t.Fatalf("expected failed run, got %#v", run)
	}
}

type captureRunAndStreamObserver struct {
	runtimeEvents      []harnessruntime.Event
	answerStreamEvents []AnswerStreamEvent
}

func (o *captureRunAndStreamObserver) OnRuntimeEvent(event harnessruntime.Event) {
	o.runtimeEvents = append(o.runtimeEvents, event)
}

func (o *captureRunAndStreamObserver) OnAnswerStreamEvent(event AnswerStreamEvent) {
	o.answerStreamEvents = append(o.answerStreamEvents, event)
}

type captureAnswerStreamObserver struct {
	events []AnswerStreamEvent
}

func (o *captureAnswerStreamObserver) OnRuntimeEvent(harnessruntime.Event) {}

func (o *captureAnswerStreamObserver) OnAnswerStreamEvent(event AnswerStreamEvent) {
	o.events = append(o.events, event)
}

func TestExecuteRunRejectsTodoActionWhenPlanModeIsNone(t *testing.T) {
	t.Parallel()

	response, events, services := executeTodoScenario(t, harnessruntime.PlanModeNone, []model.Action{{
		Action: "todo",
		Todo: &model.TodoAction{
			Operation: "set",
			Items:     []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending}},
		},
	}})

	if response.Result != nil {
		t.Fatalf("expected failed run without final result, got %#v", response)
	}
	if got, err := services.LoadRun("run_todo"); err != nil {
		t.Fatalf("load failed run: %v", err)
	} else if got.Status != harnessruntime.RunFailed {
		t.Fatalf("expected failed run, got %#v", got)
	}
	assertEventPresentLoop(t, events, "run.failed")
	assertEventAbsentLoop(t, events, "todo.updated")
	state, err := services.StateStore.LoadState("run_todo")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Todos) != 0 {
		t.Fatalf("expected no persisted todos, got %#v", state.Todos)
	}
}

func TestExecuteRunAppliesTodoSetAndContinues(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	response, events, services := executeTodoScenario(t, harnessruntime.PlanModeTodo, []model.Action{
		{
			Action: "todo",
			Todo: &model.TodoAction{
				Operation: "set",
				Items: []harnessruntime.TodoItem{{
					ID:        "todo_1",
					Content:   "Read README",
					Status:    harnessruntime.TodoPending,
					Priority:  1,
					UpdatedAt: now,
				}},
			},
		},
		{Action: "final", Answer: "done"},
	})

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if response.Run.TurnCount != 2 {
		t.Fatalf("expected todo action to increment turn count, got %#v", response.Run)
	}
	state, err := services.StateStore.LoadState(response.Run.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.TurnCount != 2 {
		t.Fatalf("expected persisted turn count 2, got %#v", state)
	}
	if len(state.Todos) != 1 {
		t.Fatalf("expected one persisted todo, got %#v", state.Todos)
	}
	if state.Todos[0].Content != "Read README" {
		t.Fatalf("unexpected todo content: %#v", state.Todos[0])
	}
	assertEventPresentLoop(t, events, "todo.updated")
}

func TestExecuteRunClearsTodosOnEmptySet(t *testing.T) {
	t.Parallel()

	response, events, services := executeTodoScenario(t, harnessruntime.PlanModeTodo, []model.Action{
		{
			Action: "todo",
			Todo: &model.TodoAction{
				Operation: "set",
				Items:     []harnessruntime.TodoItem{{ID: "todo_1", Content: "Read README", Status: harnessruntime.TodoPending}},
			},
		},
		{
			Action: "todo",
			Todo: &model.TodoAction{
				Operation: "set",
				Items:     []harnessruntime.TodoItem{},
			},
		},
		{Action: "final", Answer: "done"},
	})

	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed run, got %#v", response.Run)
	}
	if response.Run.TurnCount != 3 {
		t.Fatalf("expected todo actions plus final to count as turns, got %#v", response.Run)
	}
	state, err := services.StateStore.LoadState(response.Run.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Todos) != 0 {
		t.Fatalf("expected todos to be cleared, got %#v", state.Todos)
	}
	if got := countEventTypeLoop(events, "todo.updated"); got != 2 {
		t.Fatalf("expected two todo.updated events, got %d in %#v", got, eventTypesLoop(events))
	}
}

func executeTodoScenario(t *testing.T, planMode harnessruntime.PlanMode, actions []model.Action) (ExecutionResponse, []harnessruntime.Event, testTodoServices) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	stateStore := filesystemstore.NewStateStore(paths)
	eventStore := filesystemstore.NewEventStore(paths)
	registry := toolruntime.NewRegistry()
	executor := NewExecutor(
		cfg,
		RuntimeServices{Paths: paths, EventStore: eventStore, StateStore: stateStore},
		ModelServices{ModelFactory: func() (model.Model, error) { return &todoActionSequenceModel{actions: actions}, nil }, PromptBuilder: prompt.NewBuilder()},
		AgentServices{Planner: planner.New(), ContextManager: harnesscontext.NewManager(), MemoryManager: memory.NewManager(paths)},
		ToolServices{ToolRegistry: registry, ToolExecutor: toolruntime.NewExecutor(registry)},
		DelegationServices{DelegationManager: delegation.NewManager(paths), SkillRegistry: skill.NewRegistry(workspace)},
	)

	now := time.Now().UTC()
	task := harnessruntime.Task{ID: "task_todo", Instruction: "Inspect the repository", Workspace: workspace, CreatedAt: now}
	session := harnessruntime.Session{ID: "session_todo", Workspace: workspace, CreatedAt: now, UpdatedAt: now}
	run := harnessruntime.Run{ID: "run_todo", TaskID: task.ID, SessionID: session.ID, Role: harnessruntime.RunRoleLead, PlanMode: planMode, Status: harnessruntime.RunPending, Provider: "mock", Model: "mock-model", MaxTurns: len(actions) + 1, CreatedAt: now, UpdatedAt: now}
	plan := harnessruntime.Plan{ID: "plan_todo", RunID: run.ID, Goal: task.Instruction, Version: 1, CreatedAt: now, UpdatedAt: now, Steps: []harnessruntime.PlanStep{{ID: "step_todo", Title: "Inspect repository", Description: task.Instruction, Status: harnessruntime.StepPending}}}
	state := harnessruntime.RunState{RunID: run.ID, UpdatedAt: now}

	for _, persist := range []func() error{
		func() error { return stateStore.SaveTask(task) },
		func() error { return stateStore.SaveSession(session) },
		func() error { return stateStore.SaveRun(run) },
		func() error { return stateStore.SavePlan(plan) },
		func() error { return stateStore.SaveState(state) },
	} {
		if err := persist(); err != nil {
			t.Fatalf("persist scenario: %v", err)
		}
	}

	response, err := executor.ExecuteRun(context.Background(), task, session, run, plan, state, true, nil)
	if planMode == harnessruntime.PlanModeNone {
		if err == nil {
			t.Fatal("expected plan_mode=none todo scenario to fail")
		}
		if !strings.Contains(err.Error(), "todo") {
			t.Fatalf("expected todo-related failure, got %v", err)
		}
	} else if err != nil {
		t.Fatalf("execute todo scenario: %v", err)
	}

	events, err := eventStore.ReadAll(run.ID)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	return response, events, testTodoServices{StateStore: stateStore}
}

func executeTodoScenarioWithObserver(t *testing.T, planMode harnessruntime.PlanMode, actions []model.Action) (ExecutionResponse, *captureRunAndStreamObserver, testTodoServices) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	stateStore := filesystemstore.NewStateStore(paths)
	eventStore := filesystemstore.NewEventStore(paths)
	registry := toolruntime.NewRegistry()
	executor := NewExecutor(
		cfg,
		RuntimeServices{Paths: paths, EventStore: eventStore, StateStore: stateStore},
		ModelServices{ModelFactory: func() (model.Model, error) { return &todoActionSequenceModel{actions: actions}, nil }, PromptBuilder: prompt.NewBuilder()},
		AgentServices{Planner: planner.New(), ContextManager: harnesscontext.NewManager(), MemoryManager: memory.NewManager(paths)},
		ToolServices{ToolRegistry: registry, ToolExecutor: toolruntime.NewExecutor(registry)},
		DelegationServices{DelegationManager: delegation.NewManager(paths), SkillRegistry: skill.NewRegistry(workspace)},
	)

	now := time.Now().UTC()
	task := harnessruntime.Task{ID: "task_todo", Instruction: "Inspect the repository", Workspace: workspace, CreatedAt: now}
	session := harnessruntime.Session{ID: "session_todo", Workspace: workspace, CreatedAt: now, UpdatedAt: now}
	run := harnessruntime.Run{ID: "run_todo", TaskID: task.ID, SessionID: session.ID, Role: harnessruntime.RunRoleLead, PlanMode: planMode, Status: harnessruntime.RunPending, Provider: "mock", Model: "mock-model", MaxTurns: len(actions) + 1, CreatedAt: now, UpdatedAt: now}
	plan := harnessruntime.Plan{ID: "plan_todo", RunID: run.ID, Goal: task.Instruction, Version: 1, CreatedAt: now, UpdatedAt: now, Steps: []harnessruntime.PlanStep{{ID: "step_todo", Title: "Inspect repository", Description: task.Instruction, Status: harnessruntime.StepPending}}}
	state := harnessruntime.RunState{RunID: run.ID, UpdatedAt: now}

	for _, persist := range []func() error{
		func() error { return stateStore.SaveTask(task) },
		func() error { return stateStore.SaveSession(session) },
		func() error { return stateStore.SaveRun(run) },
		func() error { return stateStore.SavePlan(plan) },
		func() error { return stateStore.SaveState(state) },
	} {
		if err := persist(); err != nil {
			t.Fatalf("persist scenario: %v", err)
		}
	}

	observer := &captureRunAndStreamObserver{}
	response, err := executor.ExecuteRun(context.Background(), task, session, run, plan, state, true, observer)
	if planMode == harnessruntime.PlanModeNone {
		if err == nil {
			t.Fatal("expected plan_mode=none todo scenario to fail")
		}
		if !strings.Contains(err.Error(), "todo") {
			t.Fatalf("expected todo-related failure, got %v", err)
		}
	} else if err != nil {
		t.Fatalf("execute todo scenario: %v", err)
	}

	events, err := eventStore.ReadAll(run.ID)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if got := countEventTypeLoop(events, "assistant.message"); got != 1 {
		t.Fatalf("expected exactly one persisted assistant.message, got %d in %#v", got, eventTypesLoop(events))
	}

	return response, observer, testTodoServices{StateStore: stateStore, EventStore: eventStore}
}

func executeStreamingFinalAnswerScenario(t *testing.T, chunks []string) (ExecutionResponse, *captureRunAndStreamObserver, testTodoServices) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	stateStore := filesystemstore.NewStateStore(paths)
	eventStore := filesystemstore.NewEventStore(paths)
	registry := toolruntime.NewRegistry()
	executor := NewExecutor(
		cfg,
		RuntimeServices{Paths: paths, EventStore: eventStore, StateStore: stateStore},
		ModelServices{ModelFactory: func() (model.Model, error) { return &streamingFinalAnswerModel{chunks: chunks}, nil }, PromptBuilder: prompt.NewBuilder()},
		AgentServices{Planner: planner.New(), ContextManager: harnesscontext.NewManager(), MemoryManager: memory.NewManager(paths)},
		ToolServices{ToolRegistry: registry, ToolExecutor: toolruntime.NewExecutor(registry)},
		DelegationServices{DelegationManager: delegation.NewManager(paths), SkillRegistry: skill.NewRegistry(workspace)},
	)

	now := time.Now().UTC()
	task := harnessruntime.Task{ID: "task_stream", Instruction: "Summarize the repository in one sentence", Workspace: workspace, CreatedAt: now}
	session := harnessruntime.Session{ID: "session_stream", Workspace: workspace, CreatedAt: now, UpdatedAt: now}
	run := harnessruntime.Run{ID: "run_stream", TaskID: task.ID, SessionID: session.ID, Role: harnessruntime.RunRoleLead, PlanMode: harnessruntime.PlanModeNone, Status: harnessruntime.RunPending, Provider: "mock", Model: "mock-model", MaxTurns: 2, CreatedAt: now, UpdatedAt: now}
	plan := harnessruntime.Plan{ID: "plan_stream", RunID: run.ID, Goal: task.Instruction, Version: 1, CreatedAt: now, UpdatedAt: now, Steps: []harnessruntime.PlanStep{{ID: "step_stream", Title: "Summarize the repository", Description: task.Instruction, Status: harnessruntime.StepPending}}}
	state := harnessruntime.RunState{RunID: run.ID, UpdatedAt: now}

	for _, persist := range []func() error{
		func() error { return stateStore.SaveTask(task) },
		func() error { return stateStore.SaveSession(session) },
		func() error { return stateStore.SaveRun(run) },
		func() error { return stateStore.SavePlan(plan) },
		func() error { return stateStore.SaveState(state) },
	} {
		if err := persist(); err != nil {
			t.Fatalf("persist scenario: %v", err)
		}
	}

	observer := &captureRunAndStreamObserver{}
	response, err := executor.ExecuteRun(context.Background(), task, session, run, plan, state, true, observer)
	if err != nil {
		t.Fatalf("execute streaming final answer scenario: %v", err)
	}

	return response, observer, testTodoServices{StateStore: stateStore, EventStore: eventStore}
}

func executeStreamingActionScenario(t *testing.T, planMode harnessruntime.PlanMode, actions []model.Action) (ExecutionResponse, *captureRunAndStreamObserver, testTodoServices) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	stateStore := filesystemstore.NewStateStore(paths)
	eventStore := filesystemstore.NewEventStore(paths)
	registry := toolruntime.NewRegistry()
	executor := NewExecutor(
		cfg,
		RuntimeServices{Paths: paths, EventStore: eventStore, StateStore: stateStore},
		ModelServices{ModelFactory: func() (model.Model, error) { return &streamingActionSequenceModel{actions: actions}, nil }, PromptBuilder: prompt.NewBuilder()},
		AgentServices{Planner: planner.New(), ContextManager: harnesscontext.NewManager(), MemoryManager: memory.NewManager(paths)},
		ToolServices{ToolRegistry: registry, ToolExecutor: toolruntime.NewExecutor(registry)},
		DelegationServices{DelegationManager: delegation.NewManager(paths), SkillRegistry: skill.NewRegistry(workspace)},
	)

	now := time.Now().UTC()
	task := harnessruntime.Task{ID: "task_stream_action", Instruction: "Inspect the repository", Workspace: workspace, CreatedAt: now}
	session := harnessruntime.Session{ID: "session_stream_action", Workspace: workspace, CreatedAt: now, UpdatedAt: now}
	run := harnessruntime.Run{ID: "run_stream_action", TaskID: task.ID, SessionID: session.ID, Role: harnessruntime.RunRoleLead, PlanMode: planMode, Status: harnessruntime.RunPending, Provider: "mock", Model: "mock-model", MaxTurns: len(actions) + 1, CreatedAt: now, UpdatedAt: now}
	plan := harnessruntime.Plan{ID: "plan_stream_action", RunID: run.ID, Goal: task.Instruction, Version: 1, CreatedAt: now, UpdatedAt: now, Steps: []harnessruntime.PlanStep{{ID: "step_stream_action", Title: "Inspect the repository", Description: task.Instruction, Status: harnessruntime.StepPending}}}
	state := harnessruntime.RunState{RunID: run.ID, UpdatedAt: now}

	for _, persist := range []func() error{
		func() error { return stateStore.SaveTask(task) },
		func() error { return stateStore.SaveSession(session) },
		func() error { return stateStore.SaveRun(run) },
		func() error { return stateStore.SavePlan(plan) },
		func() error { return stateStore.SaveState(state) },
	} {
		if err := persist(); err != nil {
			t.Fatalf("persist scenario: %v", err)
		}
	}

	observer := &captureRunAndStreamObserver{}
	response, err := executor.ExecuteRun(context.Background(), task, session, run, plan, state, true, observer)
	if err != nil {
		t.Fatalf("execute streaming action scenario: %v", err)
	}

	return response, observer, testTodoServices{StateStore: stateStore, EventStore: eventStore}
}

func executeStreamingFailureScenario(t *testing.T) (ExecutionResponse, *captureRunAndStreamObserver, testTodoServices) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	paths := store.NewPaths(cfg.Runtime.Root)
	stateStore := filesystemstore.NewStateStore(paths)
	eventStore := filesystemstore.NewEventStore(paths)
	registry := toolruntime.NewRegistry()
	executor := NewExecutor(
		cfg,
		RuntimeServices{Paths: paths, EventStore: eventStore, StateStore: stateStore},
		ModelServices{ModelFactory: func() (model.Model, error) { return &streamingFailureModel{}, nil }, PromptBuilder: prompt.NewBuilder()},
		AgentServices{Planner: planner.New(), ContextManager: harnesscontext.NewManager(), MemoryManager: memory.NewManager(paths)},
		ToolServices{ToolRegistry: registry, ToolExecutor: toolruntime.NewExecutor(registry)},
		DelegationServices{DelegationManager: delegation.NewManager(paths), SkillRegistry: skill.NewRegistry(workspace)},
	)

	now := time.Now().UTC()
	task := harnessruntime.Task{ID: "task_stream_failure", Instruction: "Summarize the repository", Workspace: workspace, CreatedAt: now}
	session := harnessruntime.Session{ID: "session_stream_failure", Workspace: workspace, CreatedAt: now, UpdatedAt: now}
	run := harnessruntime.Run{ID: "run_stream_failure", TaskID: task.ID, SessionID: session.ID, Role: harnessruntime.RunRoleLead, PlanMode: harnessruntime.PlanModeNone, Status: harnessruntime.RunPending, Provider: "mock", Model: "mock-model", MaxTurns: 1, CreatedAt: now, UpdatedAt: now}
	plan := harnessruntime.Plan{ID: "plan_stream_failure", RunID: run.ID, Goal: task.Instruction, Version: 1, CreatedAt: now, UpdatedAt: now, Steps: []harnessruntime.PlanStep{{ID: "step_stream_failure", Title: "Summarize the repository", Description: task.Instruction, Status: harnessruntime.StepPending}}}
	state := harnessruntime.RunState{RunID: run.ID, UpdatedAt: now}

	for _, persist := range []func() error{
		func() error { return stateStore.SaveTask(task) },
		func() error { return stateStore.SaveSession(session) },
		func() error { return stateStore.SaveRun(run) },
		func() error { return stateStore.SavePlan(plan) },
		func() error { return stateStore.SaveState(state) },
	} {
		if err := persist(); err != nil {
			t.Fatalf("persist scenario: %v", err)
		}
	}

	observer := &captureRunAndStreamObserver{}
	response, err := executor.ExecuteRun(context.Background(), task, session, run, plan, state, true, observer)
	if err == nil {
		t.Fatal("expected streaming failure scenario to fail")
	}
	return response, observer, testTodoServices{StateStore: stateStore, EventStore: eventStore}
}

type streamingFinalAnswerModel struct {
	chunks []string
}

func (m *streamingFinalAnswerModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	return model.Response{Text: `{"action":"final","answer":"mock response: Hello, world"}`, FinishReason: "stop"}, nil
}

func (m *streamingFinalAnswerModel) GenerateStream(ctx context.Context, req model.Request, sink model.StreamSink) error {
	_ = ctx
	_ = req
	if sink == nil {
		return nil
	}
	if err := sink.Start(); err != nil {
		return err
	}
	for _, chunk := range m.chunks {
		if err := sink.Delta(chunk); err != nil {
			return err
		}
	}
	return sink.Complete()
}

type testTodoServices struct {
	StateStore filesystemstore.StateStore
	EventStore filesystemstore.EventStore
}

func (s testTodoServices) LoadRun(runID string) (harnessruntime.Run, error) {
	return s.StateStore.LoadRun(runID)
}

func assertEventPresentLoop(t *testing.T, events []harnessruntime.Event, eventType string) {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			return
		}
	}
	t.Fatalf("expected event %q in %#v", eventType, events)
}

func assertEventAbsentLoop(t *testing.T, events []harnessruntime.Event, eventType string) {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			t.Fatalf("did not expect event %q in %#v", eventType, events)
		}
	}
}

func countEventTypeLoop(events []harnessruntime.Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func countAnswerStreamEventType(events []AnswerStreamEvent, eventType AnswerStreamEventType) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func eventTypesLoop(events []harnessruntime.Event) []string {
	result := make([]string, 0, len(events))
	for _, event := range events {
		result = append(result, event.Type)
	}
	return result
}

type todoActionSequenceModel struct {
	actions []model.Action
	index   int
}

type streamingActionSequenceModel struct {
	actions []model.Action
	index   int
}

type streamingFailureModel struct{}

func (m *todoActionSequenceModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	if m.index >= len(m.actions) {
		return model.Response{Text: `{"action":"final","answer":"done"}`, FinishReason: "stop"}, nil
	}
	data, err := json.Marshal(m.actions[m.index])
	if err != nil {
		return model.Response{}, err
	}
	m.index++
	return model.Response{Text: string(data), FinishReason: "stop"}, nil
}

func (m *streamingActionSequenceModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	if m.index >= len(m.actions) {
		return model.Response{Text: `{"action":"final","answer":"done"}`, FinishReason: "stop"}, nil
	}
	data, err := json.Marshal(m.actions[m.index])
	if err != nil {
		return model.Response{}, err
	}
	m.index++
	return model.Response{Text: string(data), FinishReason: "stop"}, nil
}

func (m *streamingActionSequenceModel) GenerateStream(ctx context.Context, req model.Request, sink model.StreamSink) error {
	resp, err := m.Generate(ctx, req)
	if err != nil {
		return err
	}
	answer := parseAction(resp.Text)
	if answer.Action != "final" || strings.TrimSpace(answer.Answer) == "" {
		return &model.NonFinalStreamResponseError{Response: resp}
	}
	if sink == nil {
		return nil
	}
	if err := sink.Start(); err != nil {
		return err
	}
	for _, chunk := range []string{answer.Answer} {
		if err := sink.Delta(chunk); err != nil {
			return err
		}
	}
	return sink.Complete()
}

func (m *streamingFailureModel) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx
	_ = req
	return model.Response{Text: `{"action":"final","answer":"should not be used"}`, FinishReason: "stop"}, nil
}

func (m *streamingFailureModel) GenerateStream(ctx context.Context, req model.Request, sink model.StreamSink) error {
	_ = ctx
	_ = req
	if sink == nil {
		return nil
	}
	if err := sink.Start(); err != nil {
		return err
	}
	if err := sink.Delta("partial answer"); err != nil {
		return err
	}
	return errors.New("stream transport failed")
}

func TestPolicyContextReturnsIsolatedTaskAndStateSnapshots(t *testing.T) {
	exec := &runExecution{
		task: harnessruntime.Task{
			ID:          "task_1",
			Instruction: "inspect",
			Workspace:   "/tmp/workspace",
			Metadata: map[string]string{
				"skill": "reader",
			},
		},
		session: harnessruntime.Session{ID: "session_1", Workspace: "/tmp/workspace"},
		run:     harnessruntime.Run{ID: "run_1"},
		state: harnessruntime.RunState{
			RunID:           "run_1",
			PendingToolName: "test.inspect",
			PendingToolResult: map[string]any{
				"nested": map[string]any{"value": "ok"},
			},
			PendingToolResults: []harnessruntime.ToolCallResult{{
				Tool:   "test.inspect",
				Input:  map[string]any{"path": "notes.txt"},
				Result: map[string]any{"nested": map[string]any{"value": "ok"}},
			}},
		},
		plan: harnessruntime.Plan{
			ID:   "plan_1",
			Goal: "inspect",
			Steps: []harnessruntime.PlanStep{{
				ID:          "step_1",
				Title:       "read",
				Description: "read file",
			}},
		},
		currentStep: &harnessruntime.PlanStep{ID: "step_1", Title: "read", Description: "read file"},
		workingEvidence: map[string]any{
			"nested": map[string]any{"value": "ok"},
		},
		explicitMemoryCandidates: []harnessruntime.MemoryCandidate{{Content: "candidate"}},
		recalledMemories:         []harnessruntime.MemoryEntry{{Content: "memory"}},
		summaries:                []harnessruntime.Summary{{Content: "summary"}},
		retrievalProgress: retrieval.RetrievalProgress{
			SearchQueries: []string{"q1"},
		},
		mode: policy.ExecutionModeStructured,
	}

	executor := &Executor{}
	ctx := executor.policyContext(exec)

	ctx.Task.Metadata["skill"] = "writer"
	ctx.State.PendingToolResult["nested"].(map[string]any)["value"] = "tampered"
	ctx.State.PendingToolResults[0].Input["path"] = "tampered.txt"
	ctx.State.PendingToolResults[0].Result["nested"].(map[string]any)["value"] = "tampered"

	if got := exec.task.Metadata["skill"]; got != "reader" {
		t.Fatalf("expected task metadata to stay isolated, got %q", got)
	}
	if got := exec.state.PendingToolResult["nested"].(map[string]any)["value"]; got != "ok" {
		t.Fatalf("expected pending tool result to stay isolated, got %#v", got)
	}
	if got := exec.state.PendingToolResults[0].Input["path"]; got != "notes.txt" {
		t.Fatalf("expected pending tool results input to stay isolated, got %#v", got)
	}
	if got := exec.state.PendingToolResults[0].Result["nested"].(map[string]any)["value"]; got != "ok" {
		t.Fatalf("expected pending tool results result to stay isolated, got %#v", got)
	}
}

func TestRunPoliciesAfterActionRejectsNoopMutation(t *testing.T) {
	exec := &runExecution{}
	realAction := model.Action{
		Action: "tool",
		Calls: []model.ToolCall{{
			Tool:  "test.inspect",
			Input: map[string]any{"path": "notes.txt"},
		}},
	}
	realResult := policy.ActionResult{
		Kind:    policy.ActionResultToolBatch,
		Success: true,
		ToolCalls: []model.ToolCall{{
			Tool:  "test.inspect",
			Input: map[string]any{"path": "notes.txt"},
		}},
		ToolResults: []harnessruntime.ToolCallResult{{
			Tool:   "test.inspect",
			Input:  map[string]any{"path": "notes.txt"},
			Result: map[string]any{"nested": map[string]any{"value": "ok"}},
		}},
	}

	executor := &Executor{
		Policies: []policy.RuntimePolicy{afterActionMutatingPolicy{}},
	}
	_, err := executor.runPoliciesAfterAction(t.Context(), exec, realAction, realResult, nil, nil)
	if err == nil {
		t.Fatal("expected mutation error")
	}
	if got := realAction.Calls[0].Input["path"]; got != "notes.txt" {
		t.Fatalf("expected original action input to stay unchanged, got %#v", got)
	}
	if got := realResult.ToolResults[0].Result["nested"].(map[string]any)["value"]; got != "ok" {
		t.Fatalf("expected original result to stay unchanged, got %#v", got)
	}
}

func TestRunPoliciesAfterActionReturnsNilForDelegationNoop(t *testing.T) {
	exec := &runExecution{}
	action := model.Action{Action: "delegate"}
	result := policy.ActionResult{
		Kind:    policy.ActionResultDelegation,
		Success: true,
		Delegation: &harnessruntime.DelegationResult{
			ChildRunID:  "run_child",
			NeedsReplan: false,
		},
	}

	executor := &Executor{
		Policies: []policy.RuntimePolicy{policy.ReplanPolicy{}},
	}
	outcome, err := executor.runPoliciesAfterAction(t.Context(), exec, action, result, map[policy.PolicyDecision]struct{}{
		policy.DecisionReplan: {},
	}, nil)
	if err != nil {
		t.Fatalf("run policies after action: %v", err)
	}
	if outcome != nil {
		t.Fatalf("expected nil outcome for noop delegation result, got %#v", outcome)
	}
}

func TestRunPoliciesAfterActionReturnsAllowedReplanOutcome(t *testing.T) {
	exec := &runExecution{}
	action := model.Action{Action: "delegate"}
	result := policy.ActionResult{
		Kind:    policy.ActionResultDelegation,
		Success: true,
		Delegation: &harnessruntime.DelegationResult{
			ChildRunID:  "run_child",
			Summary:     "Child found missing evidence and requested a replan.",
			NeedsReplan: true,
		},
	}

	executor := &Executor{
		Policies: []policy.RuntimePolicy{policy.ReplanPolicy{}},
	}
	outcome, err := executor.runPoliciesAfterAction(t.Context(), exec, action, result, map[policy.PolicyDecision]struct{}{
		policy.DecisionReplan: {},
	}, nil)
	if err != nil {
		t.Fatalf("run policies after action: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected outcome, got nil")
	}
	if outcome.Decision != policy.DecisionReplan {
		t.Fatalf("expected replan outcome, got %#v", outcome)
	}
}

func TestRunPoliciesAfterActionRejectsDisallowedOutcome(t *testing.T) {
	exec := &runExecution{}
	action := model.Action{Action: "delegate"}
	result := policy.ActionResult{
		Kind:    policy.ActionResultDelegation,
		Success: true,
		Delegation: &harnessruntime.DelegationResult{
			ChildRunID:  "run_child",
			Summary:     "Child found missing evidence and requested a replan.",
			NeedsReplan: true,
		},
	}

	executor := &Executor{
		Policies: []policy.RuntimePolicy{policy.ReplanPolicy{}},
	}
	outcome, err := executor.runPoliciesAfterAction(t.Context(), exec, action, result, nil, nil)
	if err == nil {
		t.Fatal("expected disallowed outcome error")
	}
	if outcome != nil {
		t.Fatalf("expected no outcome when disallowed, got %#v", outcome)
	}
}

func TestRunPoliciesAfterModelReturnsAllowedBlockOutcome(t *testing.T) {
	t.Parallel()

	executor := &Executor{
		Policies: []policy.RuntimePolicy{afterModelBlockingPolicy{}},
	}

	outcome, err := executor.runPoliciesAfterModel(t.Context(), &runExecution{}, &model.Action{Action: "delegate"}, map[policy.PolicyDecision]struct{}{
		policy.DecisionBlock: {},
	}, nil)
	if err != nil {
		t.Fatalf("run policies after model: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected block outcome, got nil")
	}
	if outcome.Decision != policy.DecisionBlock {
		t.Fatalf("expected block outcome, got %#v", outcome)
	}
}

func TestRunPoliciesAfterModelRejectsDisallowedEffectfulOutcome(t *testing.T) {
	t.Parallel()

	executor := &Executor{
		Policies: []policy.RuntimePolicy{afterModelBlockingPolicy{}},
	}

	outcome, err := executor.runPoliciesAfterModel(t.Context(), &runExecution{}, &model.Action{Action: "delegate"}, nil, nil)
	if err == nil {
		t.Fatal("expected disallowed outcome error")
	}
	if outcome != nil {
		t.Fatalf("expected nil outcome, got %#v", outcome)
	}
	if !strings.Contains(err.Error(), "unexpected policy outcome") {
		t.Fatalf("expected unexpected policy outcome error, got %v", err)
	}
}

func TestRunPoliciesAfterModelSkipsExcludedDelegationPolicy(t *testing.T) {
	t.Parallel()

	executor := &Executor{
		Policies: []policy.RuntimePolicy{afterModelBlockingPolicy{}},
	}

	outcome, err := executor.runPoliciesAfterModel(t.Context(), &runExecution{}, &model.Action{Action: "delegate"}, nil, map[policy.PolicyName]struct{}{
		policy.PolicyName("after-model-blocking"): {},
	})
	if err != nil {
		t.Fatalf("run policies after model: %v", err)
	}
	if outcome != nil {
		t.Fatalf("expected nil outcome when delegation policy is excluded, got %#v", outcome)
	}
}

func TestRunPoliciesAfterModelUsesFirstEffectfulOutcome(t *testing.T) {
	t.Parallel()

	first := &countingAfterModelPolicy{
		name: "first-blocker",
		outcome: &policy.PolicyOutcome{
			Decision: policy.DecisionBlock,
			Reason:   "blocked_by_first",
		},
	}
	second := &countingAfterModelPolicy{
		name: "second-replan",
		outcome: &policy.PolicyOutcome{
			Decision: policy.DecisionReplan,
			Reason:   "should_not_run",
		},
	}

	executor := &Executor{
		Policies: []policy.RuntimePolicy{first, second},
	}

	outcome, err := executor.runPoliciesAfterModel(t.Context(), &runExecution{}, &model.Action{Action: "delegate"}, map[policy.PolicyDecision]struct{}{
		policy.DecisionBlock:  {},
		policy.DecisionReplan: {},
	}, nil)
	if err != nil {
		t.Fatalf("run policies after model: %v", err)
	}
	if outcome == nil {
		t.Fatal("expected first effectful outcome, got nil")
	}
	if outcome.Decision != policy.DecisionBlock {
		t.Fatalf("expected first policy block outcome, got %#v", outcome)
	}
	if first.afterModelCalls != 1 {
		t.Fatalf("expected first policy to be observed once, got %d", first.afterModelCalls)
	}
	if second.afterModelCalls != 0 {
		t.Fatalf("expected second policy to be skipped after first effectful outcome, got %d", second.afterModelCalls)
	}
}

func TestValidateActionForRoleAllowsSubagentDelegateShape(t *testing.T) {
	t.Parallel()

	if err := validateActionForRole(harnessruntime.RunRoleSubagent, model.Action{
		Action:         "delegate",
		DelegationGoal: "继续委派给另一个子代理",
	}); err != nil {
		t.Fatalf("expected subagent delegate shape to pass role validation, got %v", err)
	}
}

type afterActionMutatingPolicy struct{}

func (afterActionMutatingPolicy) Name() string { return "after-action-mutating" }
func (afterActionMutatingPolicy) BeforeRun(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterActionMutatingPolicy) BeforeModel(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterActionMutatingPolicy) AfterModel(_ context.Context, _ *policy.ExecutionContext, _ *model.Action) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterActionMutatingPolicy) AfterAction(_ context.Context, _ *policy.ExecutionContext, action model.Action, result policy.ActionResult) (*policy.PolicyOutcome, error) {
	action.Calls[0].Input["path"] = "tampered.txt"
	result.ToolCalls[0].Input["path"] = "tampered.txt"
	result.ToolResults[0].Result["nested"].(map[string]any)["value"] = "tampered"
	return policy.Continue(), nil
}
func (afterActionMutatingPolicy) BeforeFinish(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

type afterModelBlockingPolicy struct{}

type countingAfterModelPolicy struct {
	name            string
	outcome         *policy.PolicyOutcome
	afterModelCalls int
}

func (p *countingAfterModelPolicy) Name() string { return p.name }
func (*countingAfterModelPolicy) BeforeRun(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (*countingAfterModelPolicy) BeforeModel(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (p *countingAfterModelPolicy) AfterModel(_ context.Context, _ *policy.ExecutionContext, _ *model.Action) (*policy.PolicyOutcome, error) {
	p.afterModelCalls++
	return p.outcome, nil
}
func (*countingAfterModelPolicy) AfterAction(_ context.Context, _ *policy.ExecutionContext, _ model.Action, _ policy.ActionResult) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (*countingAfterModelPolicy) BeforeFinish(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}

func (afterModelBlockingPolicy) Name() string { return "after-model-blocking" }
func (afterModelBlockingPolicy) BeforeRun(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterModelBlockingPolicy) BeforeModel(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterModelBlockingPolicy) AfterModel(_ context.Context, _ *policy.ExecutionContext, _ *model.Action) (*policy.PolicyOutcome, error) {
	return &policy.PolicyOutcome{
		Decision: policy.DecisionBlock,
		Reason:   "blocked_by_test",
	}, nil
}
func (afterModelBlockingPolicy) AfterAction(_ context.Context, _ *policy.ExecutionContext, _ model.Action, _ policy.ActionResult) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
func (afterModelBlockingPolicy) BeforeFinish(_ context.Context, _ *policy.ExecutionContext) (*policy.PolicyOutcome, error) {
	return policy.Continue(), nil
}
