# Answer Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add first-version assistant answer streaming for the Web chat flow so users see continuous answer deltas while the run keeps executing independently of the current HTTP connection, and only the final assistant message is persisted to `.runtime`.

**Architecture:** Keep `.runtime` as the source of truth for final stable run results while introducing a separate ephemeral stream path for online answer deltas. Decouple AG-UI request lifetime from run lifetime first, then add an observer-side answer stream channel, then teach the model layer and executor to emit real answer deltas, and finally wire the AG-UI mapper and Web UI to append them.

**Tech Stack:** Go, existing `internal/model`, `internal/agent`, `internal/service`, `internal/interfaces/http/agui`, React + TypeScript Web UI, Go test, Vite web build

---

## File Map

### Runtime / orchestration

- `internal/agent/observer.go`
  - Extend observer support beyond persisted runtime events
- `internal/agent/helpers.go`
  - Preserve final assistant message persistence semantics
- `internal/agent/loop.go`
  - Insert streamed-final-answer execution path before final result persistence
- `internal/interfaces/http/agui/service.go`
  - Separate request cancellation from run execution and forward ephemeral stream events
- `internal/interfaces/http/agui/mapper.go`
  - Preserve runtime event mapping for persisted events only
- `internal/interfaces/http/handlers_agui.go`
  - Keep HTTP request handling compatible with decoupled run execution

### Model layer

- `internal/model/model.go`
  - Add optional streaming capability types
- `internal/model/mock/provider.go`
  - Add deterministic streaming behavior for tests
- `internal/model/ark/provider.go`
  - Add real or compatible streaming support, with fallback to non-streaming when unavailable

### Service / tests

- `internal/service/run_test.go`
  - Add run-level tests for cancellation semantics, persistence, and fallback behavior
- `internal/interfaces/http/router_test.go`
  - Add AG-UI endpoint tests for connection break / final result semantics
- `internal/interfaces/http/agui/mapper_test.go`
  - Keep mapping expectations aligned with final-message persistence behavior
- `internal/agent/loop_test.go`
  - Add streamed-final-answer loop behavior tests

### Web UI

- `web/src/lib/types.ts`
  - Keep AG-UI event typing aligned with repeated answer deltas
- `web/src/lib/api.ts`
  - Preserve streaming transport, but allow caller to distinguish stream interruption from run failure
- `web/src/pages/ChatPage.tsx`
  - Append repeated answer deltas and present interrupted-stream messaging without falsely marking the run failed

---

### Task 1: Decouple AG-UI request lifetime from run execution

