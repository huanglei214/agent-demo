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

type StrReplaceTool struct {
	fs safeFS
}

func NewStrReplaceTool(workspace string) StrReplaceTool {
	return StrReplaceTool{fs: newSafeFS(workspace)}
}

func (t StrReplaceTool) Name() string {
	return "fs.str_replace"
}

func (t StrReplaceTool) Description() string {
	return "Replace an exact text snippet inside a workspace file."
}

func (t StrReplaceTool) AccessMode() tool.AccessMode {
	return tool.AccessWrite
}

func (t StrReplaceTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	_ = ctx

	var req struct {
		Path       string `json:"path"`
		OldStr     string `json:"old_str"`
		NewStr     string `json:"new_str"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}
	if req.OldStr == "" {
		return tool.Result{}, errors.New("old_str is required")
	}

	path, err := t.fs.resolve(req.Path)
	if err != nil {
		return tool.Result{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return tool.Result{}, err
	}

	content := string(data)
	if !strings.Contains(content, req.OldStr) {
		return tool.Result{}, errors.New("old_str was not found in target file")
	}

	replaced := 1
	updated := strings.Replace(content, req.OldStr, req.NewStr, 1)
	if req.ReplaceAll {
		replaced = strings.Count(content, req.OldStr)
		updated = strings.ReplaceAll(content, req.OldStr, req.NewStr)
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"path":        filepath.Clean(req.Path),
			"replaced":    replaced,
			"replace_all": req.ReplaceAll,
			"bytes":       len(updated),
		},
	}, nil
}
