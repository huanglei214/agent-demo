package bash

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

func TestExecToolReturnsStructuredOutputOnSuccess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	result, err := NewExecTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"command":         "printf 'hello'",
		"workdir":         ".",
		"timeout_seconds": 5,
	}))
	if err != nil {
		t.Fatalf("exec success: %v", err)
	}

	if result.Content["exit_code"] != 0 {
		t.Fatalf("unexpected exit code: %#v", result.Content)
	}
	if result.Content["stdout"] != "hello" {
		t.Fatalf("unexpected stdout: %#v", result.Content)
	}
}

func TestExecToolReturnsStructuredOutputOnNonZeroExit(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	result, err := NewExecTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"command":         "printf 'bad' >&2; exit 7",
		"workdir":         ".",
		"timeout_seconds": 5,
	}))
	if err != nil {
		t.Fatalf("exec non-zero: %v", err)
	}

	if result.Content["exit_code"] != 7 {
		t.Fatalf("unexpected exit code: %#v", result.Content)
	}
	if result.Content["stderr"] != "bad" {
		t.Fatalf("unexpected stderr: %#v", result.Content)
	}
}

func TestExecToolTimesOut(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := NewExecTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"command":         "sleep 2",
		"workdir":         ".",
		"timeout_seconds": 1,
	}))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	detailedErr, ok := err.(toolruntime.DetailedError)
	if !ok {
		t.Fatalf("expected timeout error to implement DetailedError, got %T", err)
	}
	if detailedErr.Details()["timed_out"] != true {
		t.Fatalf("expected timed_out details, got %#v", detailedErr.Details())
	}
}

func TestExecToolSupportsTimeoutAlias(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := NewExecTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"command": "sleep 2",
		"workdir": ".",
		"timeout": 1,
	}))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error via alias, got %v", err)
	}
}

func TestExecToolRejectsOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := NewExecTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"command": "pwd",
		"workdir": "../outside",
	}))
	if err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
