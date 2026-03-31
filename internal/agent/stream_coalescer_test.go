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

	if downstream.started != 1 {
		t.Fatalf("expected downstream start to be called once, got %d", downstream.started)
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "你好，" {
		t.Fatalf("expected punctuation flush, got %#v", downstream.deltas)
	}
	if got := downstream.calls; len(got) != 2 || got[0] != "Start" || got[1] != "Delta:你好，" {
		t.Fatalf("expected downstream start before punctuation flush, got %#v", downstream.calls)
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
	clock.Advance(19 * time.Millisecond)
	if err := sink.Delta(", "); err != nil {
		t.Fatalf("delta 3: %v", err)
	}

	if len(downstream.deltas) != 0 {
		t.Fatalf("expected no flush inside short window, got %#v", downstream.deltas)
	}
	if downstream.started != 1 {
		t.Fatalf("expected downstream start to be called once, got %d", downstream.started)
	}

	if err := sink.Complete(); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if downstream.completed != 1 {
		t.Fatalf("expected downstream complete to be called once, got %d", downstream.completed)
	}
	if got := downstream.calls; len(got) != 3 || got[0] != "Start" || got[1] != "Delta:mock response: Hello, " || got[2] != "Complete" {
		t.Fatalf("expected flush before complete, got %#v", downstream.calls)
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

	if downstream.started != 1 {
		t.Fatalf("expected downstream start to be called once, got %d", downstream.started)
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "abcdefgh" {
		t.Fatalf("expected hard latency flush, got %#v", downstream.deltas)
	}
	if got := downstream.calls; len(got) != 2 || got[0] != "Start" || got[1] != "Delta:abcdefgh" {
		t.Fatalf("expected downstream start before latency flush, got %#v", downstream.calls)
	}
}

func TestCoalescingSinkFlushesOnExactBoundaries(t *testing.T) {
	policy := coalescingPolicyDefaults()

	t.Run("early flush window", func(t *testing.T) {
		clock := newFakeCoalescingClock(time.Unix(0, 0))
		downstream := &captureModelStreamSink{}
		sink := newCoalescingStreamSink(downstream, policy, clock.Now)

		if err := sink.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := sink.Delta("hello"); err != nil {
			t.Fatalf("delta 1: %v", err)
		}
		clock.Advance(policy.EarlyFlushWindow)
		if err := sink.Delta(" "); err != nil {
			t.Fatalf("delta 2: %v", err)
		}

		if got := downstream.deltas; len(got) != 1 || got[0] != "hello " {
			t.Fatalf("expected flush at exact early boundary, got %#v", downstream.deltas)
		}
	})

	t.Run("flush window", func(t *testing.T) {
		clock := newFakeCoalescingClock(time.Unix(0, 0))
		downstream := &captureModelStreamSink{}
		sink := newCoalescingStreamSink(downstream, policy, clock.Now)

		if err := sink.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := sink.Delta("123456789012345"); err != nil {
			t.Fatalf("delta 1: %v", err)
		}
		clock.Advance(policy.FlushWindow)
		if err := sink.Delta("7"); err != nil {
			t.Fatalf("delta 2: %v", err)
		}

		if got := downstream.deltas; len(got) != 1 || got[0] != "1234567890123457" {
			t.Fatalf("expected flush at exact flush window, got %#v", downstream.deltas)
		}
	})

	t.Run("max latency", func(t *testing.T) {
		clock := newFakeCoalescingClock(time.Unix(0, 0))
		downstream := &captureModelStreamSink{}
		sink := newCoalescingStreamSink(downstream, policy, clock.Now)

		if err := sink.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := sink.Delta("abc"); err != nil {
			t.Fatalf("delta 1: %v", err)
		}
		clock.Advance(policy.MaxLatency)
		if err := sink.Delta("d"); err != nil {
			t.Fatalf("delta 2: %v", err)
		}

		if got := downstream.deltas; len(got) != 1 || got[0] != "abcd" {
			t.Fatalf("expected flush at exact latency limit, got %#v", downstream.deltas)
		}
	})
}

func TestCoalescingSinkAbortsWhenStartFails(t *testing.T) {
	clock := newFakeCoalescingClock(time.Unix(0, 0))
	downstream := &failingStartStreamSink{failStart: true}
	sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)
	coalescer, ok := sink.(*coalescingStreamSink)
	if !ok {
		t.Fatalf("expected coalescing sink type, got %T", sink)
	}

	if err := sink.Start(); err == nil {
		t.Fatalf("expected start to fail")
	}

	if coalescer.started {
		t.Fatalf("expected sink not to remain started after start failure")
	}
	if !coalescer.aborted {
		t.Fatalf("expected sink to abort after start failure")
	}

	if err := sink.Delta("partial answer."); err != nil {
		t.Fatalf("expected later delta to be ignored after abort, got %v", err)
	}
	if err := sink.Complete(); err != nil {
		t.Fatalf("expected later complete to be ignored after abort, got %v", err)
	}
	if downstream.started != 1 {
		t.Fatalf("expected downstream start to be called once, got %d", downstream.started)
	}
	if len(downstream.deltas) != 0 {
		t.Fatalf("expected no downstream deltas after start failure, got %#v", downstream.deltas)
	}
	if downstream.completed != 0 {
		t.Fatalf("expected no downstream complete after start failure, got %d", downstream.completed)
	}
}

func TestCoalescingSinkAbortsWhenDeltaFlushFails(t *testing.T) {
	clock := newFakeCoalescingClock(time.Unix(0, 0))
	downstream := &failingDeltaStreamSink{failNextDelta: true}
	sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)
	coalescer, ok := sink.(*coalescingStreamSink)
	if !ok {
		t.Fatalf("expected coalescing sink type, got %T", sink)
	}

	if err := sink.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := sink.Delta("partial answer."); err == nil {
		t.Fatalf("expected delta flush to fail")
	}

	if coalescer.started {
		t.Fatalf("expected sink not to remain started after delta failure")
	}
	if !coalescer.aborted {
		t.Fatalf("expected sink to abort after delta failure")
	}
	if got := coalescer.pending.String(); got != "" {
		t.Fatalf("expected pending text to be cleared on abort, got %q", got)
	}
	if coalescer.pendingRunes != 0 {
		t.Fatalf("expected pending rune count to be cleared on abort, got %d", coalescer.pendingRunes)
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "partial answer." {
		t.Fatalf("expected attempted flush to reach downstream once, got %#v", downstream.deltas)
	}

	if err := sink.Complete(); err != nil {
		t.Fatalf("expected later complete to be ignored after abort, got %v", err)
	}
	if got := downstream.deltas; len(got) != 1 {
		t.Fatalf("expected no retry of buffered text on complete, got %#v", downstream.deltas)
	}
	if downstream.completed != 0 {
		t.Fatalf("expected no downstream complete after delta failure, got %d", downstream.completed)
	}
	if len(downstream.calls) != 2 || downstream.calls[0] != "Start" || downstream.calls[1] != "Delta:partial answer." {
		t.Fatalf("expected abort to stop after failed delta, got %#v", downstream.calls)
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

	if downstream.started != 1 {
		t.Fatalf("expected downstream start to be called once, got %d", downstream.started)
	}
	boom := errors.New("boom")
	if err := sink.Fail(boom); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if got := downstream.calls; len(got) != 3 || got[0] != "Start" || got[1] != "Delta:partial answer" || got[2] != "Fail:boom" {
		t.Fatalf("expected flush before fail, got %#v", downstream.calls)
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "partial answer" {
		t.Fatalf("expected fail to flush buffered text, got %#v", downstream.deltas)
	}
	if downstream.failed == nil || downstream.failed.Error() != "boom" {
		t.Fatalf("expected downstream failure, got %#v", downstream.failed)
	}
}

func TestCoalescingSinkAbortsWhenCompleteFails(t *testing.T) {
	clock := newFakeCoalescingClock(time.Unix(0, 0))
	downstream := &failingCompleteStreamSink{failComplete: true}
	sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)
	coalescer, ok := sink.(*coalescingStreamSink)
	if !ok {
		t.Fatalf("expected coalescing sink type, got %T", sink)
	}

	if err := sink.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := sink.Delta("partial answer."); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if err := sink.Complete(); err == nil {
		t.Fatalf("expected complete to fail")
	}

	if coalescer.started {
		t.Fatalf("expected sink not to remain started after complete failure")
	}
	if !coalescer.aborted {
		t.Fatalf("expected sink to abort after complete failure")
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "partial answer." {
		t.Fatalf("expected buffered text to flush before complete failure, got %#v", downstream.deltas)
	}
	if downstream.completed != 1 {
		t.Fatalf("expected downstream complete to be called once, got %d", downstream.completed)
	}

	if err := sink.Delta("ignored"); err != nil {
		t.Fatalf("expected later delta to be ignored after complete failure, got %v", err)
	}
	if err := sink.Complete(); err != nil {
		t.Fatalf("expected later complete to be ignored after abort, got %v", err)
	}
	if len(downstream.calls) != 3 || downstream.calls[0] != "Start" || downstream.calls[1] != "Delta:partial answer." || downstream.calls[2] != "Complete" {
		t.Fatalf("expected abort to stop after failed complete, got %#v", downstream.calls)
	}
}

func TestCoalescingSinkAbortsWhenFailFails(t *testing.T) {
	clock := newFakeCoalescingClock(time.Unix(0, 0))
	downstream := &failingFailStreamSink{failFail: true}
	sink := newCoalescingStreamSink(downstream, coalescingPolicyDefaults(), clock.Now)
	coalescer, ok := sink.(*coalescingStreamSink)
	if !ok {
		t.Fatalf("expected coalescing sink type, got %T", sink)
	}

	if err := sink.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := sink.Delta("partial answer"); err != nil {
		t.Fatalf("delta: %v", err)
	}
	boom := errors.New("boom")
	if err := sink.Fail(boom); err == nil {
		t.Fatalf("expected fail to fail")
	}

	if coalescer.started {
		t.Fatalf("expected sink not to remain started after fail failure")
	}
	if !coalescer.aborted {
		t.Fatalf("expected sink to abort after fail failure")
	}
	if got := downstream.deltas; len(got) != 1 || got[0] != "partial answer" {
		t.Fatalf("expected buffered text to flush before fail failure, got %#v", downstream.deltas)
	}
	if downstream.failed != boom {
		t.Fatalf("expected downstream failure, got %#v", downstream.failed)
	}

	if err := sink.Delta("ignored"); err != nil {
		t.Fatalf("expected later delta to be ignored after fail failure, got %v", err)
	}
	if err := sink.Complete(); err != nil {
		t.Fatalf("expected later complete to be ignored after abort, got %v", err)
	}
	if len(downstream.calls) != 3 || downstream.calls[0] != "Start" || downstream.calls[1] != "Delta:partial answer" || downstream.calls[2] != "Fail:boom" {
		t.Fatalf("expected abort to stop after failed fail, got %#v", downstream.calls)
	}
}

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
	calls     []string
}

func (s *captureModelStreamSink) Start() error {
	s.started++
	s.calls = append(s.calls, "Start")
	return nil
}

func (s *captureModelStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.calls = append(s.calls, "Delta:"+text)
	return nil
}

func (s *captureModelStreamSink) Complete() error {
	s.completed++
	s.calls = append(s.calls, "Complete")
	return nil
}

func (s *captureModelStreamSink) Fail(err error) error {
	s.failed = err
	s.calls = append(s.calls, "Fail:"+err.Error())
	return nil
}

type failingStartStreamSink struct {
	started   int
	deltas    []string
	completed int
	failed    error
	calls     []string
	failStart bool
}

func (s *failingStartStreamSink) Start() error {
	s.started++
	s.calls = append(s.calls, "Start")
	if s.failStart {
		s.failStart = false
		return errors.New("start failed")
	}
	return nil
}

func (s *failingStartStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.calls = append(s.calls, "Delta:"+text)
	return nil
}

func (s *failingStartStreamSink) Complete() error {
	s.completed++
	s.calls = append(s.calls, "Complete")
	return nil
}

func (s *failingStartStreamSink) Fail(err error) error {
	s.failed = err
	s.calls = append(s.calls, "Fail:"+err.Error())
	return nil
}

type failingDeltaStreamSink struct {
	started       int
	deltas        []string
	completed     int
	failed        error
	calls         []string
	failNextDelta bool
}

func (s *failingDeltaStreamSink) Start() error {
	s.started++
	s.calls = append(s.calls, "Start")
	return nil
}

func (s *failingDeltaStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.calls = append(s.calls, "Delta:"+text)
	if s.failNextDelta {
		s.failNextDelta = false
		return errors.New("delta failed")
	}
	return nil
}

func (s *failingDeltaStreamSink) Complete() error {
	s.completed++
	s.calls = append(s.calls, "Complete")
	return nil
}

func (s *failingDeltaStreamSink) Fail(err error) error {
	s.failed = err
	s.calls = append(s.calls, "Fail:"+err.Error())
	return nil
}

type failingCompleteStreamSink struct {
	started       int
	deltas        []string
	completed     int
	failed        error
	calls         []string
	failComplete  bool
}

func (s *failingCompleteStreamSink) Start() error {
	s.started++
	s.calls = append(s.calls, "Start")
	return nil
}

func (s *failingCompleteStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.calls = append(s.calls, "Delta:"+text)
	return nil
}

func (s *failingCompleteStreamSink) Complete() error {
	s.completed++
	s.calls = append(s.calls, "Complete")
	if s.failComplete {
		s.failComplete = false
		return errors.New("complete failed")
	}
	return nil
}

func (s *failingCompleteStreamSink) Fail(err error) error {
	s.failed = err
	s.calls = append(s.calls, "Fail:"+err.Error())
	return nil
}

type failingFailStreamSink struct {
	started   int
	deltas    []string
	completed int
	failed    error
	calls     []string
	failFail  bool
}

func (s *failingFailStreamSink) Start() error {
	s.started++
	s.calls = append(s.calls, "Start")
	return nil
}

func (s *failingFailStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.calls = append(s.calls, "Delta:"+text)
	return nil
}

func (s *failingFailStreamSink) Complete() error {
	s.completed++
	s.calls = append(s.calls, "Complete")
	return nil
}

func (s *failingFailStreamSink) Fail(err error) error {
	s.failed = err
	s.calls = append(s.calls, "Fail:"+err.Error())
	if s.failFail {
		s.failFail = false
		return errors.New("fail failed")
	}
	return nil
}
