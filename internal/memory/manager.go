package memory

import (
	"sort"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/store"
)

type RecallQuery struct {
	SessionID string
	Goal      string
	Tags      []string
	Kinds     []string
	Limit     int
}

type ExtractInput struct {
	SessionID string
	RunID     string
	Goal      string
	Result    string
	Provider  string
	Model     string
}

type Manager struct {
	store FileStore
}

func NewManager(paths store.Paths) Manager {
	return Manager{store: NewFileStore(paths)}
}

func (m Manager) Recall(query RecallQuery) ([]harnessruntime.MemoryEntry, error) {
	entries, err := m.store.Load()
	if err != nil {
		return nil, err
	}

	filtered := make([]harnessruntime.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if !matchesSession(query.SessionID, entry) {
			continue
		}
		if !matchesKinds(query.Kinds, entry.Kind) {
			continue
		}
		if !matchesTags(query.Tags, entry.Tags) {
			continue
		}
		if !matchesGoal(query.Goal, entry) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		if scopeWeight(left.Scope) != scopeWeight(right.Scope) {
			return scopeWeight(left.Scope) > scopeWeight(right.Scope)
		}
		if kindWeight(left.Kind) != kindWeight(right.Kind) {
			return kindWeight(left.Kind) > kindWeight(right.Kind)
		}
		return left.CreatedAt.After(right.CreatedAt)
	})

	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	return filtered, nil
}

func (m Manager) Commit(entries []harnessruntime.MemoryEntry) error {
	existing, err := m.store.Load()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if containsMemory(existing, entry) {
			continue
		}
		existing = append(existing, entry)
	}
	return m.store.Save(existing)
}

func (m Manager) ExtractCandidates(input ExtractInput) []harnessruntime.MemoryCandidate {
	candidates := make([]harnessruntime.MemoryCandidate, 0, 2)
	now := time.Now()

	goal := strings.TrimSpace(input.Goal)
	if goal != "" {
		candidates = append(candidates, harnessruntime.MemoryCandidate{
			Kind:        "fact",
			Scope:       "workspace",
			Content:     "Successful run goal: " + goal,
			Tags:        []string{"runtime:goal"},
			SourceRunID: input.RunID,
			CreatedAt:   now,
		})
	}

	if provider := strings.TrimSpace(input.Provider); provider != "" {
		candidates = append(candidates, harnessruntime.MemoryCandidate{
			Kind:        "preference",
			Scope:       "session",
			Content:     "Preferred provider for recent successful run: " + provider,
			Tags:        []string{"runtime:provider", "provider:" + provider},
			SourceRunID: input.RunID,
			CreatedAt:   now,
		})
	}

	return candidates
}

func (m Manager) CommitCandidates(sessionID string, candidates []harnessruntime.MemoryCandidate) ([]harnessruntime.MemoryEntry, error) {
	entries := make([]harnessruntime.MemoryEntry, 0, len(candidates))
	for _, candidate := range candidates {
		entries = append(entries, harnessruntime.MemoryEntry{
			ID:          harnessruntime.NewID("mem"),
			SessionID:   sessionID,
			Scope:       candidate.Scope,
			Kind:        candidate.Kind,
			Content:     candidate.Content,
			Tags:        candidate.Tags,
			SourceRunID: candidate.SourceRunID,
			CreatedAt:   candidate.CreatedAt,
		})
	}

	if err := m.Commit(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func matchesSession(sessionID string, entry harnessruntime.MemoryEntry) bool {
	if entry.Scope != "session" {
		return true
	}
	if sessionID == "" {
		return false
	}
	return entry.SessionID == sessionID
}

func matchesKinds(kinds []string, kind string) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, item := range kinds {
		if item == kind {
			return true
		}
	}
	return false
}

func matchesTags(required, actual []string) bool {
	if len(required) == 0 {
		return true
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		actualSet[strings.ToLower(item)] = struct{}{}
	}
	for _, item := range required {
		if _, ok := actualSet[strings.ToLower(item)]; ok {
			return true
		}
	}
	return false
}

func matchesGoal(goal string, entry harnessruntime.MemoryEntry) bool {
	goal = strings.TrimSpace(strings.ToLower(goal))
	if goal == "" {
		return true
	}

	haystack := strings.ToLower(entry.Content + " " + strings.Join(entry.Tags, " "))
	for _, token := range strings.Fields(goal) {
		if len(token) < 3 {
			continue
		}
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func scopeWeight(scope string) int {
	switch scope {
	case "session":
		return 3
	case "workspace":
		return 2
	case "global":
		return 1
	default:
		return 0
	}
}

func kindWeight(kind string) int {
	switch kind {
	case "decision":
		return 4
	case "convention":
		return 3
	case "fact":
		return 2
	case "preference":
		return 1
	default:
		return 0
	}
}

func containsMemory(existing []harnessruntime.MemoryEntry, candidate harnessruntime.MemoryEntry) bool {
	for _, entry := range existing {
		if entry.Scope == candidate.Scope && entry.Kind == candidate.Kind && entry.Content == candidate.Content {
			return true
		}
	}
	return false
}
