package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/service"
)

type commandContext struct {
	workspace string
	provider  string
	model     string
}

func NewRootCommand() (*cobra.Command, error) {
	workspace, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := config.Load(workspace)
	ctx := commandContext{
		workspace: cfg.Workspace,
		provider:  cfg.Model.Provider,
		model:     cfg.Model.Model,
	}

	rootCmd := &cobra.Command{
		Use:   "harness",
		Short: "Local agent harness CLI with chat-first primary workflow",
	}

	rootCmd.PersistentFlags().StringVar(&ctx.workspace, "workspace", cfg.Workspace, "Workspace root for the run")
	rootCmd.PersistentFlags().StringVar(&ctx.provider, "provider", cfg.Model.Provider, "Model provider name")
	rootCmd.PersistentFlags().StringVar(&ctx.model, "model", cfg.Model.Model, "Model identifier")

	rootCmd.AddCommand(
		newChatCommand(&ctx),
		newDebugCommand(&ctx),
	)

	rootCmd.AddCommand(newLegacyRunCommand(&ctx))
	rootCmd.AddCommand(newLegacyInspectCommand(&ctx))
	rootCmd.AddCommand(newLegacyReplayCommand(&ctx))
	rootCmd.AddCommand(newLegacyResumeCommand(&ctx))
	rootCmd.AddCommand(newLegacySessionCommand(&ctx))
	rootCmd.AddCommand(newLegacyToolsCommand(&ctx))

	return rootCmd, nil
}

func newLegacyRunCommand(ctx *commandContext) *cobra.Command {
	cmd := newRunCommand(ctx)
	cmd.Hidden = true
	return cmd
}

func newLegacyInspectCommand(ctx *commandContext) *cobra.Command {
	cmd := newInspectCommand(ctx)
	cmd.Hidden = true
	return cmd
}

func newLegacyReplayCommand(ctx *commandContext) *cobra.Command {
	cmd := newReplayCommand(ctx)
	cmd.Hidden = true
	return cmd
}

func newLegacyResumeCommand(ctx *commandContext) *cobra.Command {
	cmd := newResumeCommand(ctx)
	cmd.Hidden = true
	return cmd
}

func newLegacySessionCommand(ctx *commandContext) *cobra.Command {
	cmd := newSessionCommand(ctx)
	cmd.Hidden = true
	for _, child := range cmd.Commands() {
		child.Hidden = true
	}
	return cmd
}

func newLegacyToolsCommand(ctx *commandContext) *cobra.Command {
	cmd := newToolsCommand(ctx)
	cmd.Hidden = true
	for _, child := range cmd.Commands() {
		child.Hidden = true
	}
	return cmd
}

func printJSONTo(writer io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func (c *commandContext) servicesFor(cmd *cobra.Command) service.Services {
	cfg := config.LoadWithOverrides(c.workspace, config.Overrides{
		Workspace:     explicitStringFlag(cmd, "workspace", c.workspace),
		ModelProvider: explicitStringFlag(cmd, "provider", c.provider),
		ModelName:     explicitStringFlag(cmd, "model", c.model),
	})
	return service.NewServices(cfg)
}

func explicitStringFlag(cmd *cobra.Command, name, value string) string {
	flag := cmd.Flags().Lookup(name)
	if flag == nil || !flag.Changed {
		return ""
	}
	return value
}
