package memory

import (
	"testing"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestCommitAndRecallUsesStableSortingAndTopN(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)
	now := time.Now()

	entries := []harnessruntime.MemoryEntry{
		{
			ID:          "mem_1",
			SessionID:   "session_1",
			Scope:       "global",
			Kind:        "fact",
			Content:     "Ark provider is configured for this harness.",
			Tags:        []string{"arch:model"},
			SourceRunID: "run_1",
			CreatedAt:   now.Add(-2 * time.Hour),
		},
		{
			ID:          "mem_2",
			SessionID:   "session_1",
			Scope:       "session",
			Kind:        "decision",
			Content:     "Use Hertz for HTTP entrypoints in this harness.",
			Tags:        []string{"arch:http"},
			SourceRunID: "run_2",
			CreatedAt:   now.Add(-1 * time.Hour),
		},
		{
			ID:          "mem_3",
			SessionID:   "session_1",
			Scope:       "workspace",
			Kind:        "convention",
			Content:     "Keep event-first runtime design in this harness.",
			Tags:        []string{"arch:runtime"},
			SourceRunID: "run_3",
			CreatedAt:   now,
		},
	}
	if err := manager.Commit(entries); err != nil {
		t.Fatalf("commit memories: %v", err)
	}

	recalled, err := manager.Recall(RecallQuery{
		SessionID: "session_1",
		Goal:      "harness arch",
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("recall memories: %v", err)
	}

	if len(recalled) != 2 {
		t.Fatalf("expected top 2 memories, got %d", len(recalled))
	}
	if recalled[0].ID != "mem_2" {
		t.Fatalf("expected session decision memory first, got %#v", recalled[0])
	}
	if recalled[1].ID != "mem_3" {
		t.Fatalf("expected workspace convention memory second, got %#v", recalled[1])
	}
}

func TestExtractCandidatesAndCommitCandidates(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)

	candidates := manager.ExtractCandidates(ExtractInput{
		SessionID: "session_1",
		RunID:     "run_1",
		Goal:      "summarize current harness status",
		Result:    "done",
		Provider:  "mock",
		Model:     "mock-model",
	})
	if len(candidates) == 0 {
		t.Fatal("expected extracted candidates")
	}

	entries, err := manager.CommitCandidates("session_1", candidates)
	if err != nil {
		t.Fatalf("commit candidates: %v", err)
	}
	if len(entries) != len(candidates) {
		t.Fatalf("expected %d committed entries, got %d", len(candidates), len(entries))
	}

	recalled, err := manager.Recall(RecallQuery{
		SessionID: "session_1",
		Goal:      "harness",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("recall committed candidates: %v", err)
	}
	if len(recalled) == 0 {
		t.Fatal("expected committed candidates to be recallable")
	}
}
