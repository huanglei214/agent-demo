package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/huanglei214/agent-demo/internal/httpapi"
)

func newServeCommand(ctx *commandContext) *cobra.Command {
	var host string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local HTTP API for the web UI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr := fmt.Sprintf("%s:%d", host, port)
			server := &http.Server{
				Addr:              addr,
				Handler:           httpapi.NewRouter(ctx.services()),
				ReadHeaderTimeout: 5 * time.Second,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "serving local harness API on http://%s\n", addr)
			return server.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host interface for the local API server")
	cmd.Flags().IntVar(&port, "port", 8080, "Port for the local API server")
	return cmd
}
