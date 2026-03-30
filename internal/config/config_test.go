package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithOverridesPrecedence(t *testing.T) {
	workspace := t.TempDir()

	if err := os.WriteFile(filepath.Join(workspace, "config.json"), []byte(`{
  "runtime": {"root": "workspace-runtime"},
  "model": {
    "provider": "ark",
    "model": "workspace-model",
    "timeout_seconds": 60,
    "ark": {
      "base_url": "https://workspace.example.com",
      "model_id": "workspace-ark",
      "tpm": 2400,
      "max_concurrent": 4
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspace, ".env"), []byte("ARK_API_KEY=dotenv-key\n"), 0o644); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}

	t.Setenv("HARNESS_PROVIDER", "env-provider")
	t.Setenv("ARK_API_KEY", "")
	t.Setenv("HARNESS_MODEL", "env-model")
	t.Setenv("HARNESS_MODEL_TIMEOUT_SECONDS", "75")
	t.Setenv("ARK_BASE_URL", "https://env.example.com")
	t.Setenv("ARK_MODEL_ID", "")
	t.Setenv("ARK_TPM", "3600")
	t.Setenv("ARK_MAX_CONCURRENT", "6")

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
	if cfg.Model.Ark.APIKey != "dotenv-key" {
		t.Fatalf("expected dotenv ark api key override, got %q", cfg.Model.Ark.APIKey)
	}
	if cfg.Model.Ark.BaseURL != "https://env.example.com" {
		t.Fatalf("expected env ark base url override, got %q", cfg.Model.Ark.BaseURL)
	}
	if cfg.Model.Ark.ModelID != "workspace-ark" {
		t.Fatalf("expected workspace model id, got %q", cfg.Model.Ark.ModelID)
	}
	if cfg.Model.Ark.TPM != 3600 {
		t.Fatalf("expected env ark TPM override, got %d", cfg.Model.Ark.TPM)
	}
	if cfg.Model.Ark.MaxConcurrent != 6 {
		t.Fatalf("expected env ark max concurrent override, got %d", cfg.Model.Ark.MaxConcurrent)
	}
}

func TestLoadWithOverridesConfigInterpolatesDotEnv(t *testing.T) {
	workspace := t.TempDir()
	clearConfigEnv(t)

	if err := os.WriteFile(filepath.Join(workspace, ".env"), []byte("ARK_API_KEY=dotenv-key\n"), 0o644); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "config.json"), []byte(`{
  "model": {
    "provider": "ark",
    "ark": {
      "api_key": "${ARK_API_KEY}",
      "base_url": "https://ark.cn-beijing.volces.com/api/v3",
      "model_id": "ep-test"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	cfg := Load(workspace)
	if cfg.Model.Ark.APIKey != "dotenv-key" {
		t.Fatalf("expected interpolated dotenv api key, got %q", cfg.Model.Ark.APIKey)
	}
}

func TestLoadWithOverridesConfigInterpolationPrefersProcessEnv(t *testing.T) {
	workspace := t.TempDir()
	clearConfigEnv(t)

	if err := os.WriteFile(filepath.Join(workspace, ".env"), []byte("ARK_API_KEY=dotenv-key\n"), 0o644); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "config.json"), []byte(`{
  "model": {
    "provider": "ark",
    "ark": {
      "api_key": "${ARK_API_KEY}",
      "base_url": "https://ark.cn-beijing.volces.com/api/v3",
      "model_id": "ep-test"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}
	t.Setenv("ARK_API_KEY", "env-key")

	cfg := Load(workspace)
	if cfg.Model.Ark.APIKey != "env-key" {
		t.Fatalf("expected process env to win during interpolation, got %q", cfg.Model.Ark.APIKey)
	}
}

func TestLoadWithOverridesWorkspaceFlagSelectsWorkspaceConfig(t *testing.T) {
	clearConfigEnv(t)
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	if err := os.WriteFile(filepath.Join(workspaceA, "config.json"), []byte(`{"model":{"provider":"mock"}}`), 0o644); err != nil {
		t.Fatalf("write workspace A config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceB, "config.json"), []byte(`{"model":{"provider":"ark","model":"workspace-b"}}`), 0o644); err != nil {
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

func TestLoadWithOverridesEnvironmentWinsOverDotEnv(t *testing.T) {
	workspace := t.TempDir()
	clearConfigEnv(t)

	if err := os.WriteFile(filepath.Join(workspace, ".env"), []byte("ARK_API_KEY=dotenv-key\n"), 0o644); err != nil {
		t.Fatalf("write dotenv file: %v", err)
	}
	t.Setenv("ARK_API_KEY", "env-key")

	cfg := Load(workspace)
	if cfg.Model.Ark.APIKey != "env-key" {
		t.Fatalf("expected process env to override dotenv, got %q", cfg.Model.Ark.APIKey)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ARK_API_KEY",
		"ARK_BASE_URL",
		"ARK_MODEL_ID",
		"ARK_TPM",
		"ARK_MAX_CONCURRENT",
		"HARNESS_PROVIDER",
		"HARNESS_MODEL",
		"HARNESS_MODEL_TIMEOUT_SECONDS",
		"HARNESS_RUNTIME_ROOT",
	} {
		t.Setenv(key, "")
	}
}
