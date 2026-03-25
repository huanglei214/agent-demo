package cli

import "github.com/spf13/cobra"

func newSessionCommand(ctx *commandContext) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Inspect persisted session state",
	}

	var recentLimit int
	inspectCmd := &cobra.Command{
		Use:   "inspect <session-id>",
		Short: "Inspect session messages and related runs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.services().InspectSession(args[0], recentLimit)
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), response)
		},
	}
	inspectCmd.Flags().IntVar(&recentLimit, "recent", 10, "Number of recent session messages to include")
	sessionCmd.AddCommand(inspectCmd)

	return sessionCmd
}
