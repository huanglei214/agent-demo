package main

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestNewRootCommandExposesExpectedFlags(t *testing.T) {
	cmd, err := newRootCommand(t.TempDir())
	if err != nil {
		t.Fatalf("newRootCommand returned error: %v", err)
	}

	for _, name := range []string{"workspace", "provider", "model", "host", "port"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected %q flag to be registered", name)
		}
	}

	if cmd.Use != "harness-web" {
		t.Fatalf("expected Cobra-style command use, got %q", cmd.Use)
	}
}

func TestServeServerReturnsAfterShutdownSignal(t *testing.T) {
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}
	signals := make(chan os.Signal, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()

	if err := serveServer(server, signals); err != nil {
		t.Fatalf("serveServer returned error: %v", err)
	}
}
