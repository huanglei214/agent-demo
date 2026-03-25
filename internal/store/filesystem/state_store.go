package filesystem

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

type StateStore struct {
	paths store.Paths
}

func NewStateStore(paths store.Paths) StateStore {
	return StateStore{paths: paths}
}

func (s StateStore) SaveTask(task harnessruntime.Task) error {
	return writeJSON(s.paths.TaskPath(task.ID), task)
}

func (s StateStore) SaveSession(session harnessruntime.Session) error {
	return writeJSON(s.paths.SessionPath(session.ID), session)
}

func (s StateStore) SaveRun(run harnessruntime.Run) error {
	return writeJSON(s.paths.RunPath(run.ID), run)
}

func (s StateStore) SaveState(state harnessruntime.RunState) error {
	return writeJSON(s.paths.StatePath(state.RunID), state)
}

func (s StateStore) SavePlan(plan harnessruntime.Plan) error {
	return writeJSON(s.paths.PlanPath(plan.RunID), plan)
}

func (s StateStore) SaveResult(result harnessruntime.RunResult) error {
	return writeJSON(s.paths.ResultPath(result.RunID), result)
}

func (s StateStore) SaveSummaries(runID string, summaries []harnessruntime.Summary) error {
	return writeJSON(s.paths.SummariesPath(runID), summaries)
}

func (s StateStore) SaveRunMemories(memories harnessruntime.RunMemories) error {
	return writeJSON(s.paths.RunMemoriesPath(memories.RunID), memories)
}

func (s StateStore) AppendSessionMessage(message harnessruntime.SessionMessage) error {
	path := s.paths.SessionMessagesPath(message.SessionID)
	return appendJSONL(path, message)
}

func (s StateStore) LoadRun(runID string) (harnessruntime.Run, error) {
	var run harnessruntime.Run
	err := readJSON(s.paths.RunPath(runID), &run)
	return run, err
}

func (s StateStore) LoadTask(taskID string) (harnessruntime.Task, error) {
	var task harnessruntime.Task
	err := readJSON(s.paths.TaskPath(taskID), &task)
	return task, err
}

func (s StateStore) LoadSession(sessionID string) (harnessruntime.Session, error) {
	var session harnessruntime.Session
	err := readJSON(s.paths.SessionPath(sessionID), &session)
	return session, err
}

func (s StateStore) LoadSessionMessages(sessionID string) ([]harnessruntime.SessionMessage, error) {
	path := s.paths.SessionMessagesPath(sessionID)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	messages := make([]harnessruntime.SessionMessage, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var message harnessruntime.SessionMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, scanner.Err()
}

func (s StateStore) LoadRecentSessionMessages(sessionID string, limit int) ([]harnessruntime.SessionMessage, error) {
	messages, err := s.LoadSessionMessages(sessionID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || len(messages) <= limit {
		return messages, nil
	}
	return messages[len(messages)-limit:], nil
}

func (s StateStore) LoadState(runID string) (harnessruntime.RunState, error) {
	var state harnessruntime.RunState
	err := readJSON(s.paths.StatePath(runID), &state)
	return state, err
}

func (s StateStore) LoadPlan(runID string) (harnessruntime.Plan, error) {
	var plan harnessruntime.Plan
	err := readJSON(s.paths.PlanPath(runID), &plan)
	return plan, err
}

func (s StateStore) LoadResult(runID string) (harnessruntime.RunResult, error) {
	var result harnessruntime.RunResult
	err := readJSON(s.paths.ResultPath(runID), &result)
	return result, err
}

func (s StateStore) LoadSummaries(runID string) ([]harnessruntime.Summary, error) {
	var summaries []harnessruntime.Summary
	err := readJSON(s.paths.SummariesPath(runID), &summaries)
	return summaries, err
}

func (s StateStore) LoadRunMemories(runID string) (harnessruntime.RunMemories, error) {
	var memories harnessruntime.RunMemories
	err := readJSON(s.paths.RunMemoriesPath(runID), &memories)
	return memories, err
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func appendJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}

	return file.Sync()
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, out)
}
