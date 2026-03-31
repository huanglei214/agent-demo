# Stream Coalescing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a common answer-stream coalescing layer so all streaming outputs emit smoother, ChatGPT-like chunks without changing the final answer text.

**Architecture:** Keep providers streaming raw deltas as fast as possible. Wrap the common answer stream sink in `internal/agent` with a coalescing sink that buffers deltas and flushes them on punctuation, short timing windows, or hard latency limits. This keeps Ark, mock, Web AG-UI, and CLI on one policy.

**Tech Stack:** Go, existing `internal/agent` streaming observer pipeline, Go tests, runtime/AG-UI integration tests

---

## File Structure

- Modify: `internal/agent/helpers.go`
  - Current answer stream accumulator lives here; this is the correct common insertion point.
- Create: `internal/agent/stream_coalescer.go`
  - New coalescing sink implementation with injectable clock.
- Create: `internal/agent/stream_coalescer_test.go`
  - Unit tests for buffering, punctuation flush, timeout flush, complete/fail flush.
- Modify: `internal/service/run_test.go`
  - Update streaming service expectations to match coalesced chunks while preserving final text.
- Modify: `internal/agent/loop_test.go`
  - Update loop-level answer stream assertions to match common coalescing behavior.
- Modify: `internal/interfaces/http/router_test.go`
  - Tighten AG-UI assertions so we validate sensible chunking instead of just “multiple chunks”.

## Task 1: Write Coalescing Sink Unit Tests First

**Files:**
- Create: `internal/agent/stream_coalescer_test.go`
- Reference: `internal/model/model.go`

- [ ] **Step 1: Write the failing tests for punctuation, buffering, and completion behavior**

```go
package agent

import (
    "errors"
    "testing"
    "time"
)

func TestCoalescingSinkFlushesOnPunctuation(t *testing.T) {
    clock := newFakeCoalescingClock(time.Unix(0, 0))
    downstream := &captureModelStreamSink{}
    sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)

    if err := sink.Start(); err != nil {
        t.Fatalf("start: %v", err)
    }
    if err := sink.Delta("你好"); err != nil {
        t.Fatalf("delta 1: %v", err)
    }
    if err := sink.Delta("，"); err != nil {
        t.Fatalf("delta 2: %v", err)
    }

    if got := downstream.deltas; len(got) != 1 || got[0] != "你好，" {
        t.Fatalf("expected punctuation flush, got %#v", downstream.deltas)
    }
}

func TestCoalescingSinkBuffersWithinWindow(t *testing.T) {
    clock := newFakeCoalescingClock(time.Unix(0, 0))
    downstream := &captureModelStreamSink{}
    sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)

    if err := sink.Start(); err != nil {
        t.Fatalf("start: %v", err)
    }
    if err := sink.Delta("mock response"); err != nil {
        t.Fatalf("delta 1: %v", err)
    }
    clock.Advance(20 * time.Millisecond)
    if err := sink.Delta(": Hello"); err != nil {
        t.Fatalf("delta 2: %v", err)
    }
    clock.Advance(20 * time.Millisecond)
    if err := sink.Delta(", "); err != nil {
        t.Fatalf("delta 3: %v", err)
    }

    if len(downstream.deltas) != 0 {
        t.Fatalf("expected no flush inside short window, got %#v", downstream.deltas)
    }

    if err := sink.Complete(); err != nil {
        t.Fatalf("complete: %v", err)
    }
    if got := downstream.deltas; len(got) != 1 || got[0] != "mock response: Hello, " {
        t.Fatalf("expected buffered flush on complete, got %#v", downstream.deltas)
    }
}

func TestCoalescingSinkFlushesAfterLatencyLimit(t *testing.T) {
    clock := newFakeCoalescingClock(time.Unix(0, 0))
    downstream := &captureModelStreamSink{}
    sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)

    if err := sink.Start(); err != nil {
        t.Fatalf("start: %v", err)
    }
    if err := sink.Delta("abcdef"); err != nil {
        t.Fatalf("delta 1: %v", err)
    }
    clock.Advance(90 * time.Millisecond)
    if err := sink.Delta("gh"); err != nil {
        t.Fatalf("delta 2: %v", err)
    }

    if got := downstream.deltas; len(got) != 1 || got[0] != "abcdefgh" {
        t.Fatalf("expected hard latency flush, got %#v", downstream.deltas)
    }
}

func TestCoalescingSinkFlushesBeforeFail(t *testing.T) {
    clock := newFakeCoalescingClock(time.Unix(0, 0))
    downstream := &captureModelStreamSink{}
    sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)

    if err := sink.Start(); err != nil {
        t.Fatalf("start: %v", err)
    }
    if err := sink.Delta("partial answer"); err != nil {
        t.Fatalf("delta: %v", err)
    }

    boom := errors.New("boom")
    if err := sink.Fail(boom); err != nil {
        t.Fatalf("fail: %v", err)
    }
    if got := downstream.deltas; len(got) != 1 || got[0] != "partial answer" {
        t.Fatalf("expected fail to flush buffered text, got %#v", downstream.deltas)
    }
    if downstream.failed == nil || downstream.failed.Error() != "boom" {
        t.Fatalf("expected downstream failure, got %#v", downstream.failed)
    }
}
```

