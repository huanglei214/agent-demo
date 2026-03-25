package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/huanglei214/agent-demo/internal/tool"
)

func TestReadFileToolSuccess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(target, []byte("hello harness"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	result, err := NewReadFileTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path": "notes.txt",
	}))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if result.Content["content"] != "hello harness" {
		t.Fatalf("unexpected content: %#v", result.Content)
	}
}

func TestListDirToolSuccess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}

	result, err := NewListDirTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path": ".",
	}))
	if err != nil {
		t.Fatalf("list dir: %v", err)
	}

	entries, ok := result.Content["entries"].([]map[string]any)
	if ok {
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		return
	}

	rawEntries, ok := result.Content["entries"].([]any)
	if !ok || len(rawEntries) != 2 {
		t.Fatalf("unexpected entries payload: %#v", result.Content["entries"])
	}
}

func TestSearchToolSuccess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "alpha.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "beta.md"), []byte("nothing here"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	result, err := NewSearchTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path":    ".",
		"pattern": "hello",
	}))
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if matches, ok := result.Content["matches"].([]map[string]any); ok {
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		return
	}

	rawMatches, ok := result.Content["matches"].([]any)
	if !ok || len(rawMatches) != 1 {
		t.Fatalf("unexpected matches: %#v", result.Content["matches"])
	}
}

func TestStatToolSuccess(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "meta.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	result, err := NewStatTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path": "meta.txt",
	}))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if result.Content["name"] != "meta.txt" {
		t.Fatalf("unexpected stat payload: %#v", result.Content)
	}
}

func TestWriteFileToolCreateAndUpdate(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	tool := NewWriteFileTool(workspace)

	created, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":      "out/result.txt",
		"content":   "first",
		"overwrite": false,
	}))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if created.Content["write_mode"] != "created" {
		t.Fatalf("unexpected create mode: %#v", created.Content)
	}

	updated, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":      "out/result.txt",
		"content":   "second",
		"overwrite": true,
	}))
	if err != nil {
		t.Fatalf("update file: %v", err)
	}
	if updated.Content["write_mode"] != "updated" {
		t.Fatalf("unexpected update mode: %#v", updated.Content)
	}
}

func TestWriteFileToolRejectsOverwriteWithoutFlag(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "exists.txt")
	if err := os.WriteFile(target, []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := NewWriteFileTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path":      "exists.txt",
		"content":   "updated",
		"overwrite": false,
	}))
	if !os.IsExist(err) {
		t.Fatalf("expected os.ErrExist, got %v", err)
	}
}

func TestFileSystemToolAccessModes(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	if got := NewReadFileTool(workspace).AccessMode(); got != tool.AccessReadOnly {
		t.Fatalf("expected read tool to be read_only, got %s", got)
	}
	if got := NewListDirTool(workspace).AccessMode(); got != tool.AccessReadOnly {
		t.Fatalf("expected list tool to be read_only, got %s", got)
	}
	if got := NewSearchTool(workspace).AccessMode(); got != tool.AccessReadOnly {
		t.Fatalf("expected search tool to be read_only, got %s", got)
	}
	if got := NewStatTool(workspace).AccessMode(); got != tool.AccessReadOnly {
		t.Fatalf("expected stat tool to be read_only, got %s", got)
	}
	if got := NewWriteFileTool(workspace).AccessMode(); got != tool.AccessWrite {
		t.Fatalf("expected write tool to be write, got %s", got)
	}
}

func TestFileSystemToolsRejectOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}

	_, err := NewReadFileTool(workspace).Execute(context.Background(), mustJSON(t, map[string]any{
		"path": "../outside.txt",
	}))
	if err == nil {
		t.Fatal("expected boundary error, got nil")
	}
}

func mustJSON(t *testing.T, value map[string]any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test input: %v", err)
	}
	return data
}