**Files:**
- Modify: `internal/interfaces/http/agui/service.go`
- Modify: `internal/interfaces/http/handlers_agui.go`
- Test: `internal/interfaces/http/router_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write the failing AG-UI cancellation test**

Add a new HTTP-level test in `internal/interfaces/http/router_test.go` that:

```go
func TestAGUIChatDisconnectDoesNotFailRun(t *testing.T) {
    t.Parallel()

    handler, services := newTestHandler(t)
    req := httptest.NewRequest(http.MethodPost, "/api/agui/chat", bytes.NewBufferString(`{
        "messages": [{"id":"msg_user_1","role":"user","content":"Summarize this repository"}],
        "state": {"workspace":"`+services.Config.Workspace+`","provider":"mock","model":"mock-model","maxTurns":4}
    }`))
    ctx, cancel := context.WithCancel(req.Context())
    req = req.WithContext(ctx)

    recorder := httptest.NewRecorder()
    done := make(chan struct{})
    go func() {
        handler.ServeHTTP(recorder, req)
        close(done)
    }()

    cancel()
    <-done

    runs, err := services.ListRuns(1)
    if err != nil {
        t.Fatalf("list runs: %v", err)
    }
    if len(runs) == 0 {
        t.Fatalf("expected at least one run")
    }

    inspection, err := services.InspectRun(runs[0].ID)
    if err != nil {
        t.Fatalf("inspect run: %v", err)
    }
    if inspection.Run.Status == harnessruntime.RunFailed {
        t.Fatalf("expected disconnected AG-UI request not to fail run, got %#v", inspection.Run)
    }
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/interfaces/http -run TestAGUIChatDisconnectDoesNotFailRun -v`

Expected: FAIL because the AG-UI request currently passes `r.Context()` into the run and disconnect cancellation can fail the run.

- [ ] **Step 3: Implement decoupled run execution context in AG-UI service**

Update `internal/interfaces/http/agui/service.go` so the run is started with a context that is not canceled when the HTTP request ends. Preserve request-scoped streaming writes separately.

Use logic equivalent to:

```go
runCtx := context.Background()
observer := newChannelObserver()

go func() {
    response, err := s.services.StartRunStream(runCtx, service.RunRequest{
        Instruction: message.Content,
        Workspace:   workspace,
        Provider:    provider,
        Model:       model,
        MaxTurns:    maxTurns,
        SessionID:   threadID,
        PlanMode:    harnessruntime.PlanMode(planMode),
    }, observer)
    outcomeCh <- runOutcome{response: response, err: err}
    close(outcomeCh)
    close(observer.events)
}()
```

In the event-writing loop, when `writer.Write(...)` fails, stop consuming or forwarding events for this request, but do not cancel the run.

- [ ] **Step 4: Preserve clean handler behavior for disconnected writers**

In `internal/interfaces/http/handlers_agui.go`, keep the handler from writing a second `RUN_ERROR` if the stream has already become unwritable due to client disconnect.

Use a guard similar to:

```go
if err := service.StreamChat(r.Context(), req, writer); err != nil {
    log.Printf("agui chat failed ... error=%v", err)
    _ = writer.Write(agui.Event{Type: "RUN_ERROR", Error: err.Error()})
    return
}
```

but only attempt the fallback `RUN_ERROR` write when the writer is still usable.

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/interfaces/http ./internal/service -run 'TestAGUIChatDisconnectDoesNotFailRun|TestStartRunReturnsContextCanceledWhenRequestContextIsCanceled' -v`

Expected: PASS, with the AG-UI disconnect test proving request disconnect no longer forces run failure, while the service-level explicit canceled-context test still passes.

- [ ] **Step 6: Commit**

```bash
git add internal/interfaces/http/agui/service.go internal/interfaces/http/handlers_agui.go internal/interfaces/http/router_test.go internal/service/run_test.go
git commit -m "fix: decouple agui request cancellation from run execution"
```

### Task 2: Add an ephemeral answer stream observer channel

**Files:**
- Modify: `internal/agent/observer.go`
- Modify: `internal/interfaces/http/agui/service.go`
- Test: `internal/service/run_test.go`
- Test: `internal/agent/loop_test.go`

- [ ] **Step 1: Write the failing observer stream test**

Add a loop- or service-level test that captures both persisted runtime events and ephemeral answer stream events:

```go
type captureObserver struct {
    runtimeEvents []harnessruntime.Event
    streamEvents  []agent.AnswerStreamEvent
}

func (o *captureObserver) OnRuntimeEvent(event harnessruntime.Event) {
    o.runtimeEvents = append(o.runtimeEvents, event)
}

func (o *captureObserver) OnAnswerStreamEvent(event agent.AnswerStreamEvent) {
    o.streamEvents = append(o.streamEvents, event)
}
```

Assert that a streamed answer produces:

- at least one `AnswerStreamEventDelta`
- exactly one persisted `assistant.message`

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent ./internal/service -run 'Test.*AnswerStream.*|Test.*StreamObserver.*' -v`

Expected: FAIL because `RunObserver` does not yet expose a stream event hook.

- [ ] **Step 3: Add minimal observer-side answer stream types**

In `internal/agent/observer.go`, add types along the lines of:

```go
type AnswerStreamEventType string

const (
    AnswerStreamEventStart     AnswerStreamEventType = "start"
    AnswerStreamEventDelta     AnswerStreamEventType = "delta"
    AnswerStreamEventCompleted AnswerStreamEventType = "completed"
    AnswerStreamEventFailed    AnswerStreamEventType = "failed"
)

type AnswerStreamEvent struct {
    RunID      string
    SessionID  string
    MessageID  string
    Type       AnswerStreamEventType
    Delta      string
    ErrMessage string
}
```

Extend `RunObserver` with:

```go
OnAnswerStreamEvent(event AnswerStreamEvent)
```

and update the noop implementation accordingly.

- [ ] **Step 4: Update AG-UI channel observer to forward answer stream events**

Extend `channelObserver` in `internal/interfaces/http/agui/service.go` to hold a second channel for answer stream events:

```go
type channelObserver struct {
    events       chan harnessruntime.Event
    answerStream chan agent.AnswerStreamEvent
}
```

Implement:

```go
func (o *channelObserver) OnAnswerStreamEvent(event agent.AnswerStreamEvent) {
    o.answerStream <- event
}
```

Leave runtime event behavior unchanged.

- [ ] **Step 5: Run the focused test to verify it passes**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent ./internal/service -run 'Test.*AnswerStream.*|Test.*StreamObserver.*' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/observer.go internal/interfaces/http/agui/service.go internal/agent/loop_test.go internal/service/run_test.go
git commit -m "feat: add ephemeral answer stream observer channel"
```

### Task 3: Add optional model streaming capability and mock support

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/model/mock/provider.go`
- Modify: `internal/model/ark/provider.go`
- Test: `internal/model/ark/provider_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write the failing mock streaming test**

Add a model-layer or service-layer test that uses the mock provider to emit multiple answer deltas and asserts they are observed in order.

Use expectations like:

```go
want := []string{"Hello", ", ", "world"}
```

and assert the stream observer sees three ordered deltas, while the final persisted answer becomes `"Hello, world"`.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/model ./internal/service -run 'Test.*Mock.*Streaming.*|Test.*StreamedFinalAnswer.*' -v`

Expected: FAIL because the model layer has no streaming capability yet.

- [ ] **Step 3: Add optional streaming types to the model package**

In `internal/model/model.go`, add a separate optional interface instead of replacing `Model`:

```go
type StreamSink interface {
    Start() error
    Delta(text string) error
    Complete() error
    Fail(err error) error
}

type StreamingModel interface {
    GenerateStream(ctx context.Context, req Request, sink StreamSink) error
}
```

Keep the existing `Model` interface unchanged.

- [ ] **Step 4: Implement deterministic streaming in the mock provider**

Update `internal/model/mock/provider.go` to implement `StreamingModel` with simple chunked output based on the existing final text.

Use behavior equivalent to:

```go
func (p Provider) GenerateStream(ctx context.Context, req model.Request, sink model.StreamSink) error {
    resp, err := p.Generate(ctx, req)
    if err != nil {
        _ = sink.Fail(err)
        return err
    }
    if err := sink.Start(); err != nil {
        return err
    }
    for _, chunk := range []string{"Hello", ", ", "world"} {
        if err := sink.Delta(chunk); err != nil {
            return err
        }
    }
    if err := sink.Complete(); err != nil {
        return err
    }
    return nil
}
```

Use actual response text chunking rather than the literal chunks shown above.

- [ ] **Step 5: Add ark fallback shape without breaking current generate path**

In `internal/model/ark/provider.go`, add a `GenerateStream(...)` implementation if the provider can support real streaming. If not, implement a compatibility fallback that calls `Generate(...)` and emits a single delta through the sink.

The key requirement for this step is not “true token streaming” yet; it is “streaming interface available with safe fallback behavior”.

- [ ] **Step 6: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/model ./internal/service -run 'Test.*Mock.*Streaming.*|Test.*StreamedFinalAnswer.*' -v`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/model/model.go internal/model/mock/provider.go internal/model/ark/provider.go internal/model/ark/provider_test.go internal/service/run_test.go
git commit -m "feat: add optional model answer streaming support"
```

### Task 4: Stream final answers through the executor without persisting deltas

**Files:**
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/helpers.go`
- Test: `internal/agent/loop_test.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write the failing executor streaming test**

Add a test that verifies a streamed final answer:

- emits `AnswerStreamEventStart`
- emits multiple `AnswerStreamEventDelta`
- emits `AnswerStreamEventCompleted`
- persists exactly one final `assistant.message`
- persists no answer delta runtime events

Also assert the final persisted assistant message equals the concatenated deltas.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent ./internal/service -run 'Test.*StreamedFinalAnswer.*|Test.*DoesNotPersistAnswerDeltas.*' -v`

Expected: FAIL because the executor currently only persists a final assistant message after a non-streamed model response.

- [ ] **Step 3: Add a streamed-final-answer execution path**

Refactor final-answer handling in `internal/agent/loop.go` so that when the chosen provider implements `model.StreamingModel`, the executor:

```go
var answer strings.Builder
messageID := harnessruntime.NewID("msg")
started := false

sink := newExecutorAnswerStreamSink(func(delta string) error {
    if !started {
        exec.observer.OnAnswerStreamEvent(agent.AnswerStreamEvent{
            RunID: exec.run.ID, SessionID: exec.session.ID, MessageID: messageID, Type: agent.AnswerStreamEventStart,
        })
        started = true
    }
    answer.WriteString(delta)
    exec.observer.OnAnswerStreamEvent(agent.AnswerStreamEvent{
        RunID: exec.run.ID, SessionID: exec.session.ID, MessageID: messageID, Type: agent.AnswerStreamEventDelta, Delta: delta,
    })
    return nil
})
```

On success, emit `Completed`, then pass `answer.String()` into the existing assistant message persistence path.

- [ ] **Step 4: Keep non-streaming fallback behavior intact**

If the provider does not implement `model.StreamingModel`, preserve the old final-answer behavior so the rest of the system still works.

Do not create any runtime events for answer deltas. The only persisted final-answer event should remain `assistant.message`.

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/agent ./internal/service -run 'Test.*StreamedFinalAnswer.*|Test.*DoesNotPersistAnswerDeltas.*' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/loop.go internal/agent/helpers.go internal/agent/loop_test.go internal/service/run_test.go
git commit -m "feat: stream final answers without persisting deltas"
```

### Task 5: Map answer stream events to AG-UI and update Web UI behavior

**Files:**
- Modify: `internal/interfaces/http/agui/service.go`
- Modify: `internal/interfaces/http/agui/mapper_test.go`
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/pages/ChatPage.tsx`
- Test: `internal/interfaces/http/router_test.go`
- Test: `make web-build`

- [ ] **Step 1: Write the failing AG-UI/Web stream delta test**

Add or extend tests so the AG-UI stream body contains multiple answer content chunks instead of a single content event when using a streamed provider.

Use assertions like:

```go
if strings.Count(bodyText, `"type":"TEXT_MESSAGE_CONTENT"`) < 2 {
    t.Fatalf("expected multiple streamed answer chunks, got %s", bodyText)
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/interfaces/http -run 'TestAGUIChat.*Stream.*|TestAGUIChatEndpoint' -v`

Expected: FAIL because AG-UI currently only emits a single content event from the final `assistant.message` runtime event.

- [ ] **Step 3: Forward ephemeral answer stream events through AG-UI**

In `internal/interfaces/http/agui/service.go`, add a select branch for the observer's answer stream channel and write SSE events directly:

```go
case item, ok := <-observer.answerStream:
    if !ok {
        observer.answerStream = nil
        continue
    }
    switch item.Type {
    case agent.AnswerStreamEventStart:
        _ = writer.Write(Event{Type: "TEXT_MESSAGE_START", ThreadID: item.SessionID, RunID: item.RunID, MessageID: item.MessageID, Role: "assistant"})
    case agent.AnswerStreamEventDelta:
        _ = writer.Write(Event{Type: "TEXT_MESSAGE_CONTENT", ThreadID: item.SessionID, RunID: item.RunID, MessageID: item.MessageID, Delta: item.Delta})
    case agent.AnswerStreamEventCompleted:
        _ = writer.Write(Event{Type: "TEXT_MESSAGE_END", ThreadID: item.SessionID, RunID: item.RunID, MessageID: item.MessageID})
    }
```

Keep `MapRuntimeEvent(...)` unchanged for persisted runtime events; it should continue to map final `assistant.message` for replay / inspect paths.

- [ ] **Step 4: Update the Web UI to distinguish stream interruption from run failure**

In `web/src/pages/ChatPage.tsx`, preserve repeated `TEXT_MESSAGE_CONTENT` append behavior, but stop unconditionally treating a broken AG-UI fetch stream as run failure.

Update the catch path to set a softer error when some answer content has already been received. Use behavior like:

```ts
setStreamState((current) => (current === "failed" ? current : "finished"));
setError("Connection interrupted. The run may still be completing in the background.");
setActivityOpen(true);
```

only when the interruption is transport-level rather than an explicit `RUN_ERROR` event.

- [ ] **Step 5: Run the focused tests and build to verify they pass**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/interfaces/http -run 'TestAGUIChat.*Stream.*|TestAGUIChatEndpoint' -v`

Expected: PASS.

Run: `make web-build`

Expected: successful Vite production build.

- [ ] **Step 6: Commit**

```bash
git add internal/interfaces/http/agui/service.go internal/interfaces/http/agui/mapper_test.go internal/interfaces/http/router_test.go web/src/lib/types.ts web/src/lib/api.ts web/src/pages/ChatPage.tsx
git commit -m "feat: stream assistant answer deltas to agui web clients"
```

### Task 6: Full verification and documentation sync

**Files:**
- Modify: `docs/superpowers/specs/2026-03-30-answer-streaming-design.md` (only if implementation diverged)
- Modify: `docs/superpowers/plans/2026-03-30-answer-streaming-implementation.md`

- [ ] **Step 1: Run the targeted backend verification suite**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./internal/model ./internal/agent ./internal/service ./internal/interfaces/http/... -v`

Expected: PASS.

- [ ] **Step 2: Run the broader repository verification suite**

Run: `GOCACHE=/tmp/agent-demo-gocache go test ./...`

Expected: PASS.

- [ ] **Step 3: Re-check spec and implementation alignment**

Verify the implemented behavior still matches the approved design on these points:

- request disconnect does not fail runs
- only the current online connection sees answer deltas
- `.runtime` persists final assistant messages only
- replay does not attempt to reconstruct token streaming
- providers without streaming support still produce final answers

If implementation diverged, update:

```md
- docs/superpowers/specs/2026-03-30-answer-streaming-design.md
```

with the exact shipped behavior.

- [ ] **Step 4: Commit final verification / doc sync if needed**

```bash
git add docs/superpowers/specs/2026-03-30-answer-streaming-design.md docs/superpowers/plans/2026-03-30-answer-streaming-implementation.md
git commit -m "docs: finalize answer streaming implementation notes"
```
