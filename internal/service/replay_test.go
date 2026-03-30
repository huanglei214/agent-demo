package service

import (
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestReplayRunAfterReturnsOnlyEventsBeyondSequence(t *testing.T) {
	t.Parallel()

	services := NewServices(newTestConfig(t))
	now := time.Now()

	task, session, run, _, _ := seedStoredRun(t, services, services.Config.Workspace, now, harnessruntime.RunPending, "run_replay_after")
	for sequence := int64(4); sequence <= 5; sequence++ {
		if err := services.EventStore.Append(harnessruntime.Event{
			ID:        harnessruntime.NewID("evt"),
			RunID:     run.ID,
			SessionID: session.ID,
			TaskID:    task.ID,
			Sequence:  sequence,
			Type:      "runtime.event",
			Timestamp: now,
			Actor:     "runtime",
		}); err != nil {
			t.Fatalf("append event %d: %v", sequence, err)
		}
	}

	events, err := services.ReplayRunAfter(run.ID, 3)
	if err != nil {
		t.Fatalf("replay run after: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after sequence 3, got %#v", events)
	}
	if events[0].Sequence != 4 || events[1].Sequence != 5 {
		t.Fatalf("expected sequences 4 and 5, got %#v", events)
	}
}

func newTestConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Load(t.TempDir())
}
