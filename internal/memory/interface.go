package memory

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type Service interface {
	Recall(query RecallQuery) ([]harnessruntime.MemoryEntry, error)
	DetectExplicitRemember(input ExplicitRememberInput) ([]harnessruntime.MemoryCandidate, string, bool)
	ExtractCandidates(input ExtractInput) []harnessruntime.MemoryCandidate
	CommitCandidates(sessionID string, candidates []harnessruntime.MemoryCandidate) ([]harnessruntime.MemoryEntry, error)
}
