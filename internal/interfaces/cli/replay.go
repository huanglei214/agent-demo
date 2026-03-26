package cli

import "github.com/spf13/cobra"

func newReplayCommand(ctx *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:   "replay <run-id>",
		Short: "Replay persisted events for a run as summarized timeline entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := ctx.services().ReplayRunSummary(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), events)
		},
	}
}
