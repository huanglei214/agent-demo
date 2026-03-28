package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/service"
)

func newRunCommand(ctx *commandContext) *cobra.Command {
	return buildRunCommand(ctx, "run <instruction>", "Create a local harness run")
}

func buildRunCommand(ctx *commandContext, use, short string) *cobra.Command {
	var maxTurns int
	var sessionID string
	var skillName string

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			services := ctx.servicesFor(cmd)
			response, err := services.StartRun(service.RunRequest{
				Instruction: strings.Join(args, " "),
				Workspace:   services.Config.Workspace,
				Provider:    services.Config.Model.Provider,
				Model:       services.Config.Model.Model,
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
