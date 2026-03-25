package store

import (
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
	return filepath.Join(p.RunsDir(), runID)
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

func (p Paths) SummariesPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "summaries.json")
}

func (p Paths) MemoriesPath() string {
	return filepath.Join(p.root, "memories.json")
}

func (p Paths) RunMemoriesPath(runID string) string {
	return filepath.Join(p.RunDir(runID), "memories.json")
}

func (p Paths) ChildrenDir(runID string) string {
	return filepath.Join(p.RunDir(runID), "children")
}

func (p Paths) ChildPath(parentRunID, childRunID string) string {
	return filepath.Join(p.ChildrenDir(parentRunID), childRunID+".json")
}
