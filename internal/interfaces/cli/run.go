package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/app"
)

func newRunCommand(ctx *commandContext) *cobra.Command {
	var maxTurns int
	var sessionID string
	var skillName string

	cmd := &cobra.Command{
		Use:   "run <instruction>",
		Short: "Create a local harness run",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.services().StartRun(app.RunRequest{
				Instruction: strings.Join(args, " "),
				Workspace:   ctx.config.Workspace,
				Provider:    ctx.config.Model.Provider,
				Model:       ctx.config.Model.Model,
				MaxTurns:    maxTurns,
				SessionID:   sessionID,
				Skill:       skillName,
			})
			if err != nil {
				return err
			}

			if err := printJSONTo(cmd.OutOrStdout(), response); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&maxTurns, "max-turns", 20, "Maximum model turns for the run")
	cmd.Flags().StringVar(&sessionID, "session", "", "Append this instruction to an existing session")
	cmd.Flags().StringVar(&skillName, "skill", "", "Activate a named skill for this run")
	return cmd
}
