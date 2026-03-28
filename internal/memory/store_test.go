package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

func TestFileStoreSaveRecoversStaleLock(t *testing.T) {
	t.Parallel()

	paths := store.NewPaths(t.TempDir())
	fileStore := NewFileStore(paths)
	lockPath := paths.MemoriesPath() + ".lock"

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	staleTime := time.Now().Add(-staleLockTimeout - time.Second)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("mark stale lock: %v", err)
	}

	entries := []harnessruntime.MemoryEntry{{
		ID:          "mem_1",
		SessionID:   "session_1",
		Scope:       "session",
		Kind:        "fact",
		Content:     "alpha",
		SourceRunID: "run_1",
		CreatedAt:   time.Now(),
	}}
	if err := fileStore.Save(entries); err != nil {
		t.Fatalf("save with stale lock: %v", err)
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale lock to be removed, got err=%v", err)
	}

	loaded, err := fileStore.Load()
	if err != nil {
		t.Fatalf("load after stale lock recovery: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "mem_1" {
		t.Fatalf("unexpected loaded entries: %#v", loaded)
	}
}
