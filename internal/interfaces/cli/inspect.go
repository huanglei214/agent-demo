package cli

import "github.com/spf13/cobra"

func newInspectCommand(ctx *commandContext) *cobra.Command {
	return buildInspectCommand(ctx, "inspect <run-id>", "Inspect run state, current step, failures, and child summaries")
}

func buildInspectCommand(ctx *commandContext, use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.servicesFor(cmd).InspectRun(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), response)
		},
	}
}
