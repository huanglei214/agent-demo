package config

import "testing"

func TestLoadUsesDefaultModelTimeoutSeconds(t *testing.T) {
	t.Setenv("HARNESS_MODEL_TIMEOUT_SECONDS", "")

	cfg := Load(t.TempDir())
	if cfg.Model.TimeoutSeconds != 90 {
		t.Fatalf("expected default model timeout 90, got %d", cfg.Model.TimeoutSeconds)
	}
}

func TestLoadUsesConfiguredModelTimeoutSeconds(t *testing.T) {
	t.Setenv("HARNESS_MODEL_TIMEOUT_SECONDS", "135")

	cfg := Load(t.TempDir())
	if cfg.Model.TimeoutSeconds != 135 {
		t.Fatalf("expected configured model timeout 135, got %d", cfg.Model.TimeoutSeconds)
	}
}
