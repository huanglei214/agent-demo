package cli

import "github.com/spf13/cobra"

func newToolsCommand(ctx *commandContext) *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Inspect registered tool descriptors",
	}

	toolsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List built-in tool descriptors",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSONTo(cmd.OutOrStdout(), ctx.services().ListTools())
		},
	})

	return toolsCmd
}
