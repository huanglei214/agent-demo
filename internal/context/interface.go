package context

import harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"

type Service interface {
	Build(input BuildInput) ModelContext
	ShouldCompact(input CompactionCheckInput) (bool, string)
	Compact(input CompactInput) (harnessruntime.Summary, error)
}
