package cli

import "github.com/spf13/cobra"

func newDebugCommand(ctx *commandContext) *cobra.Command {
	debugCmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug and inspection helpers for runtime artifacts",
	}

	debugCmd.AddCommand(&cobra.Command{
		Use:   "events <run-id>",
		Short: "Print raw events for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := ctx.servicesFor(cmd).ReplayRun(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), events)
		},
	})

	debugCmd.AddCommand(buildRunCommand(ctx, "run <instruction>", "Create a one-off run for debugging or scripting"))
	debugCmd.AddCommand(buildInspectCommand(ctx, "inspect <run-id>", "Inspect run state, current step, failures, and child summaries"))
	debugCmd.AddCommand(buildReplayCommand(ctx, "replay <run-id>", "Replay persisted events for a run as summarized timeline entries"))
	debugCmd.AddCommand(buildResumeCommand(ctx, "resume <run-id>", "Resume an unfinished run from persisted artifacts"))
	debugCmd.AddCommand(buildSessionInspectCommand(ctx, "session <session-id>", "Inspect session messages and related runs"))
	debugCmd.AddCommand(buildToolsListCommand(ctx, "tools", "List built-in tool descriptors"))

	return debugCmd
}
