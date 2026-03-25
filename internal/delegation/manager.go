package delegation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

type Policy struct {
	MaxDepth     int
	MaxChildren  int
	AllowedTools []string
}

type Option func(*Policy)

type Manager struct {
	paths  store.Paths
	policy Policy
}

type ChildRecord struct {
	Task      harnessruntime.DelegationTask   `json:"task"`
	Run       harnessruntime.Run              `json:"run"`
	Result    harnessruntime.DelegationResult `json:"result"`
	UpdatedAt time.Time                       `json:"updated_at"`
}

func NewManager(paths store.Paths, options ...Option) Manager {
	policy := Policy{
		MaxDepth:     2,
		MaxChildren:  2,
		AllowedTools: []string{"fs.read_file", "fs.list_dir", "fs.search", "fs.stat"},
	}
	for _, option := range options {
		option(&policy)
	}

	return Manager{
		paths:  paths,
		policy: policy,
	}
}

func WithAllowedTools(names []string) Option {
	copied := append([]string{}, names...)
	return func(policy *Policy) {
		policy.AllowedTools = copied
	}
}

func (m Manager) CanDelegate(_ context.Context, run harnessruntime.Run, step harnessruntime.PlanStep) (bool, string) {
	if !step.Delegatable {
		return false, "step_not_delegatable"
	}
	currentDepth, err := m.depth(run)
	if err != nil {
		return false, "depth_lookup_failed"
	}
	if currentDepth >= m.policy.MaxDepth {
		return false, "max_depth_exceeded"
	}
	activeChildren, err := m.activeChildren(run.ID)
	if err == nil && activeChildren >= m.policy.MaxChildren {
		return false, "max_children_exceeded"
	}
	return true, ""
}

func (m Manager) BuildTask(parentRun harnessruntime.Run, plan harnessruntime.Plan, step harnessruntime.PlanStep, goal string, memories []harnessruntime.MemoryEntry, summaries []harnessruntime.Summary) harnessruntime.DelegationTask {
	memorySnippets := make([]string, 0, len(memories))
	for _, entry := range memories {
		memorySnippets = append(memorySnippets, entry.Content)
	}
	summarySnippets := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		summarySnippets = append(summarySnippets, summary.Content)
	}

	return harnessruntime.DelegationTask{
		ID:            harnessruntime.NewID("delegation"),
		ParentRunID:   parentRun.ID,
		SessionID:     parentRun.SessionID,
		PlanStepID:    step.ID,
		Goal:          strings.TrimSpace(goal),
		AllowedTools:  append([]string{}, m.policy.AllowedTools...),
		ParentGoal:    plan.Goal,
		StepTitle:     step.Title,
		StepDesc:      step.Description,
		Constraints:   []string{"child run must return structured summary", "child run cannot write long-term memory directly"},
		ContextMemory: append(memorySnippets, summarySnippets...),
		CreatedAt:     time.Now(),
	}
}

func (m Manager) ListChildren(parentRunID string) ([]ChildRecord, error) {
	dir := m.paths.ChildrenDir(parentRunID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChildRecord{}, nil
		}
		return nil, err
	}

	result := make([]ChildRecord, 0, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var record ChildRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func (m Manager) SaveChild(parentRunID string, record ChildRecord) error {
	path := m.paths.ChildPath(parentRunID, record.Run.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (m Manager) ValidateTools(run harnessruntime.Run, toolName string) error {
	if run.ParentRunID == "" {
		return nil
	}
	for _, allowed := range m.policy.AllowedTools {
		if allowed == toolName {
			return nil
		}
	}
	return fmt.Errorf("tool %s is not allowed for child run", toolName)
}

func (m Manager) activeChildren(parentRunID string) (int, error) {
	children, err := m.ListChildren(parentRunID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, child := range children {
		switch child.Run.Status {
		case harnessruntime.RunPending, harnessruntime.RunRunning, harnessruntime.RunBlocked:
			count++
		}
	}
	return count, nil
}

func (m Manager) depth(run harnessruntime.Run) (int, error) {
	depth := 0
	current := run
	for current.ParentRunID != "" {
		parent, err := m.loadRun(current.ParentRunID)
		if err != nil {
			if os.IsNotExist(err) {
				return depth + 1, nil
			}
			return 0, err
		}
		depth++
		current = parent
	}
	return depth, nil
}

func (m Manager) loadRun(runID string) (harnessruntime.Run, error) {
	path := m.paths.RunPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return harnessruntime.Run{}, err
	}
	var run harnessruntime.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return harnessruntime.Run{}, err
	}
	return run, nil
}
