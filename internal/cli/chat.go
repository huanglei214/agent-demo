package cli

import (
	"bufio"
	"fmt"
	"strings"

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

			if strings.TrimSpace(sessionID) == "" {
				session, err := services.CreateSession(ctx.config.Workspace)
				if err != nil {
					return err
				}
				sessionID = session.ID
				fmt.Fprintf(cmd.OutOrStdout(), "session_id: %s\n", sessionID)
			} else {
				session, err := services.LoadSession(sessionID)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "session_id: %s (continued)\n", session.ID)
			}

			scanner := bufio.NewScanner(cmd.InOrStdin())
			for {
				fmt.Fprintf(cmd.OutOrStdout(), "you> ")
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						return err
					}
					break
				}

				input := strings.TrimSpace(scanner.Text())
				switch input {
				case "":
					continue
				case "/exit", "/quit":
					fmt.Fprintln(cmd.OutOrStdout(), "bye")
					return nil
				}

				response, err := services.StartRun(app.RunRequest{
					Instruction: input,
					Workspace:   ctx.config.Workspace,
					Provider:    ctx.config.Model.Provider,
					Model:       ctx.config.Model.Model,
					MaxTurns:    maxTurns,
					SessionID:   sessionID,
				})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
					continue
				}
				if response.Result != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "assistant> %s\n", response.Result.Output)
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&maxTurns, "max-turns", 20, "Maximum model turns for each chat round")
	cmd.Flags().StringVar(&sessionID, "session", "", "Continue an existing session")
	return cmd
}
