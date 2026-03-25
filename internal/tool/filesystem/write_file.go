package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type WriteFileTool struct {
	fs safeFS
}

func NewWriteFileTool(workspace string) WriteFileTool {
	return WriteFileTool{fs: newSafeFS(workspace)}
}

func (t WriteFileTool) Name() string {
	return "fs.write_file"
}

func (t WriteFileTool) Description() string {
	return "Write a text file inside the workspace with overwrite protection."
}

func (t WriteFileTool) AccessMode() tool.AccessMode {
	return tool.AccessWrite
}

func (t WriteFileTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	_ = ctx

	var req struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	path, err := t.fs.resolve(req.Path)
	if err != nil {
		return tool.Result{}, err
	}

	mode := "created"
	if _, err := os.Stat(path); err == nil {
		if !req.Overwrite {
			return tool.Result{}, os.ErrExist
		}
		mode = "updated"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return tool.Result{}, err
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":       filepath.Clean(req.Path),
			"bytes":      len(req.Content),
			"write_mode": mode,
		},
	}, nil
}
