package delegation

import (
	"testing"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestBuildResultUnwrapsFinalWrapper(t *testing.T) {
	result, err := BuildResult(
		harnessruntime.Run{ID: "child-run"},
		&harnessruntime.RunResult{
			Output: `{"action":"final","answer":"{\"summary\":\"天气查询完成\",\"artifacts\":[],\"findings\":[],\"risks\":[],\"recommendations\":[],\"needs_replan\":false}"}`,
		},
	)
	if err != nil {
		t.Fatalf("BuildResult: %v", err)
	}
	if result.ChildRunID != "child-run" {
		t.Fatalf("expected child run id to be preserved, got %#v", result)
	}
	if result.Summary != "天气查询完成" {
		t.Fatalf("expected structured summary, got %#v", result)
	}
}
