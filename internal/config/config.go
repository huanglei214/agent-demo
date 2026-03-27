package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Workspace string
	Runtime   RuntimeConfig
	Model     ModelConfig
}

type RuntimeConfig struct {
	Root string
}

type ModelConfig struct {
	Provider       string
	Model          string
	TimeoutSeconds int
	Ark            ArkConfig
}

type ArkConfig struct {
	APIKey  string
	BaseURL string
	ModelID string
}

func Load(workspace string) Config {
	if workspace == "" {
		cwd, err := os.Getwd()
		if err == nil {
			workspace = cwd
		}
	}

	return Config{
		Workspace: workspace,
		Runtime: RuntimeConfig{
			Root: filepath.Join(workspace, ".runtime"),
		},
		Model: ModelConfig{
			Provider:       defaultValue(os.Getenv("HARNESS_PROVIDER"), "ark"),
			Model:          defaultValue(os.Getenv("HARNESS_MODEL"), os.Getenv("ARK_MODEL_ID")),
			TimeoutSeconds: defaultInt(os.Getenv("HARNESS_MODEL_TIMEOUT_SECONDS"), 90),
			Ark: ArkConfig{
				APIKey:  os.Getenv("ARK_API_KEY"),
				BaseURL: os.Getenv("ARK_BASE_URL"),
				ModelID: os.Getenv("ARK_MODEL_ID"),
			},
		},
	}
}

func defaultValue(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func defaultInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
