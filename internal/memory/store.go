package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

const (
	lockRetryInterval = 10 * time.Millisecond
	lockWaitTimeout   = 2 * time.Second
	staleLockTimeout  = lockWaitTimeout
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

func (s FileStore) LoadRelevant(sessionID string) ([]harnessruntime.MemoryEntry, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return s.Load()
	}

	shared, sharedErr := s.loadShard(s.sharedPath())
	sessionEntries, sessionErr := s.loadShard(s.sessionPath(sessionID))
	switch {
	case sharedErr == nil || sessionErr == nil:
		return append(shared, sessionEntries...), nil
	case errors.Is(sharedErr, os.ErrNotExist) && errors.Is(sessionErr, os.ErrNotExist):
		return s.Load()
	case sharedErr != nil && !errors.Is(sharedErr, os.ErrNotExist):
		return nil, sharedErr
	default:
		return nil, sessionErr
	}
}

func (s FileStore) Save(entries []harnessruntime.MemoryEntry) error {
	return s.withLock(func() error {
		return s.write(entries)
	})
}

func (s FileStore) Update(update func([]harnessruntime.MemoryEntry) ([]harnessruntime.MemoryEntry, error)) error {
	return s.withLock(func() error {
		existing, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		next, err := update(existing)
		if err != nil {
			return err
		}
		return s.write(next)
	})
}

func (s FileStore) loadUnlocked() ([]harnessruntime.MemoryEntry, error) {
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

func (s FileStore) write(entries []harnessruntime.MemoryEntry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(s.path), "memories-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(append(data, '\n')); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	return s.writeShards(entries)
}

func (s FileStore) writeShards(entries []harnessruntime.MemoryEntry) error {
	if err := os.MkdirAll(s.shardDir(), 0o755); err != nil {
		return err
	}
	sessionEntries := make(map[string][]harnessruntime.MemoryEntry)
	sharedEntries := make([]harnessruntime.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Scope == "session" && strings.TrimSpace(entry.SessionID) != "" {
			sessionEntries[entry.SessionID] = append(sessionEntries[entry.SessionID], entry)
			continue
		}
		sharedEntries = append(sharedEntries, entry)
	}
	if err := writeJSONFile(s.sharedPath(), sharedEntries); err != nil {
		return err
	}
	sessionDir := filepath.Dir(s.sessionPath("placeholder"))
	if err := os.RemoveAll(sessionDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return err
	}
	for sessionID, items := range sessionEntries {
		if err := writeJSONFile(s.sessionPath(sessionID), items); err != nil {
			return err
		}
	}
	return nil
}

func (s FileStore) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lockPath := s.path + ".lock"
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			lockFile.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if removed, err := removeStaleLock(lockPath, time.Now()); err != nil {
			return err
		} else if removed {
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out acquiring memory store lock")
		}
		time.Sleep(lockRetryInterval)
	}
}

func removeStaleLock(lockPath string, now time.Time) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if now.Sub(info.ModTime()) < staleLockTimeout {
		return false, nil
	}
	if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func (s FileStore) shardDir() string {
	return filepath.Dir(s.sharedPath())
}

func (s FileStore) sharedPath() string {
	root := filepath.Dir(s.path)
	return filepath.Join(root, "memories.d", "shared.json")
}

func (s FileStore) sessionPath(sessionID string) string {
	root := filepath.Dir(s.path)
	return filepath.Join(root, "memories.d", "sessions", sessionID+".json")
}

func (s FileStore) loadShard(path string) ([]harnessruntime.MemoryEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []harnessruntime.MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
