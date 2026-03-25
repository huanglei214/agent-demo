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

func TestRunChatLoopSupportsMultilineInput(t *testing.T) {
	var (
		out    bytes.Buffer
		errOut bytes.Buffer
		got    []string
	)

	err := runChatLoop(
		strings.NewReader("第一行\\\n第二行\n/exit\n"),
		&out,
		&errOut,
		func(input string) (chatLoopAction, error) {
			return handleChatCommand(app.Services{}, "", input, &out)
		},
		func(input string) error {
			got = append(got, input)
			return nil
		},
		"",
	)
	if err != nil {
		t.Fatalf("run chat loop: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one submitted input, got %#v", got)
	}
	if got[0] != "第一行\n第二行" {
		t.Fatalf("unexpected multiline payload: %#v", got[0])
	}
	if !strings.Contains(out.String(), "...> ") {
		t.Fatalf("expected continuation prompt in output, got:\n%s", out.String())
	}
}

func TestChatCommandSupportsSessionHistoryAndHelpers(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	services := app.NewServices(config.Load(workspace))

	session, err := services.CreateSession(workspace)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := services.StartRun(app.RunRequest{
		Instruction: "你好",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   session.ID,
	}); err != nil {
		t.Fatalf("seed session run: %v", err)
	}

	cmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("/session\n/history 2\n/clear\n/exit\n"))
	cmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "chat", "--session", session.ID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute chat helper command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "session_id: "+session.ID) {
		t.Fatalf("expected session command output, got:\n%s", output)
	}
	if !strings.Contains(output, "user> 你好") || !strings.Contains(output, "assistant>") {
		t.Fatalf("expected history output, got:\n%s", output)
	}
	if !strings.Contains(output, "\033[H\033[2J") {
		t.Fatalf("expected clear screen escape sequence, got:\n%s", output)
	}
}
