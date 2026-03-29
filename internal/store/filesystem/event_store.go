package filesystem

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

const maxJSONLScanTokenSize = 1024 * 1024

type EventStore struct {
	paths store.Paths
}

func NewEventStore(paths store.Paths) EventStore {
	return EventStore{paths: paths}
}

func (s EventStore) Append(event harnessruntime.Event) error {
	path := s.paths.EventsPath(event.RunID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}

	return file.Sync()
}

func (s EventStore) ReadAll(runID string) ([]harnessruntime.Event, error) {
	path := s.paths.EventsPath(runID)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]harnessruntime.Event, 0)
	scanner := newJSONLScanner(file)
	for scanner.Scan() {
		var event harnessruntime.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

func (s EventStore) NextSequence(runID string) (int64, error) {
	path := s.paths.EventsPath(runID)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	defer file.Close()

	var count int64
	scanner := newJSONLScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count + 1, nil
}

func newJSONLScanner(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxJSONLScanTokenSize)
	return scanner
}
