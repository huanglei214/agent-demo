package cli

import "github.com/spf13/cobra"

func newResumeCommand(ctx *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume an unfinished run from persisted artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := ctx.services().ResumeRun(args[0])
			if err != nil {
				return err
			}
			return printJSONTo(cmd.OutOrStdout(), response)
		},
	}
}
