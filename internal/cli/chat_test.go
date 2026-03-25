package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/huanglei214/agent-demo/internal/app"
	"github.com/huanglei214/agent-demo/internal/config"
)

func TestChatCommandStartsSessionAndReplies(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()

	cmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("你好\n/exit\n"))
	cmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "chat"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute chat command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "session_id:") || !strings.Contains(output, "assistant>") {
		t.Fatalf("unexpected chat output:\n%s", output)
	}
}

func TestRunCommandSupportsSessionFlag(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	services := app.NewServices(config.Load(workspace))
	session, err := services.CreateSession(workspace)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "run", "--session", session.ID, "继续聊"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute run command: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode run output: %v\noutput=%s", err, out.String())
	}

	messages, err := services.StateStore.LoadSessionMessages(session.ID)
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected one round worth of messages, got %#v", messages)
	}
}