- [ ] **Step 2: Add local test helpers in the same test file**

```go
type fakeCoalescingClock struct {
    current time.Time
}

func newFakeCoalescingClock(start time.Time) *fakeCoalescingClock {
    return &fakeCoalescingClock{current: start}
}

func (c *fakeCoalescingClock) Now() time.Time {
    return c.current
}

func (c *fakeCoalescingClock) Advance(d time.Duration) {
    c.current = c.current.Add(d)
}

type captureModelStreamSink struct {
    started   int
    deltas    []string
    completed int
    failed    error
}

func (s *captureModelStreamSink) Start() error {
    s.started++
    return nil
}

func (s *captureModelStreamSink) Delta(text string) error {
    s.deltas = append(s.deltas, text)
    return nil
}

func (s *captureModelStreamSink) Complete() error {
    s.completed++
    return nil
}

func (s *captureModelStreamSink) Fail(err error) error {
    s.failed = err
    return nil
}
```

- [ ] **Step 3: Run the new unit test file to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/agent -run 'TestCoalescingSink' -count=1`
Expected: FAIL with undefined symbols like `newCoalescingStreamSink`, `coalescingPolicyDefaults`, or missing implementation.

- [ ] **Step 4: Commit the failing test scaffold**

```bash
git add internal/agent/stream_coalescer_test.go
git commit -m "test: define stream coalescing behavior"
```

## Task 2: Implement the Common Coalescing Sink

**Files:**
- Create: `internal/agent/stream_coalescer.go`
- Reference: `internal/model/model.go`

- [ ] **Step 1: Add the coalescing sink types and policy defaults**

```go
package agent

import (
    "strings"
    "time"

    "github.com/huanglei214/agent-demo/internal/model"
)

type coalescingPolicy struct {
    FlushWindow      time.Duration
    EarlyFlushWindow time.Duration
    MaxLatency       time.Duration
    MaxVisibleRunes  int
}

func coalescingPolicyDefaults() coalescingPolicy {
    return coalescingPolicy{
        FlushWindow:      50 * time.Millisecond,
        EarlyFlushWindow: 40 * time.Millisecond,
        MaxLatency:       80 * time.Millisecond,
        MaxVisibleRunes:  16,
    }
}

type coalescingStreamSink struct {
    downstream model.StreamSink
    policy     coalescingPolicy
    now        func() time.Time

    started      bool
    completed    bool
    lastFlushAt  time.Time
    pending      strings.Builder
    pendingRunes int
}

