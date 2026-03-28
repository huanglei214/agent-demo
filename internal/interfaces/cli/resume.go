package cli

import "github.com/spf13/cobra"

func newResumeCommand(ctx *commandContext) *cobra.Command {
	return buildResumeCommand(ctx, "resume <run-id>", "Resume an unfinished run from persisted artifacts")
}

func buildResumeCommand(ctx *commandContext, use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.servicesFor(cmd).ResumeRun(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), response)
		},
	}
}
