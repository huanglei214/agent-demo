package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/app"
	"github.com/huanglei214/agent-demo/internal/config"
)

type commandContext struct {
	config config.Config
}

func NewRootCommand() (*cobra.Command, error) {
	workspace, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := config.Load(workspace)
	ctx := commandContext{
		config: cfg,
	}

	rootCmd := &cobra.Command{
		Use:   "harness",
		Short: "Local agent harness CLI",
	}

	rootCmd.PersistentFlags().StringVar(&ctx.config.Workspace, "workspace", cfg.Workspace, "Workspace root for the run")
	rootCmd.PersistentFlags().StringVar(&ctx.config.Model.Provider, "provider", cfg.Model.Provider, "Model provider name")
	rootCmd.PersistentFlags().StringVar(&ctx.config.Model.Model, "model", cfg.Model.Model, "Model identifier")

	rootCmd.AddCommand(
		newRunCommand(&ctx),
		newChatCommand(&ctx),
		newServeCommand(&ctx),
		newInspectCommand(&ctx),
		newReplayCommand(&ctx),
		newSessionCommand(&ctx),
		newResumeCommand(&ctx),
		newToolsCommand(&ctx),
		newDebugCommand(&ctx),
	)

	return rootCmd, nil
}

func printJSONTo(writer io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func (c *commandContext) services() app.Services {
	cfg := config.Load(c.config.Workspace)
	cfg.Model.Provider = c.config.Model.Provider
	cfg.Model.Model = c.config.Model.Model
	return app.NewServices(cfg)
}
