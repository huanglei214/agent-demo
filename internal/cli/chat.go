package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/app"
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
				func(input string) (chatLoopAction, error) {
					return handleChatCommand(services, sessionID, input, out)
				},
				func(input string) error {
					response, err := services.StartRun(app.RunRequest{
						Instruction: input,
						Workspace:   ctx.config.Workspace,
						Provider:    ctx.config.Model.Provider,
						Model:       ctx.config.Model.Model,
						MaxTurns:    maxTurns,
						SessionID:   sessionID,
					})
					if err != nil {
						return err
					}
					if response.Result != nil {
						fmt.Fprintf(out, "assistant> %s\n", response.Result.Output)
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

func runChatLoop(in io.Reader, out, errOut io.Writer, handleCommand func(string) (chatLoopAction, error), handleInput func(string) error, historyPath string) error {
	if rl, ok, err := newTTYReadline(in, out, historyPath); err != nil {
		return err
	} else if ok {
		defer rl.Close()
		for {
			input, err := readTTYChatInput(rl)
			if err != nil {
				if errors.Is(err, readline.ErrInterrupt) {
					fmt.Fprintln(out, "bye")
					return nil
				}
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
			switch input {
			case "":
				continue
			}
			if strings.HasPrefix(input, "/") {
				action, err := handleCommand(input)
				if err != nil {
					fmt.Fprintf(errOut, "error: %v\n", err)
					continue
				}
				if action == chatLoopExit {
					return nil
				}
				continue
			}
			rl.SaveHistory(input)
			if err := handleInput(input); err != nil {
				fmt.Fprintf(errOut, "error: %v\n", err)
			}
		}
		return nil
	}

	scanner := bufio.NewScanner(in)
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
			action, err := handleCommand(input)
			if err != nil {
				fmt.Fprintf(errOut, "error: %v\n", err)
				continue
			}
			if action == chatLoopExit {
				return nil
			}
			continue
		}
		if err := handleInput(input); err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
		}
	}
	return nil
}

func readTTYChatInput(rl *readline.Instance) (string, error) {
	var parts []string
	for {
		line, err := rl.Readline()
		if err != nil {
			return "", err
		}

		trimmedRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedRight, "\\") {
			parts = append(parts, strings.TrimSuffix(trimmedRight, "\\"))
			rl.SetPrompt("...> ")
			continue
		}

		parts = append(parts, line)
		rl.SetPrompt("you> ")
		return strings.TrimSpace(strings.Join(parts, "\n")), nil
	}
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

func newTTYReadline(in io.Reader, out io.Writer, historyPath string) (*readline.Instance, bool, error) {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return nil, false, nil
	}

	inInfo, err := inFile.Stat()
	if err != nil {
		return nil, false, err
	}
	outInfo, err := outFile.Stat()
	if err != nil {
		return nil, false, err
	}
	if inInfo.Mode()&os.ModeCharDevice == 0 || outInfo.Mode()&os.ModeCharDevice == 0 {
		return nil, false, nil
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "you> ",
		Stdin:           inFile,
		Stdout:          outFile,
		HistoryFile:     historyPath,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, false, err
	}
	return rl, true, nil
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
