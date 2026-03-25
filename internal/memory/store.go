package memory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

type FileStore struct {
	path string
}

func NewFileStore(paths store.Paths) FileStore {
	return FileStore{path: paths.MemoriesPath()}
}

func (s FileStore) Load() ([]harnessruntime.MemoryEntry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []harnessruntime.MemoryEntry{}, nil
		}
		return nil, err
	}

	var entries []harnessruntime.MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s FileStore) Save(entries []harnessruntime.MemoryEntry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, append(data, '\n'), 0o644)
}
