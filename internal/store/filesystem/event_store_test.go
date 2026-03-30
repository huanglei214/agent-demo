package filesystem

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestEventStoreAppendReadAllAndNextSequence(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	eventStore := NewEventStore(paths)

	runID := "run_test"
	events := []harnessruntime.Event{
		{
			ID:        "evt_1",
			RunID:     runID,
			SessionID: "session_1",
			TaskID:    "task_1",
			Sequence:  1,
			Type:      "run.created",
			Timestamp: time.Now(),
			Actor:     "system",
			Payload: map[string]any{
				"status": "pending",
			},
		},
		{
			ID:        "evt_2",
			RunID:     runID,
			SessionID: "session_1",
			TaskID:    "task_1",
			Sequence:  2,
			Type:      "run.started",
			Timestamp: time.Now(),
			Actor:     "runtime",
		},
	}

	for _, event := range events {
		if err := eventStore.Append(event); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	got, err := eventStore.ReadAll(runID)
	if err != nil {
		t.Fatalf("read all events: %v", err)
	}

	if len(got) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(got))
	}

	if got[0].Type != "run.created" || got[1].Type != "run.started" {
		t.Fatalf("unexpected event order: %#v", got)
	}

	next, err := eventStore.NextSequence(runID)
	if err != nil {
		t.Fatalf("next sequence: %v", err)
	}

	if next != 3 {
		t.Fatalf("expected next sequence 3, got %d", next)
	}
}

func TestEventStoreNextSequenceForNewRun(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	eventStore := NewEventStore(paths)

	next, err := eventStore.NextSequence("missing_run")
	if err != nil {
		t.Fatalf("next sequence for new run: %v", err)
	}

	if next != 1 {
		t.Fatalf("expected next sequence 1, got %d", next)
	}
}

func TestEventStoreNextSequenceCountsLinesWithoutParsingJSON(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	eventStore := NewEventStore(paths)

	path := paths.EventsPath("run_bad_json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir events dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json}\n{\"ok\":true}\n"), 0o644); err != nil {
		t.Fatalf("write events file: %v", err)
	}

	next, err := eventStore.NextSequence("run_bad_json")
	if err != nil {
		t.Fatalf("next sequence with malformed jsonl: %v", err)
	}
	if next != 3 {
		t.Fatalf("expected next sequence 3, got %d", next)
	}
}

func TestEventStoreReadAfterReturnsOnlyNewerEvents(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	eventStore := NewEventStore(paths)

	runID := "run_after"
	for i := 1; i <= 4; i++ {
		if err := eventStore.Append(harnessruntime.Event{
			ID:        harnessruntime.NewID("evt"),
			RunID:     runID,
			SessionID: "session_1",
			TaskID:    "task_1",
			Sequence:  int64(i),
			Type:      "runtime.event",
			Timestamp: time.Now(),
			Actor:     "runtime",
		}); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	got, err := eventStore.ReadAfter(runID, 2)
	if err != nil {
		t.Fatalf("read events after sequence: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events after sequence 2, got %#v", got)
	}
	if got[0].Sequence != 3 || got[1].Sequence != 4 {
		t.Fatalf("expected sequences 3 and 4, got %#v", got)
	}
}