func newCoalescingStreamSink(downstream model.StreamSink, policy coalescingPolicy, now func() time.Time) model.StreamSink {
    if downstream == nil {
        return nil
    }
    if now == nil {
        now = time.Now
    }
    return &coalescingStreamSink{
        downstream: downstream,
        policy:     policy,
        now:        now,
    }
}
```

- [ ] **Step 2: Implement lifecycle methods and flush logic**

```go
func (s *coalescingStreamSink) Start() error {
    if s.started {
        return nil
    }
    s.started = true
    s.lastFlushAt = s.now()
    return s.downstream.Start()
}

func (s *coalescingStreamSink) Delta(text string) error {
    if text == "" {
        return nil
    }
    if !s.started {
        if err := s.Start(); err != nil {
            return err
        }
    }

    s.pending.WriteString(text)
    s.pendingRunes += visibleRuneCount(text)

    if s.shouldFlush(text) {
        return s.flush()
    }
    return nil
}

func (s *coalescingStreamSink) Complete() error {
    if !s.started {
        if err := s.Start(); err != nil {
            return err
        }
    }
    if err := s.flush(); err != nil {
        return err
    }
    s.completed = true
    return s.downstream.Complete()
}

func (s *coalescingStreamSink) Fail(err error) error {
    if s.started {
        if flushErr := s.flush(); flushErr != nil {
            return flushErr
        }
    }
    return s.downstream.Fail(err)
}
```

- [ ] **Step 3: Add boundary detection helpers**

```go
func (s *coalescingStreamSink) shouldFlush(text string) bool {
    pending := s.pending.String()
    if pending == "" {
        return false
    }
    if endsWithImmediateBoundary(pending) {
        return true
    }

    elapsed := s.now().Sub(s.lastFlushAt)
    if endsWithSpaceBoundary(pending) && elapsed >= s.policy.EarlyFlushWindow {
        return true
    }
    if s.pendingRunes >= s.policy.MaxVisibleRunes && elapsed >= s.policy.FlushWindow {
        return true
    }
    return elapsed >= s.policy.MaxLatency
}

func (s *coalescingStreamSink) flush() error {
    text := s.pending.String()
    if text == "" {
        return nil
    }
    s.pending.Reset()
    s.pendingRunes = 0
    s.lastFlushAt = s.now()
    return s.downstream.Delta(text)
}

func visibleRuneCount(text string) int {
    count := 0
    for _, r := range text {
        if r == '\r' || r == '\n' {
            continue
        }
        count++
    }
    return count
}

func endsWithImmediateBoundary(text string) bool {
    if text == "" {
        return false
    }
    r := []rune(text)
    switch r[len(r)-1] {
    case '，', '。', '！', '？', '；', '：', ',', '.', '!', '?', ';', ':', '\n':
        return true
    default:
        return false
    }
}

func endsWithSpaceBoundary(text string) bool {
    if text == "" {
        return false
    }
    r := []rune(text)
    return r[len(r)-1] == ' '
}
```

- [ ] **Step 4: Run the coalescing unit tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/agent -run 'TestCoalescingSink' -count=1`
Expected: PASS

- [ ] **Step 5: Commit the sink implementation**

```bash
git add internal/agent/stream_coalescer.go internal/agent/stream_coalescer_test.go
git commit -m "feat: add common stream coalescing sink"
```

## Task 3: Wire the Coalescing Sink into the Common Answer Stream Path

**Files:**
- Modify: `internal/agent/helpers.go`
- Test: `internal/service/run_test.go`

- [ ] **Step 1: Write a failing integration test showing common coalescing at the service layer**

Add this test to `internal/service/run_test.go` near the existing streaming tests:

