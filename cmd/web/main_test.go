package main

import "testing"

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
