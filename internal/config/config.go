package config

import (
	"encoding/json"
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
	return LoadWithOverrides(workspace, Overrides{})
}

type Overrides struct {
	Workspace        string
	RuntimeRoot      string
	ModelProvider    string
	ModelName        string
	ModelTimeoutSecs int
	ArkAPIKey        string
	ArkBaseURL       string
	ArkModelID       string
}

type fileConfig struct {
	Runtime *fileRuntimeConfig `json:"runtime,omitempty"`
	Model   *fileModelConfig   `json:"model,omitempty"`
}

type fileRuntimeConfig struct {
	Root *string `json:"root,omitempty"`
}

type fileModelConfig struct {
	Provider       *string        `json:"provider,omitempty"`
	Model          *string        `json:"model,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	Ark            *fileArkConfig `json:"ark,omitempty"`
}

type fileArkConfig struct {
	APIKey  *string `json:"api_key,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	ModelID *string `json:"model_id,omitempty"`
}

func LoadWithOverrides(workspace string, overrides Overrides) Config {
	if overrides.Workspace != "" {
		workspace = overrides.Workspace
	}
	if workspace == "" {
		cwd, err := os.Getwd()
		if err == nil {
			workspace = cwd
		}
	}

	cfg := Config{
		Workspace: workspace,
		Runtime: RuntimeConfig{
			Root: filepath.Join(workspace, ".runtime"),
		},
		Model: ModelConfig{
			Provider:       "ark",
			Model:          "",
			TimeoutSeconds: 90,
			Ark: ArkConfig{
				APIKey:  "",
				BaseURL: "",
				ModelID: "",
			},
		},
	}

	mergeConfigFile(&cfg, userConfigPath())
	mergeConfigFile(&cfg, workspaceConfigPath(workspace))
	applyEnvOverrides(&cfg)
	applyExplicitOverrides(&cfg, overrides)

	if cfg.Model.Model == "" {
		cfg.Model.Model = cfg.Model.Ark.ModelID
	}
	if cfg.Runtime.Root == "" {
		cfg.Runtime.Root = filepath.Join(cfg.Workspace, ".runtime")
	}
	return cfg
}

func mergeConfigFile(cfg *Config, path string) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var parsed fileConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return
	}
	baseDir := filepath.Dir(path)
	if parsed.Runtime != nil && parsed.Runtime.Root != nil {
		cfg.Runtime.Root = resolveConfigPath(baseDir, *parsed.Runtime.Root)
	}
	if parsed.Model != nil {
		if parsed.Model.Provider != nil {
			cfg.Model.Provider = *parsed.Model.Provider
		}
		if parsed.Model.Model != nil {
			cfg.Model.Model = *parsed.Model.Model
		}
		if parsed.Model.TimeoutSeconds != nil && *parsed.Model.TimeoutSeconds > 0 {
			cfg.Model.TimeoutSeconds = *parsed.Model.TimeoutSeconds
		}
		if parsed.Model.Ark != nil {
			if parsed.Model.Ark.APIKey != nil {
				cfg.Model.Ark.APIKey = *parsed.Model.Ark.APIKey
			}
			if parsed.Model.Ark.BaseURL != nil {
				cfg.Model.Ark.BaseURL = *parsed.Model.Ark.BaseURL
			}
			if parsed.Model.Ark.ModelID != nil {
				cfg.Model.Ark.ModelID = *parsed.Model.Ark.ModelID
			}
		}
	}
}

func applyEnvOverrides(cfg *Config) {
	if value := os.Getenv("HARNESS_RUNTIME_ROOT"); value != "" {
		cfg.Runtime.Root = value
	}
	if value := os.Getenv("HARNESS_PROVIDER"); value != "" {
		cfg.Model.Provider = value
	}
	if value := os.Getenv("HARNESS_MODEL"); value != "" {
		cfg.Model.Model = value
	}
	cfg.Model.TimeoutSeconds = defaultInt(os.Getenv("HARNESS_MODEL_TIMEOUT_SECONDS"), cfg.Model.TimeoutSeconds)
	if value := os.Getenv("ARK_API_KEY"); value != "" {
		cfg.Model.Ark.APIKey = value
	}
	if value := os.Getenv("ARK_BASE_URL"); value != "" {
		cfg.Model.Ark.BaseURL = value
	}
	if value := os.Getenv("ARK_MODEL_ID"); value != "" {
		cfg.Model.Ark.ModelID = value
	}
}

func applyExplicitOverrides(cfg *Config, overrides Overrides) {
	if overrides.Workspace != "" {
		cfg.Workspace = overrides.Workspace
	}
	if overrides.RuntimeRoot != "" {
		cfg.Runtime.Root = overrides.RuntimeRoot
	}
	if overrides.ModelProvider != "" {
		cfg.Model.Provider = overrides.ModelProvider
	}
	if overrides.ModelName != "" {
		cfg.Model.Model = overrides.ModelName
	}
	if overrides.ModelTimeoutSecs > 0 {
		cfg.Model.TimeoutSeconds = overrides.ModelTimeoutSecs
	}
	if overrides.ArkAPIKey != "" {
		cfg.Model.Ark.APIKey = overrides.ArkAPIKey
	}
	if overrides.ArkBaseURL != "" {
		cfg.Model.Ark.BaseURL = overrides.ArkBaseURL
	}
	if overrides.ArkModelID != "" {
		cfg.Model.Ark.ModelID = overrides.ArkModelID
	}
}

func workspaceConfigPath(workspace string) string {
	if workspace == "" {
		return ""
	}
	return filepath.Join(workspace, ".agent-demo.json")
}

func userConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".agent-demo", "config.json")
}

func resolveConfigPath(baseDir, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
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
