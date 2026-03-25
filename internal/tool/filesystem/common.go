package filesystem

import (
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

	return absolute, nil
}
