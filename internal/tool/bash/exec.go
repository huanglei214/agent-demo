package bash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/tool"
)

const (
	defaultTimeoutSeconds = 30
	maxCommandOutputRunes = 4000
)

var dangerousExecutables = map[string]struct{}{
	"dd":        {},
	"halt":      {},
	"mkfs":      {},
	"mkfs.ext4": {},
	"mkfs.xfs":  {},
	"poweroff":  {},
	"reboot":    {},
	"rm":        {},
	"shutdown":  {},
}

type ExecTool struct {
	workspace string
}

type TimeoutError struct {
	Command        string
	Workdir        string
	TimeoutSeconds int
}

type DangerousCommandError struct {
	Command    string
	Executable string
	Segment    string
}

func (e DangerousCommandError) Error() string {
	return "dangerous command is blocked"
}

func (e DangerousCommandError) Details() map[string]any {
	return map[string]any{
		"command":    e.Command,
		"executable": e.Executable,
		"segment":    e.Segment,
		"blocked":    true,
	}
}

func (e TimeoutError) Error() string {
	return "command timed out"
}

func (e TimeoutError) Details() map[string]any {
	return map[string]any{
		"command":         e.Command,
		"workdir":         e.Workdir,
		"timeout_seconds": e.TimeoutSeconds,
		"timed_out":       true,
	}
}

func NewExecTool(workspace string) ExecTool {
	return ExecTool{workspace: workspace}
}

func (t ExecTool) Name() string {
	return "bash.exec"
}

func (t ExecTool) Description() string {
	return "Execute a shell command inside the workspace and return structured output."
}

func (t ExecTool) AccessMode() tool.AccessMode {
	return tool.AccessExec
}

func (t ExecTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var req struct {
		Command        string `json:"command"`
		Workdir        string `json:"workdir"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		Timeout        int    `json:"timeout"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	command := strings.TrimSpace(req.Command)
	if command == "" {
		return tool.Result{}, errors.New("command is required")
	}
	if err := rejectDangerousCommand(command); err != nil {
		return tool.Result{}, err
	}
	workdir, err := resolveInsideWorkspace(t.workspace, req.Workdir)
	if err != nil {
		return tool.Result{}, err
	}

	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = req.Timeout
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-lc", command)
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return tool.Result{}, TimeoutError{
			Command:        command,
			Workdir:        filepath.Clean(req.Workdir),
			TimeoutSeconds: timeoutSeconds,
		}
	}

	stdoutText, stdoutTruncated := truncateRunes(stdout.String(), maxCommandOutputRunes)
	stderrText, stderrTruncated := truncateRunes(stderr.String(), maxCommandOutputRunes)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return tool.Result{}, err
		}
	}

	return tool.Result{
		Content: map[string]any{
			"command":   command,
			"workdir":   filepath.Clean(req.Workdir),
			"exit_code": exitCode,
			"stdout":    stdoutText,
			"stderr":    stderrText,
			"truncated": stdoutTruncated || stderrTruncated,
			"timed_out": false,
		},
	}, nil
}

func resolveInsideWorkspace(workspace, path string) (string, error) {
	if path == "" {
		path = "."
	}
	cleaned := filepath.Clean(path)
	absolute := cleaned
	if !filepath.IsAbs(cleaned) {
		absolute = filepath.Join(workspace, cleaned)
	}
	absolute = filepath.Clean(absolute)
	root := filepath.Clean(workspace)
	if absolute != root && !strings.HasPrefix(absolute, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path is outside workspace: %s", path)
	}
	return absolute, nil
}

func truncateRunes(input string, maxRunes int) (string, bool) {
	runes := []rune(input)
	if len(runes) <= maxRunes {
		return input, false
	}
	return string(runes[:maxRunes]), true
}

func rejectDangerousCommand(command string) error {
	for _, segment := range splitCommandSegments(command) {
		executable := firstExecutable(segment)
		if executable == "" {
			continue
		}
		if _, blocked := dangerousExecutables[executable]; blocked {
			return DangerousCommandError{
				Command:    command,
				Executable: executable,
				Segment:    strings.TrimSpace(segment),
			}
		}
	}
	return nil
}

func splitCommandSegments(command string) []string {
	segments := make([]string, 0, 4)
	start := 0
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false

	for idx := 0; idx < len(command); idx++ {
		ch := command[idx]
		if escaped {
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			if !inSingleQuote {
				escaped = true
			}
		case '\'':
			if !inDoubleQuote && !inBacktick {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote && !inBacktick {
				inDoubleQuote = !inDoubleQuote
			}
		case '`':
			if !inSingleQuote && !inDoubleQuote {
				inBacktick = !inBacktick
			}
		case '&':
			if !inSingleQuote && !inDoubleQuote && !inBacktick && idx+1 < len(command) && command[idx+1] == '&' {
				segments = append(segments, command[start:idx])
				idx++
				start = idx + 1
			}
		case '|':
			if !inSingleQuote && !inDoubleQuote && !inBacktick {
				segments = append(segments, command[start:idx])
				if idx+1 < len(command) && command[idx+1] == '|' {
					idx++
				}
				start = idx + 1
			}
		case ';', '\n':
			if !inSingleQuote && !inDoubleQuote && !inBacktick {
				segments = append(segments, command[start:idx])
				start = idx + 1
			}
		}
	}

	return append(segments, command[start:])
}

func firstExecutable(segment string) string {
	fields := strings.Fields(strings.TrimSpace(segment))
	for _, field := range fields {
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "/") && !strings.HasPrefix(field, ".") {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) == 2 && parts[0] != "" {
				continue
			}
		}
		switch field {
		case "sudo", "env", "command":
			continue
		}
		return filepath.Base(field)
	}
	return ""
}
