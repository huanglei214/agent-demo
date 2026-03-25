package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type StatTool struct {
	fs safeFS
}

func NewStatTool(workspace string) StatTool {
	return StatTool{fs: newSafeFS(workspace)}
}

func (t StatTool) Name() string {
	return "fs.stat"
}

func (t StatTool) Description() string {
	return "Inspect file or directory metadata inside the workspace."
}

func (t StatTool) AccessMode() tool.AccessMode {
	return tool.AccessReadOnly
}

func (t StatTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
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

	info, err := os.Stat(path)
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":        filepath.Clean(req.Path),
			"name":        info.Name(),
			"is_dir":      info.IsDir(),
			"size":        info.Size(),
			"mode":        info.Mode().String(),
			"modified_at": info.ModTime(),
		},
	}, nil
}
