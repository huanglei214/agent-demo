package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type SearchTool struct {
	fs safeFS
}

func NewSearchTool(workspace string) SearchTool {
	return SearchTool{fs: newSafeFS(workspace)}
}

func (t SearchTool) Name() string {
	return "fs.search"
}

func (t SearchTool) Description() string {
	return "Search file paths or file contents inside the workspace."
}

func (t SearchTool) AccessMode() tool.AccessMode {
	return tool.AccessReadOnly
}

func (t SearchTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	_ = ctx

	var req struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	root, err := t.fs.resolve(req.Path)
	if err != nil {
		return tool.Result{}, err
	}

	matches := make([]map[string]any, 0)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(t.fs.workspace, path)
		if err != nil {
			return err
		}

		if strings.Contains(relative, req.Pattern) {
			matches = append(matches, map[string]any{
				"path":  relative,
				"match": "path",
			})
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(data), req.Pattern) {
			matches = append(matches, map[string]any{
				"path":  relative,
				"match": "content",
			})
		}
		return nil
	})
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":    filepath.Clean(req.Path),
			"pattern": req.Pattern,
			"matches": matches,
		},
	}, nil
}
