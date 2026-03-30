package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/config"
	httpapi "github.com/huanglei214/agent-demo/internal/interfaces/http"
	"github.com/huanglei214/agent-demo/internal/service"
)

type commandOptions struct {
	workspace string
	provider  string
	model     string
	host      string
	port      int
}

func main() {
	workspace, err := os.Getwd()
	if err != nil {
		log.Printf("failed to detect current workspace: %v", err)
		os.Exit(1)
	}

	cmd, err := newRootCommand(workspace)
	if err != nil {
		log.Printf("failed to initialize web command: %v", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		log.Printf("web command failed: %v", err)
		os.Exit(1)
	}
}

func newRootCommand(workspace string) (*cobra.Command, error) {
	cfg := config.Load(workspace)
	opts := commandOptions{
		workspace: cfg.Workspace,
		provider:  cfg.Model.Provider,
		model:     cfg.Model.Model,
		host:      "127.0.0.1",
		port:      8088,
	}

	cmd := &cobra.Command{
		Use:          "harness-web",
		Short:        "Start the local harness HTTP API server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.workspace, "workspace", opts.workspace, "Workspace root for the local API server")
	cmd.Flags().StringVar(&opts.provider, "provider", opts.provider, "Model provider name")
	cmd.Flags().StringVar(&opts.model, "model", opts.model, "Model identifier")
	cmd.Flags().StringVar(&opts.host, "host", opts.host, "Host interface for the local API server")
	cmd.Flags().IntVar(&opts.port, "port", opts.port, "Port for the local API server")
	return cmd, nil
}

func runServer(cmd *cobra.Command, opts commandOptions) error {
	cfg := config.LoadWithOverrides(opts.workspace, config.Overrides{
		Workspace:     changedStringFlag(cmd, "workspace", opts.workspace),
		ModelProvider: changedStringFlag(cmd, "provider", opts.provider),
		ModelName:     changedStringFlag(cmd, "model", opts.model),
	})

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", opts.host, opts.port),
		Handler:           httpapi.NewRouter(service.NewServices(cfg)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("serving local harness API on http://%s", server.Addr)
	if err := serveServer(server, shutdownSignals()); err != nil {
		return err
	}
	return nil
}

func serveServer(server *http.Server, signals <-chan os.Signal) error {
	if signals != nil {
		go func() {
			<-signals
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func shutdownSignals() <-chan os.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	return signals
}

func changedStringFlag(cmd *cobra.Command, name, value string) string {
	flag := cmd.Flags().Lookup(name)
	if flag == nil || !flag.Changed {
		return ""
	}
	return value
}
