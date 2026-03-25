package cli

import "github.com/spf13/cobra"

func newDebugCommand(ctx *commandContext) *cobra.Command {
	debugCmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug helpers for runtime artifacts",
	}

	debugCmd.AddCommand(&cobra.Command{
		Use:   "events <run-id>",
		Short: "Print raw events for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := ctx.services().ReplayRun(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), events)
		},
	})

	return debugCmd
}
