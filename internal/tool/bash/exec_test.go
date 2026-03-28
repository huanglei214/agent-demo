package bash

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecToolRejectsDangerousCommand(t *testing.T) {
	t.Parallel()

	_, err := NewExecTool(t.TempDir()).Execute(context.Background(), mustExecJSON(t, map[string]any{
		"command": "rm -rf tmp",
	}))
	if err == nil || !strings.Contains(err.Error(), "dangerous command is blocked") {
		t.Fatalf("expected dangerous command error, got %v", err)
	}
}

func TestExecToolRejectsDangerousCommandInChain(t *testing.T) {
	t.Parallel()

	_, err := NewExecTool(t.TempDir()).Execute(context.Background(), mustExecJSON(t, map[string]any{
		"command": "echo ok && shutdown -h now",
	}))
	if err == nil || !strings.Contains(err.Error(), "dangerous command is blocked") {
		t.Fatalf("expected chained dangerous command error, got %v", err)
	}
}

func TestExecToolAllowsQuotedDangerousText(t *testing.T) {
	t.Parallel()

	result, err := NewExecTool(t.TempDir()).Execute(context.Background(), mustExecJSON(t, map[string]any{
		"command": "printf '%s' 'ok && rm -rf /'",
	}))
	if err != nil {
		t.Fatalf("expected quoted dangerous text to be allowed, got %v", err)
	}
	if got := result.Content["stdout"]; got != "ok && rm -rf /" {
		t.Fatalf("unexpected stdout: %#v", got)
	}
}

func mustExecJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
