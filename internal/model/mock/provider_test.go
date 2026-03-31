package mock

import (
	"context"
	"testing"

	"github.com/huanglei214/agent-demo/internal/agent"
	"github.com/huanglei214/agent-demo/internal/model"
)

func TestGenerateStreamEmitsOrderedDeltas(t *testing.T) {
	t.Parallel()

	provider := New()
	req := model.Request{Input: "Hello, world"}
	resp, err := provider.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	wantAnswer := agent.ParseAction(resp.Text).Answer

	sink := &capturingStreamSink{}
	if err := provider.GenerateStream(context.Background(), req, sink); err != nil {
		t.Fatalf("generate stream: %v", err)
	}

	want := []string{"mock response: Hello", ", ", "world"}
	if got := sink.deltas; len(got) != len(want) {
		t.Fatalf("expected %d streamed deltas, got %#v", len(want), got)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("expected delta %d to be %q, got %q (all deltas=%#v)", i, want[i], got[i], got)
			}
		}
	}
	if sink.started != 1 {
		t.Fatalf("expected one start event, got %d", sink.started)
	}
	if sink.completed != 1 {
		t.Fatalf("expected one completion event, got %d", sink.completed)
	}
	if sink.failed != 0 {
		t.Fatalf("expected no failures, got %d", sink.failed)
	}
	if got := sink.text(); got != wantAnswer {
		t.Fatalf("expected streamed answer %q, got %q", wantAnswer, got)
	}
}

type capturingStreamSink struct {
	started   int
	completed int
	failed    int
	deltas    []string
	answer    string
}

func (s *capturingStreamSink) Start() error {
	s.started++
	return nil
}

func (s *capturingStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	s.answer += text
	return nil
}

func (s *capturingStreamSink) Complete() error {
	s.completed++
	return nil
}

func (s *capturingStreamSink) Fail(err error) error {
	_ = err
	s.failed++
	return nil
}

func (s *capturingStreamSink) text() string {
	return s.answer
}
