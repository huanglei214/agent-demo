package cli

import "github.com/spf13/cobra"

func newToolsCommand(ctx *commandContext) *cobra.Command {
	return buildLegacyToolsCommand(ctx)
}

func buildLegacyToolsCommand(ctx *commandContext) *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Inspect registered tool descriptors",
	}

	toolsCmd.AddCommand(buildToolsListCommand(ctx, "list", "List built-in tool descriptors"))
	return toolsCmd
}

func buildToolsListCommand(ctx *commandContext, use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSONTo(cmd.OutOrStdout(), ctx.servicesFor(cmd).ListTools())
		},
	}
}