```go
func TestStartRunStreamCoalescesStreamingAnswerEvents(t *testing.T) {
    t.Parallel()

    workspace := t.TempDir()
    cfg := config.Load(workspace)
    cfg.Workspace = workspace
    cfg.Runtime.Root = filepath.Join(workspace, ".runtime")
    cfg.Model.Provider = "mock"
    cfg.Model.Model = "mock-model"

    observer := &captureStreamingRunObserver{}
    services := newTestServices(t, cfg, func(_ *agent.RuntimeServices, modelServices *agent.ModelServices, _ *agent.AgentServices, _ *agent.ToolServices, _ *agent.DelegationServices) {
        modelServices.ModelFactory = func() (model.Model, error) {
            return &streamingRunModel{chunks: []string{"mock", " response", ": ", "Hello", ", ", "world"}}, nil
        }
    })

    response, err := services.StartRunStream(context.Background(), RunRequest{
        Instruction: "Summarize the repository in one short paragraph",
        Workspace:   workspace,
        Provider:    "mock",
        Model:       "mock-model",
        MaxTurns:    4,
    }, observer)
    if err != nil {
        t.Fatalf("expected run to succeed, got %v", err)
    }
    if response.Result == nil || response.Result.Output != "mock response: Hello, world" {
        t.Fatalf("expected final answer to remain intact, got %#v", response.Result)
    }

    got := observer.answerDeltaTexts()
    want := []string{"mock response: Hello, ", "world"}
    if len(got) != len(want) {
        t.Fatalf("expected coalesced answer deltas %#v, got %#v", want, got)
    }
    for i := range want {
        if got[i] != want[i] {
            t.Fatalf("expected delta %d to be %q, got %#v", i, want[i], got)
        }
    }
}
```

- [ ] **Step 2: Add helper method on the observer test double**

```go
func (o *captureStreamingRunObserver) answerDeltaTexts() []string {
    var deltas []string
    for _, event := range o.answerEvents {
        if event.Type == agent.AnswerStreamEventDelta {
            deltas = append(deltas, event.Delta)
        }
    }
    return deltas
}
```

- [ ] **Step 3: Run the service-layer streaming tests to verify the new test fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/service -run 'TestStartRunStream(ReceivesStreamingAnswerEvents|CoalescesStreamingAnswerEvents)' -count=1`
Expected: FAIL because the old path still forwards raw deltas one-by-one.

- [ ] **Step 4: Wrap the accumulator with the coalescing sink in `internal/agent/helpers.go`**

Replace the direct provider call path with this shape:

```go
func (e *Executor) generateModelResponse(runCtx context.Context, exec *runExecution, req model.Request) (model.Response, error) {
    if streamingProvider, ok := exec.provider.(model.StreamingModel); ok {
        messageID := harnessruntime.NewID("msg")
        accumulator := &answerStreamAccumulator{
            observer:  ensureRunObserver(exec.observer),
            runID:     exec.run.ID,
            sessionID: exec.session.ID,
            messageID: messageID,
        }
        sink := newCoalescingStreamSink(accumulator, coalescingPolicyDefaults(), time.Now)
        err := e.generateStreamWithModelTimeout(runCtx, streamingProvider, req, sink)
        if err != nil {
            var nonFinal *model.NonFinalStreamResponseError
            if errors.As(err, &nonFinal) {
                return nonFinal.Response, nil
            }
            _ = accumulator.Fail(err)
            return model.Response{}, err
        }
        return model.Response{Text: accumulator.text(), FinishReason: "stop"}, nil
    }

    return e.generateWithModelTimeout(runCtx, exec.provider, req)
}
```

- [ ] **Step 5: Run the service-layer streaming tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/service -run 'TestStartRunStream(ReceivesStreamingAnswerEvents|CoalescesStreamingAnswerEvents)' -count=1`
Expected: PASS

- [ ] **Step 6: Commit the common wiring**

```bash
git add internal/agent/helpers.go internal/service/run_test.go
git commit -m "feat: coalesce common answer stream output"
```

## Task 4: Update Agent and AG-UI Assertions Around Coalesced Chunks

**Files:**
- Modify: `internal/agent/loop_test.go`
- Modify: `internal/interfaces/http/router_test.go`

