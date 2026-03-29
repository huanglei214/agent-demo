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

type ExplicitRememberInput struct {
	SessionID   string
	RunID       string
	Instruction string
}

type Manager struct {
	store FileStore
}

func NewManager(paths store.Paths) Manager {
	return Manager{store: NewFileStore(paths)}
}

func (m Manager) Recall(query RecallQuery) ([]harnessruntime.MemoryEntry, error) {
	entries, err := m.store.LoadRelevant(query.SessionID)
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
	return m.store.Update(func(existing []harnessruntime.MemoryEntry) ([]harnessruntime.MemoryEntry, error) {
		for _, entry := range entries {
			if containsMemory(existing, entry) {
				continue
			}
			existing = append(existing, entry)
		}
		return existing, nil
	})
}

func (m Manager) DetectExplicitRemember(input ExplicitRememberInput) ([]harnessruntime.MemoryCandidate, string, bool) {
	statement, ok := normalizeRememberStatement(input.Instruction)
	if !ok {
		return nil, "", false
	}

	now := time.Now()
	candidate := harnessruntime.MemoryCandidate{
		Kind:        "fact",
		Scope:       "session",
		Content:     "用户要求记住：" + statement,
		Tags:        []string{"memory:explicit"},
		SourceRunID: input.RunID,
		CreatedAt:   now,
	}
	answer := "好的，我会把这条信息记在当前会话的 Memory 里。"

	switch {
	case strings.HasPrefix(statement, "我是"):
		value := strings.TrimSpace(strings.TrimPrefix(statement, "我是"))
		if value == "" {
			break
		}
		candidate.Content = "用户身份是" + value
		candidate.Tags = []string{"memory:explicit", "user:identity"}
		if looksLikeName(value) {
			candidate.Tags = append(candidate.Tags, "user:name")
		}
		answer = "好的，我会记住你是" + value + "。"
	case strings.HasPrefix(statement, "我叫"):
		value := strings.TrimSpace(strings.TrimPrefix(statement, "我叫"))
		if value == "" {
			break
		}
		candidate.Content = "用户名字是" + value
		candidate.Tags = []string{"memory:explicit", "user:identity", "user:name"}
		answer = "好的，我会记住你叫" + value + "。"
	case strings.HasPrefix(statement, "我的名字是"):
		value := strings.TrimSpace(strings.TrimPrefix(statement, "我的名字是"))
		if value == "" {
			break
		}
		candidate.Content = "用户名字是" + value
		candidate.Tags = []string{"memory:explicit", "user:identity", "user:name"}
		answer = "好的，我会记住你的名字是" + value + "。"
	case strings.HasPrefix(statement, "你是"):
		value := strings.TrimSpace(strings.TrimPrefix(statement, "你是"))
		if value == "" {
			break
		}
		candidate.Content = "本会话中助手身份设定为" + value
		candidate.Kind = "preference"
		candidate.Tags = []string{"memory:explicit", "assistant:identity"}
		answer = "好的，我会记住我是" + value + "。"
	}

	return []harnessruntime.MemoryCandidate{candidate}, answer, true
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

	if matchesIdentityGoal(goal, entry.Tags) {
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

func matchesIdentityGoal(goal string, tags []string) bool {
	compactGoal := strings.ReplaceAll(goal, " ", "")
	switch {
	case strings.Contains(compactGoal, "我是谁"),
		strings.Contains(compactGoal, "我叫什么"),
		strings.Contains(compactGoal, "我的名字"):
		return hasAnyTag(tags, "user:identity", "user:name")
	case strings.Contains(compactGoal, "你是谁"):
		return hasAnyTag(tags, "assistant:identity")
	default:
		return false
	}
}

func hasAnyTag(tags []string, expected ...string) bool {
	if len(tags) == 0 {
		return false
	}
	actual := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		actual[strings.ToLower(tag)] = struct{}{}
	}
	for _, item := range expected {
		if _, ok := actual[strings.ToLower(item)]; ok {
			return true
		}
	}
	return false
}

func normalizeRememberStatement(instruction string) (string, bool) {
	statement := strings.TrimSpace(instruction)
	if statement == "" {
		return "", false
	}

	suffixPhrases := []string{
		"请记住", "记住", "请记下", "记下来", "记一下", "帮我记住", "麻烦记住",
	}
	for _, phrase := range suffixPhrases {
		if idx := strings.Index(statement, phrase); idx >= 0 {
			prefix := strings.TrimSpace(statement[:idx])
			if prefix != "" {
				return trimStatementNoise(prefix), true
			}
		}
	}

	prefixPhrases := []string{
		"记住", "请记住", "请记下", "记下来", "记一下",
	}
	for _, phrase := range prefixPhrases {
		if strings.HasPrefix(statement, phrase) {
			remainder := strings.TrimSpace(strings.TrimPrefix(statement, phrase))
			if remainder != "" {
				return trimStatementNoise(remainder), true
			}
		}
	}

	return "", false
}

func trimStatementNoise(statement string) string {
	statement = strings.TrimSpace(statement)
	statement = strings.Trim(statement, "，,。.!！？?；;：: ")
	return statement
}

func looksLikeName(value string) bool {
	length := len([]rune(strings.TrimSpace(value)))
	return length > 0 && length <= 8
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
