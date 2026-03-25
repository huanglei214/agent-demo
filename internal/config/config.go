package config

import (
	"os"
	"path/filepath"
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
	Provider string
	Model    string
	Ark      ArkConfig
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
			Provider: defaultValue(os.Getenv("HARNESS_PROVIDER"), "ark"),
			Model:    defaultValue(os.Getenv("HARNESS_MODEL"), os.Getenv("ARK_MODEL_ID")),
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
