package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type ListDirTool struct {
	fs safeFS
}

func NewListDirTool(workspace string) ListDirTool {
	return ListDirTool{fs: newSafeFS(workspace)}
}

func (t ListDirTool) Name() string {
	return "fs.list_dir"
}

func (t ListDirTool) Description() string {
	return "List direct entries for a directory inside the workspace."
}

func (t ListDirTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
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

	entries, err := os.ReadDir(path)
	if err != nil {
		return tool.Result{}, err
	}

	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return tool.Result{}, err
		}
		items = append(items, map[string]any{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
			"size":  info.Size(),
			"path":  filepath.Join(filepath.Clean(req.Path), entry.Name()),
		})
	}

	return tool.Result{
		Content: map[string]any{
			"path":    filepath.Clean(req.Path),
			"entries": items,
		},
	}, nil
}
