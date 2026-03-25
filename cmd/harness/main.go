package main

import (
	"log"
	"os"

	"github.com/huanglei214/agent-demo/internal/cli"
)

func main() {
	cmd, err := cli.NewRootCommand()
	if err != nil {
		log.Printf("failed to initialize CLI: %v", err)
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
