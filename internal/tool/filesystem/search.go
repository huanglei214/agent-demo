package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type SearchTool struct {
	fs safeFS
}

const maxSearchMatches = 100

func NewSearchTool(workspace string) SearchTool {
	return SearchTool{fs: newSafeFS(workspace)}
}

func (t SearchTool) Name() string {
	return "fs.search"
}

func (t SearchTool) Description() string {
	return "Search workspace file paths or file contents by pattern or query, with capped results."
}

func (t SearchTool) AccessMode() tool.AccessMode {
	return tool.AccessReadOnly
}

func (t SearchTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	_ = ctx

	var req struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Query   string `json:"query"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		pattern = strings.TrimSpace(req.Query)
	}
	if pattern == "" {
		return tool.Result{}, errors.New("search pattern is required")
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

		if strings.Contains(relative, pattern) {
			matches = append(matches, map[string]any{
				"path":  relative,
				"match": "path",
			})
			if len(matches) >= maxSearchMatches {
				return filepath.SkipAll
			}
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(data), pattern) {
			matches = append(matches, map[string]any{
				"path":  relative,
				"match": "content",
			})
			if len(matches) >= maxSearchMatches {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":      filepath.Clean(req.Path),
			"pattern":   pattern,
			"matches":   matches,
			"truncated": len(matches) >= maxSearchMatches,
		},
	}, nil
}
