package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/huanglei214/agent-demo/internal/app"
	"github.com/huanglei214/agent-demo/internal/config"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
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
		func(input string, output io.Writer) (chatLoopAction, error) {
			return handleChatCommand(app.Services{}, "", input, output)
		},
		func(input string, _, _ io.Writer) error {
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

func TestRunChatLoopSupportsLargeScannerInput(t *testing.T) {
	var (
		out    bytes.Buffer
		errOut bytes.Buffer
		got    []string
	)

	largeInput := strings.Repeat("武汉天气怎么样", 8000)
	err := runChatLoop(
		strings.NewReader(largeInput+"\n/exit\n"),
		&out,
		&errOut,
		func(input string, output io.Writer) (chatLoopAction, error) {
			return handleChatCommand(app.Services{}, "", input, output)
		},
		func(input string, _, _ io.Writer) error {
			got = append(got, input)
			return nil
		},
		"",
	)
	if err != nil {
		t.Fatalf("run chat loop with large input: %v", err)
	}
	if !reflect.DeepEqual(got, []string{largeInput}) {
		t.Fatalf("unexpected submitted inputs: %#v", got)
	}
}

func TestFormatChatFailureRendersToolTimeoutDetails(t *testing.T) {
	event := harnessruntime.Event{
		Type: "tool.failed",
		Payload: map[string]any{
			"tool":  "bash.exec",
			"error": "command timed out",
			"details": map[string]any{
				"timed_out":       true,
				"timeout_seconds": 1.0,
				"workdir":         ".",
			},
		},
	}

	got := formatChatFailure(event)
	want := "tool failed: bash.exec timed out after 1s (workdir=.)"
	if got != want {
		t.Fatalf("unexpected formatted failure:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestFormatChatFailureRendersRunFailure(t *testing.T) {
	event := harnessruntime.Event{
		Type: "run.failed",
		Payload: map[string]any{
			"error": "ark timeout error: request timed out",
		},
	}

	got := formatChatFailure(event)
	want := "run failed: ark timeout error: request timed out"
	if got != want {
		t.Fatalf("unexpected formatted run failure:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestReadReaderChatInput(t *testing.T) {
	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader("你好\n"))
	input, ok, err := readReaderChatInput(reader, &out)
	if err != nil {
		t.Fatalf("read reader chat input: %v", err)
	}
	if !ok {
		t.Fatalf("expected input to be available")
	}
	if input != "你好" {
		t.Fatalf("unexpected input: %s", input)
	}
	if got := out.String(); got != "you> " {
		t.Fatalf("unexpected prompt output: %q", got)
	}
}

func TestReadReaderChatInputSupportsMultiline(t *testing.T) {
	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader("第一行\\\n第二行\n"))
	input, ok, err := readReaderChatInput(reader, &out)
	if err != nil {
		t.Fatalf("read reader multiline input: %v", err)
	}
	if !ok {
		t.Fatalf("expected multiline input to be available")
	}
	if input != "第一行\n第二行" {
		t.Fatalf("unexpected multiline input: %q", input)
	}
	if got := out.String(); got != "you> ...> " {
		t.Fatalf("unexpected prompt output: %q", got)
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

func TestSessionInspectCommandShowsMessagesAndRuns(t *testing.T) {
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
	cmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "session", "inspect", session.ID, "--recent", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute session inspect: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"messages"`) || !strings.Contains(output, `"runs"`) {
		t.Fatalf("expected session inspect output to include messages and runs, got:\n%s", output)
	}
	if !strings.Contains(output, session.ID) {
		t.Fatalf("expected session id in output, got:\n%s", output)
	}
}

func TestRootCommandDoesNotExposeServeSubcommand(t *testing.T) {
	cmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}

	for _, child := range cmd.Commands() {
		if child.Name() == "serve" {
			t.Fatalf("unexpected serve subcommand in root command")
		}
	}
}

func TestInspectCommandIncludesCurrentStepAndChildRuns(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	services := app.NewServices(config.Load(workspace))
	response, err := services.StartRun(app.RunRequest{
		Instruction: "请委派一个子任务来分析这个仓库，然后给我总结",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start delegated run: %v", err)
	}

	cmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "inspect", response.Run.ID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"current_step"`) || !strings.Contains(output, `"child_runs"`) || !strings.Contains(output, `"model_calls"`) {
		t.Fatalf("expected inspect output to include current_step and child_runs, got:\n%s", output)
	}
}

func TestReplayCommandReturnsSummariesWhileDebugEventsRemainRaw(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	services := app.NewServices(config.Load(workspace))
	response, err := services.StartRun(app.RunRequest{
		Instruction: "请读取 README.md 并总结当前项目状态",
		Workspace:   workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	replayCmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}
	var replayOut bytes.Buffer
	replayCmd.SetOut(&replayOut)
	replayCmd.SetErr(&replayOut)
	replayCmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "replay", response.Run.ID})
	if err := replayCmd.Execute(); err != nil {
		t.Fatalf("execute replay command: %v", err)
	}

	debugCmd, err := NewRootCommand()
	if err != nil {
		t.Fatalf("new root command: %v", err)
	}
	var debugOut bytes.Buffer
	debugCmd.SetOut(&debugOut)
	debugCmd.SetErr(&debugOut)
	debugCmd.SetArgs([]string{"--workspace", workspace, "--provider", "mock", "debug", "events", response.Run.ID})
	if err := debugCmd.Execute(); err != nil {
		t.Fatalf("execute debug events command: %v", err)
	}

	if !strings.Contains(replayOut.String(), `"summary"`) {
		t.Fatalf("expected replay output to include summary field, got:\n%s", replayOut.String())
	}
	if strings.Contains(replayOut.String(), `"payload"`) {
		t.Fatalf("expected replay output to omit raw payload field, got:\n%s", replayOut.String())
	}
	if !strings.Contains(debugOut.String(), `"payload"`) || !strings.Contains(debugOut.String(), `"sequence"`) {
		t.Fatalf("expected debug events output to remain raw, got:\n%s", debugOut.String())
	}
}
