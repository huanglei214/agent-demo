package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterh/liner"
	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/app"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func newChatCommand(ctx *commandContext) *cobra.Command {
	var (
		maxTurns  int
		sessionID string
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start or continue an interactive multi-turn chat session",
		RunE: func(cmd *cobra.Command, args []string) error {
			services := ctx.services()
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			if strings.TrimSpace(sessionID) == "" {
				session, err := services.CreateSession(ctx.config.Workspace)
				if err != nil {
					return err
				}
				sessionID = session.ID
				fmt.Fprintf(out, "session_id: %s\n", sessionID)
			} else {
				session, err := services.LoadSession(sessionID)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "session_id: %s (continued)\n", session.ID)
			}

			return runChatLoop(
				cmd.InOrStdin(),
				out,
				errOut,
				func(input string, output io.Writer) (chatLoopAction, error) {
					return handleChatCommand(services, sessionID, input, output)
				},
				func(input string, output, errorOutput io.Writer) error {
					response, err := services.StartRun(app.RunRequest{
						Instruction: input,
						Workspace:   ctx.config.Workspace,
						Provider:    ctx.config.Model.Provider,
						Model:       ctx.config.Model.Model,
						MaxTurns:    maxTurns,
						SessionID:   sessionID,
					})
					if err != nil {
						if rendered, renderErr := renderLatestChatFailure(services, sessionID); renderErr == nil && rendered != "" {
							fmt.Fprintln(errorOutput, rendered)
							return nil
						}
						return err
					}
					if response.Result != nil {
						fmt.Fprintf(output, "assistant> %s\n", response.Result.Output)
					}
					return nil
				},
				ctx.services().Paths.SessionInputHistoryPath(sessionID),
			)
		},
	}

	cmd.Flags().IntVar(&maxTurns, "max-turns", 20, "Maximum model turns for each chat round")
	cmd.Flags().StringVar(&sessionID, "session", "", "Continue an existing session")
	return cmd
}

type chatLoopAction int

const (
	chatLoopContinue chatLoopAction = iota
	chatLoopExit
)

const (
	defaultScannerBufferSize = 64 * 1024
	maxScannerTokenSize      = 1024 * 1024
)

func runChatLoop(
	in io.Reader,
	out, errOut io.Writer,
	handleCommand func(string, io.Writer) (chatLoopAction, error),
	handleInput func(string, io.Writer, io.Writer) error,
	historyPath string,
) error {
	if _, _, ok, err := ttyInputFiles(in, out); err != nil {
		return err
	} else if ok {
		line := liner.NewLiner()
		defer line.Close()

		line.SetCtrlCAborts(true)
		line.SetTabCompletionStyle(liner.TabCircular)
		line.SetMultiLineMode(true)

		if historyPath != "" {
			if f, err := os.Open(historyPath); err == nil {
				line.ReadHistory(f)
				f.Close()
			}
		}

		for {
			input, ok, err := readLinerChatInput(line)
			if err != nil {
				if errors.Is(err, liner.ErrPromptAborted) {
					return nil
				}
				return err
			}
			if !ok {
				break
			}
			switch input {
			case "":
				continue
			}
			if strings.HasPrefix(input, "/") {
				action, err := handleCommand(input, out)
				if err != nil {
					fmt.Fprintf(out, "error: %v\n", err)
					continue
				}
				if action == chatLoopExit {
					return nil
				}
				continue
			}

			line.AppendHistory(input)
			if historyPath != "" {
				if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err == nil {
					if f, err := os.Create(historyPath); err == nil {
						line.WriteHistory(f)
						f.Close()
					}
				}
			}

			if err := handleInput(input, out, out); err != nil {
				fmt.Fprintf(out, "error: %v\n", err)
			}
		}
		return nil
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, defaultScannerBufferSize), maxScannerTokenSize)
	for {
		input, ok, err := readScannerChatInput(scanner, out)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		switch input {
		case "":
			continue
		}
		if strings.HasPrefix(input, "/") {
			action, err := handleCommand(input, out)
			if err != nil {
				fmt.Fprintf(errOut, "error: %v\n", err)
				continue
			}
			if action == chatLoopExit {
				return nil
			}
			continue
		}
		if err := handleInput(input, out, errOut); err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
		}
	}
	return nil
}

func readScannerChatInput(scanner *bufio.Scanner, out io.Writer) (string, bool, error) {
	var parts []string
	prompt := "you> "
	for {
		fmt.Fprint(out, prompt)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", false, err
			}
			if len(parts) == 0 {
				return "", false, nil
			}
			return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
		}

		line := scanner.Text()
		trimmedRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedRight, "\\") {
			parts = append(parts, strings.TrimSuffix(trimmedRight, "\\"))
			prompt = "...> "
			continue
		}

		parts = append(parts, line)
		return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
	}
}

func readReaderChatInput(reader *bufio.Reader, out io.Writer) (string, bool, error) {
	var parts []string
	prompt := "you> "
	for {
		fmt.Fprint(out, prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(parts) == 0 && strings.TrimSpace(line) == "" {
					return "", false, nil
				}
				if strings.TrimSpace(line) != "" {
					parts = append(parts, strings.TrimRight(line, "\r\n"))
				}
				return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
			}
			return "", false, err
		}

		line = strings.TrimRight(line, "\r\n")
		trimmedRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedRight, "\\") {
			parts = append(parts, strings.TrimSuffix(trimmedRight, "\\"))
			prompt = "...> "
			continue
		}

		parts = append(parts, line)
		return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
	}
}

