package memory

import (
	"strings"
	"sync"
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

func TestDetectExplicitRememberAndRecallIdentity(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)

	candidates, answer, ok := manager.DetectExplicitRemember(ExplicitRememberInput{
		SessionID:   "session_1",
		RunID:       "run_1",
		Instruction: "我是黄磊，请记住",
	})
	if !ok {
		t.Fatal("expected explicit remember intent to be detected")
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one explicit memory candidate, got %d", len(candidates))
	}
	if !strings.Contains(answer, "黄磊") {
		t.Fatalf("expected answer to reference remembered value, got %q", answer)
	}

	entries, err := manager.CommitCandidates("session_1", candidates)
	if err != nil {
		t.Fatalf("commit explicit memory candidates: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one committed entry, got %d", len(entries))
	}

	recalled, err := manager.Recall(RecallQuery{
		SessionID: "session_1",
		Goal:      "我是谁",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("recall explicit identity memory: %v", err)
	}
	if len(recalled) == 0 {
		t.Fatal("expected explicit identity memory to be recallable")
	}
	if !strings.Contains(recalled[0].Content, "黄磊") {
		t.Fatalf("expected recalled identity memory to contain 黄磊, got %#v", recalled[0])
	}
}

func TestCommitPreservesEntriesAcrossConcurrentWriters(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	manager := NewManager(paths)

	entries := []harnessruntime.MemoryEntry{
		{
			ID:          "mem_a",
			SessionID:   "session_1",
			Scope:       "session",
			Kind:        "fact",
			Content:     "alpha",
			SourceRunID: "run_a",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "mem_b",
			SessionID:   "session_1",
			Scope:       "session",
			Kind:        "fact",
			Content:     "beta",
			SourceRunID: "run_b",
			CreatedAt:   time.Now(),
		},
	}

	var wg sync.WaitGroup
	for _, entry := range entries {
		entry := entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := manager.Commit([]harnessruntime.MemoryEntry{entry}); err != nil {
				t.Errorf("commit memory %s: %v", entry.ID, err)
			}
		}()
	}
	wg.Wait()

	recalled, err := manager.Recall(RecallQuery{
		SessionID: "session_1",
		Goal:      "alpha beta",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("recall concurrent commits: %v", err)
	}
	if len(recalled) != 2 {
		t.Fatalf("expected both concurrent entries to persist, got %#v", recalled)
	}
}
