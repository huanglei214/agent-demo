package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type ReadFileTool struct {
	fs safeFS
}

func NewReadFileTool(workspace string) ReadFileTool {
	return ReadFileTool{fs: newSafeFS(workspace)}
}

func (t ReadFileTool) Name() string {
	return "fs.read_file"
}

func (t ReadFileTool) Description() string {
	return "Read a UTF-8 text file inside the workspace."
}

func (t ReadFileTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	_ = ctx

	var req struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	path, err := t.fs.resolve(req.Path)
	if err != nil {
		return tool.Result{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":    filepath.Clean(req.Path),
			"content": string(data),
			"bytes":   len(data),
		},
	}, nil
}