func readLinerChatInput(line *liner.State) (string, bool, error) {
	var parts []string
	prompt := "you> "
	for {
		input, err := line.Prompt(prompt)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(parts) == 0 && strings.TrimSpace(input) == "" {
					return "", false, nil
				}
				if strings.TrimSpace(input) != "" {
					parts = append(parts, strings.TrimRight(input, "\r\n"))
				}
				return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
			}
			return "", false, err
		}

		trimmedRight := strings.TrimRight(input, " \t")
		if strings.HasSuffix(trimmedRight, "\\") {
			parts = append(parts, strings.TrimSuffix(trimmedRight, "\\"))
			prompt = "...> "
			continue
		}

		parts = append(parts, input)
		return strings.TrimSpace(strings.Join(parts, "\n")), true, nil
	}
}

func ttyInputFiles(in io.Reader, out io.Writer) (*os.File, *os.File, bool, error) {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return nil, nil, false, nil
	}

	inInfo, err := inFile.Stat()
	if err != nil {
		return nil, nil, false, err
	}
	outInfo, err := outFile.Stat()
	if err != nil {
		return nil, nil, false, err
	}
	if inInfo.Mode()&os.ModeCharDevice == 0 || outInfo.Mode()&os.ModeCharDevice == 0 {
		return nil, nil, false, nil
	}
	return inFile, outFile, true, nil
}

func handleChatCommand(services app.Services, sessionID, input string, out io.Writer) (chatLoopAction, error) {
	command := strings.TrimSpace(input)
	switch {
	case command == "/exit" || command == "/quit":
		fmt.Fprintln(out, "bye")
		return chatLoopExit, nil
	case command == "/session":
		fmt.Fprintf(out, "session_id: %s\n", sessionID)
		return chatLoopContinue, nil
	case command == "/clear":
		fmt.Fprint(out, "\033[H\033[2J")
		return chatLoopContinue, nil
	case strings.HasPrefix(command, "/history"):
		limit := 10
		fields := strings.Fields(command)
		if len(fields) > 1 {
			value, err := strconv.Atoi(fields[1])
			if err != nil || value <= 0 {
				return chatLoopContinue, fmt.Errorf("invalid history limit: %s", fields[1])
			}
			limit = value
		}
		messages, err := services.StateStore.LoadRecentSessionMessages(sessionID, limit)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(out, "no history")
				return chatLoopContinue, nil
			}
			return chatLoopContinue, err
		}
		if len(messages) == 0 {
			fmt.Fprintln(out, "no history")
			return chatLoopContinue, nil
		}
		for _, message := range messages {
			label := "assistant"
			if message.Role == "user" {
				label = "user"
			}
			fmt.Fprintf(out, "%s> %s\n", label, message.Content)
		}
		return chatLoopContinue, nil
	default:
		return chatLoopContinue, fmt.Errorf("unknown command: %s", command)
	}
}

func renderLatestChatFailure(services app.Services, sessionID string) (string, error) {
	sessionInfo, err := services.InspectSession(sessionID, 1)
	if err != nil {
		return "", err
	}
	if len(sessionInfo.Runs) == 0 {
		return "", nil
	}

	inspect, err := services.InspectRun(sessionInfo.Runs[0].RunID)
	if err != nil {
		return "", err
	}
	if inspect.RecentFailure == nil {
		return "", nil
	}
	return formatChatFailure(*inspect.RecentFailure), nil
}

func formatChatFailure(event harnessruntime.Event) string {
	switch event.Type {
	case "tool.failed":
		toolName := payloadString(event.Payload, "tool")
		errMsg := payloadString(event.Payload, "error")
		if details := payloadMap(event.Payload, "details"); payloadBool(details, "timed_out") {
			timeout := payloadInt(details, "timeout_seconds")
			workdir := payloadString(details, "workdir")
			if timeout > 0 && workdir != "" {
				return fmt.Sprintf("tool failed: %s timed out after %ds (workdir=%s)", fallbackToolName(toolName), timeout, workdir)
			}
			if timeout > 0 {
				return fmt.Sprintf("tool failed: %s timed out after %ds", fallbackToolName(toolName), timeout)
			}
			return fmt.Sprintf("tool failed: %s timed out", fallbackToolName(toolName))
		}
		if toolName != "" && errMsg != "" {
			return fmt.Sprintf("tool failed: %s: %s", toolName, errMsg)
		}
		if errMsg != "" {
			return fmt.Sprintf("tool failed: %s", errMsg)
		}
	case "run.failed":
		if errMsg := payloadString(event.Payload, "error"); errMsg != "" {
			return fmt.Sprintf("run failed: %s", errMsg)
		}
	case "subagent.rejected":
		if errMsg := payloadString(event.Payload, "reason"); errMsg != "" {
			return fmt.Sprintf("subagent rejected: %s", errMsg)
		}
	}
	return "error: run failed"
}

func fallbackToolName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "tool"
	}
	return name
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func payloadMap(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok {
		return nil
	}
	details, _ := value.(map[string]any)
	return details
}

func payloadBool(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	parsed, _ := value.(bool)
	return parsed
}

func payloadInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
