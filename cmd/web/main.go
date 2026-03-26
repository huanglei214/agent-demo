package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/huanglei214/agent-demo/internal/app"
	"github.com/huanglei214/agent-demo/internal/config"
	httpapi "github.com/huanglei214/agent-demo/internal/interfaces/http"
)

func main() {
	workspace, err := os.Getwd()
	if err != nil {
		log.Printf("failed to detect current workspace: %v", err)
		os.Exit(1)
	}

	cfg := config.Load(workspace)
	host := "127.0.0.1"
	port := 8080

	flags := flag.NewFlagSet("web", flag.ExitOnError)
	flags.StringVar(&cfg.Workspace, "workspace", cfg.Workspace, "Workspace root for the local API server")
	flags.StringVar(&cfg.Model.Provider, "provider", cfg.Model.Provider, "Model provider name")
	flags.StringVar(&cfg.Model.Model, "model", cfg.Model.Model, "Model identifier")
	flags.StringVar(&host, "host", host, "Host interface for the local API server")
	flags.IntVar(&port, "port", port, "Port for the local API server")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: harness-web [flags]")
		fmt.Fprintln(flags.Output())
		flags.PrintDefaults()
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	cfg = config.Load(cfg.Workspace)
	cfg.Model.Provider = flagValueOrDefault(flags, "provider", cfg.Model.Provider)
	cfg.Model.Model = flagValueOrDefault(flags, "model", cfg.Model.Model)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
		Handler:           httpapi.NewRouter(app.NewServices(cfg)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("serving local harness API on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("local API server exited with error: %v", err)
		os.Exit(1)
	}
}

func flagValueOrDefault(flags *flag.FlagSet, name, fallback string) string {
	value := flags.Lookup(name)
	if value == nil || value.Value.String() == "" {
		return fallback
	}
	return value.Value.String()
}