- [ ] **Step 1: Update the loop-level failure-path assertion to match flush-before-fail behavior**

In `internal/agent/loop_test.go`, keep the current failure test but allow the emitted delta to be the coalesced string. The core assertion should remain:

```go
if observer.answerStreamEvents[1].Type != AnswerStreamEventDelta || observer.answerStreamEvents[1].Delta != "partial answer" {
    t.Fatalf("expected second stream event to be coalesced delta, got %#v", observer.answerStreamEvents)
}
```

If any existing test assumes multiple tiny deltas from the streaming model, rewrite it to assert:
- one `AnswerStreamEventStart`
- one or more coalesced `AnswerStreamEventDelta`
- exact concatenated final text
- terminal `AnswerStreamEventCompleted` or `AnswerStreamEventFailed`

- [ ] **Step 2: Tighten AG-UI endpoint assertions in `internal/interfaces/http/router_test.go`**

Replace the generic count assertion with content-shape assertions derived from the mock stream:

```go
if !strings.Contains(bodyText, `"type":"TEXT_MESSAGE_CONTENT"`) {
    t.Fatalf("expected streamed text content, got %s", bodyText)
}
if strings.Contains(bodyText, `"delta":"m"`) {
    t.Fatalf("expected coalesced chunks rather than single-character deltas, got %s", bodyText)
}
```

Also keep the existing lifecycle assertions:
- exactly one `TEXT_MESSAGE_START`
- exactly one `TEXT_MESSAGE_END`
- `TEXT_MESSAGE_END` before `RUN_FINISHED`

- [ ] **Step 3: Run the focused agent and HTTP tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/agent ./internal/interfaces/http -run 'TestExecuteRun(StreamFailureEmitsFailedAnswerStreamEvent|StreamedFinalAnswerPersistsOnlyFinalAssistantMessage)|TestAGUIChatEndpoint' -count=1`
Expected: PASS

- [ ] **Step 4: Commit the updated integration assertions**

```bash
git add internal/agent/loop_test.go internal/interfaces/http/router_test.go
git commit -m "test: update streaming assertions for coalesced chunks"
```

## Task 5: Full Verification and Real Ark Check

**Files:**
- Verify only

- [ ] **Step 1: Run the relevant Go package tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./internal/agent ./internal/service ./internal/interfaces/http ./internal/model/ark -count=1`
Expected: PASS for targeted streaming-related packages, or document any pre-existing unrelated failure.

- [ ] **Step 2: Run a real Ark CLI verification**

Run: `make run PROVIDER=ark MODEL=doubao-1.8 ARGS='请用两三句话总结这个仓库的用途，并尽量连续输出。'`
Expected: terminal output appears in short phrase-like chunks instead of single-character jumps.

- [ ] **Step 3: Run a real AG-UI SSE verification**

Run server in one terminal: `make serve PROVIDER=ark MODEL=doubao-1.8 PORT=18088`

Run request in another terminal:

```bash
curl -N -s -H 'Content-Type: application/json' -H 'Accept: text/event-stream' \
  -X POST http://127.0.0.1:18088/api/agui/chat \
  --data '{"messages":[{"id":"msg_user_1","role":"user","content":"请用两三句话总结这个仓库的用途，并尽量连续输出。"}],"state":{"workspace":"'$PWD'","provider":"ark","model":"doubao-1.8","maxTurns":4}}'
```

Expected:
- multiple `TEXT_MESSAGE_CONTENT` events still occur
- chunk boundaries look like short phrases or short clauses, not one Chinese character per event
- exactly one `TEXT_MESSAGE_START` and one `TEXT_MESSAGE_END`

- [ ] **Step 4: Commit the final verified implementation**

```bash
git add internal/agent/stream_coalescer.go internal/agent/stream_coalescer_test.go internal/agent/helpers.go internal/service/run_test.go internal/agent/loop_test.go internal/interfaces/http/router_test.go
git commit -m "fix: smooth streamed answer chunking"
```
