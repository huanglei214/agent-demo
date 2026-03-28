package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type safeFS struct {
	workspace string
}

func newSafeFS(workspace string) safeFS {
	return safeFS{workspace: workspace}
}

func (fs safeFS) resolve(path string) (string, error) {
	if path == "" {
		path = "."
	}

	cleaned := filepath.Clean(path)
	absolute := cleaned
	if !filepath.IsAbs(cleaned) {
		absolute = filepath.Join(fs.workspace, cleaned)
	}

	absolute = filepath.Clean(absolute)
	workspace := filepath.Clean(fs.workspace)
	if absolute != workspace && !strings.HasPrefix(absolute, workspace+string(os.PathSeparator)) {
		return "", fmt.Errorf("path is outside workspace: %s", path)
	}

	return fs.resolveSymlinksWithinWorkspace(absolute, workspace, path)
}

func (fs safeFS) resolveSymlinksWithinWorkspace(absolute, workspace, original string) (string, error) {
	relative, err := filepath.Rel(workspace, absolute)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return workspace, nil
	}

	current := workspace
	for _, segment := range strings.Split(relative, string(os.PathSeparator)) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return "", err
		}
		current = filepath.Clean(resolved)
		if current != workspace && !strings.HasPrefix(current, workspace+string(os.PathSeparator)) {
			return "", fmt.Errorf("path resolves outside workspace: %s", original)
		}
	}

	return current, nil
}
