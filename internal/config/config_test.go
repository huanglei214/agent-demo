package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithOverridesPrecedence(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)

	userDir := filepath.Join(home, ".agent-demo")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("mkdir user config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "config.json"), []byte(`{
  "runtime": {"root": "user-runtime"},
  "model": {
    "provider": "mock",
    "model": "user-model",
    "timeout_seconds": 45,
    "ark": {
      "base_url": "https://user.example.com",
      "model_id": "user-ark"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspace, ".agent-demo.json"), []byte(`{
  "runtime": {"root": "workspace-runtime"},
  "model": {
    "provider": "ark",
    "model": "workspace-model",
    "timeout_seconds": 60,
    "ark": {
      "base_url": "https://workspace.example.com",
      "model_id": "workspace-ark"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	t.Setenv("HARNESS_PROVIDER", "env-provider")
	t.Setenv("HARNESS_MODEL", "env-model")
	t.Setenv("HARNESS_MODEL_TIMEOUT_SECONDS", "75")
	t.Setenv("ARK_BASE_URL", "https://env.example.com")
	t.Setenv("ARK_MODEL_ID", "")

	cfg := LoadWithOverrides(workspace, Overrides{
		ModelProvider: "explicit-provider",
		ModelName:     "explicit-model",
	})

	if cfg.Runtime.Root != filepath.Join(workspace, "workspace-runtime") {
		t.Fatalf("expected workspace config runtime root, got %q", cfg.Runtime.Root)
	}
	if cfg.Model.Provider != "explicit-provider" {
		t.Fatalf("expected explicit provider, got %q", cfg.Model.Provider)
	}
	if cfg.Model.Model != "explicit-model" {
		t.Fatalf("expected explicit model, got %q", cfg.Model.Model)
	}
	if cfg.Model.TimeoutSeconds != 75 {
		t.Fatalf("expected env timeout override, got %d", cfg.Model.TimeoutSeconds)
	}
	if cfg.Model.Ark.BaseURL != "https://env.example.com" {
		t.Fatalf("expected env ark base url override, got %q", cfg.Model.Ark.BaseURL)
	}
	if cfg.Model.Ark.ModelID != "workspace-ark" {
		t.Fatalf("expected workspace model id, got %q", cfg.Model.Ark.ModelID)
	}
}

func TestLoadWithOverridesWorkspaceFlagSelectsWorkspaceConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	if err := os.WriteFile(filepath.Join(workspaceA, ".agent-demo.json"), []byte(`{"model":{"provider":"mock"}}`), 0o644); err != nil {
		t.Fatalf("write workspace A config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceB, ".agent-demo.json"), []byte(`{"model":{"provider":"ark","model":"workspace-b"}}`), 0o644); err != nil {
		t.Fatalf("write workspace B config: %v", err)
	}

	cfg := LoadWithOverrides(workspaceA, Overrides{
		Workspace: workspaceB,
	})

	if cfg.Workspace != workspaceB {
		t.Fatalf("expected workspace override to win, got %q", cfg.Workspace)
	}
	if cfg.Model.Provider != "ark" {
		t.Fatalf("expected workspace B provider, got %q", cfg.Model.Provider)
	}
	if cfg.Model.Model != "workspace-b" {
		t.Fatalf("expected workspace B model, got %q", cfg.Model.Model)
	}
}
