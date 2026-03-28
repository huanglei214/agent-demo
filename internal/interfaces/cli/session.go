package cli

import "github.com/spf13/cobra"

func newSessionCommand(ctx *commandContext) *cobra.Command {
	return buildLegacySessionCommand(ctx)
}

func buildLegacySessionCommand(ctx *commandContext) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Inspect persisted session state",
	}

	sessionCmd.AddCommand(buildSessionInspectCommand(ctx, "inspect <session-id>", "Inspect session messages and related runs"))
	return sessionCmd
}

func buildSessionInspectCommand(ctx *commandContext, use, short string) *cobra.Command {
	var recentLimit int
	inspectCmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.servicesFor(cmd).InspectSession(args[0], recentLimit)
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), response)
		},
	}
	inspectCmd.Flags().IntVar(&recentLimit, "recent", 10, "Number of recent session messages to include")
	return inspectCmd
}
