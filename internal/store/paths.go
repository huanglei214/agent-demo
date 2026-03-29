package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Paths struct {
	root string
}

func NewPaths(root string) Paths {
	return Paths{root: root}
}

func (p Paths) Root() string {
	return p.root
}

func (p Paths) RunsDir() string {
	return filepath.Join(p.root, "runs")
}

func (p Paths) RunDir(runID string) string {
	topLevel := filepath.Join(p.RunsDir(), runID)
	if _, err := os.Stat(topLevel); err == nil {
		return topLevel
	}
	data, err := os.ReadFile(p.ChildRunIndexPath(runID))
	if err != nil {
		return topLevel
	}
	var record childRunIndex
	if err := json.Unmarshal(data, &record); err != nil || record.ParentRunID == "" {
		return topLevel
	}
	return p.ChildRunDir(record.ParentRunID, runID)
}

func (p Paths) TasksDir() string {
	return filepath.Join(p.root, "tasks")
}

func (p Paths) TaskPath(taskID string) string {
	return filepath.Join(p.TasksDir(), taskID+".json")
}

func (p Paths) SessionsDir() string {
	return filepath.Join(p.root, "sessions")
}

func (p Paths) SessionDir(sessionID string) string {
	return filepath.Join(p.SessionsDir(), sessionID)
}

func (p Paths) SessionPath(sessionID string) string {
	return filepath.Join(p.SessionDir(sessionID), "session.json")
}

func (p Paths) SessionMessagesPath(sessionID string) string {
	return filepath.Join(p.SessionDir(sessionID), "messages.jsonl")
}

func (p Paths) SessionRunsDir(sessionID string) string {
	return filepath.Join(p.SessionDir(sessionID), "runs")
}

func (p Paths) SessionRunPath(sessionID, runID string) string {
	return filepath.Join(p.SessionRunsDir(sessionID), runID+".json")
}

func (p Paths) SessionInputHistoryPath(sessionID string) string {
	return filepath.Join(p.SessionDir(sessionID), "input.history")
}

func (p Paths) RunPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "run.json")
}

func (p Paths) StatePath(runID string) string {
	return filepath.Join(p.RunDir(runID), "state.json")
}

func (p Paths) PlanPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "plan.json")
}

func (p Paths) ResultPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "result.json")
}

func (p Paths) EventsPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "events.jsonl")
}

func (p Paths) ModelCallsPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "model_calls.jsonl")
}

func (p Paths) SummariesPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "summaries.json")
}

func (p Paths) MemoriesPath() string {
	return filepath.Join(p.root, "memories.json")
}

func (p Paths) MemoriesShardDir() string {
	return filepath.Join(p.root, "memories.d")
}

func (p Paths) SharedMemoriesPath() string {
	return filepath.Join(p.MemoriesShardDir(), "shared.json")
}

func (p Paths) SessionMemoriesPath(sessionID string) string {
	return filepath.Join(p.MemoriesShardDir(), "sessions", sessionID+".json")
}

func (p Paths) RunMemoriesPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "memories.json")
}

func (p Paths) ChildrenDir(runID string) string {
	return filepath.Join(p.RunDir(runID), "children")
}

func (p Paths) ChildRunDir(parentRunID, childRunID string) string {
	return filepath.Join(p.ChildrenDir(parentRunID), childRunID)
}

func (p Paths) ChildPath(parentRunID, childRunID string) string {
	return filepath.Join(p.ChildrenDir(parentRunID), childRunID+".json")
}

func (p Paths) ChildRunIndexDir() string {
	return filepath.Join(p.RunsDir(), ".children")
}

func (p Paths) ChildRunIndexPath(childRunID string) string {
	return filepath.Join(p.ChildRunIndexDir(), childRunID+".json")
}

type childRunIndex struct {
	ParentRunID string `json:"parent_run_id"`
}
