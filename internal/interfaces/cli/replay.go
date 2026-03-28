package cli

import "github.com/spf13/cobra"

func newReplayCommand(ctx *commandContext) *cobra.Command {
	return buildReplayCommand(ctx, "replay <run-id>", "Replay persisted events for a run as summarized timeline entries")
}

func buildReplayCommand(ctx *commandContext, use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := ctx.servicesFor(cmd).ReplayRunSummary(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), events)
		},
	}
}
