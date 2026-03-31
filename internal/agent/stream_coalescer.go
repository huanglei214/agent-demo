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
	aborted      bool
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

func (s *coalescingStreamSink) Start() error {
	if s.started || s.terminal() {
		return nil
	}
	if err := s.downstream.Start(); err != nil {
		s.abort()
		return err
	}
	s.started = true
	s.lastFlushAt = s.now()
	return nil
}

func (s *coalescingStreamSink) Delta(text string) error {
	if text == "" || s.terminal() {
		return nil
	}
	if !s.started {
		if err := s.Start(); err != nil {
			return err
		}
	}

	s.pending.WriteString(text)
	s.pendingRunes += visibleRuneCount(text)

	if s.shouldFlush() {
		return s.flush()
	}
	return nil
}

func (s *coalescingStreamSink) Complete() error {
	if s.terminal() {
		return nil
	}
	if !s.started {
		if err := s.Start(); err != nil {
			return err
		}
	}
	if err := s.flush(); err != nil {
		return err
	}
	s.completed = true
	if err := s.downstream.Complete(); err != nil {
		s.abort()
		return err
	}
	return nil
}

func (s *coalescingStreamSink) Fail(err error) error {
	if s.terminal() {
		return nil
	}
	if s.started {
		if flushErr := s.flush(); flushErr != nil {
			return flushErr
		}
	}
	s.completed = true
	if failErr := s.downstream.Fail(err); failErr != nil {
		s.abort()
		return failErr
	}
	return nil
}

func (s *coalescingStreamSink) terminal() bool {
	return s.completed || s.aborted
}

func (s *coalescingStreamSink) abort() {
	s.aborted = true
	s.started = false
	s.pending.Reset()
	s.pendingRunes = 0
}

func (s *coalescingStreamSink) shouldFlush() bool {
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
	if err := s.downstream.Delta(text); err != nil {
		s.abort()
		return err
	}
	s.pending.Reset()
	s.pendingRunes = 0
	s.lastFlushAt = s.now()
	return nil
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
