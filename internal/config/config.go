package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	APIKey        string
	BaseURL       string
	ModelID       string
	TPM           int
	MaxConcurrent int
}

var envPlaceholderPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

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
	APIKey        *string `json:"api_key,omitempty"`
	BaseURL       *string `json:"base_url,omitempty"`
	ModelID       *string `json:"model_id,omitempty"`
	TPM           *int    `json:"tpm,omitempty"`
	MaxConcurrent *int    `json:"max_concurrent,omitempty"`
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

	dotEnvValues, _ := loadDotEnv(dotEnvPath(workspace))
	mergeConfigFile(&cfg, workspaceConfigPath(workspace), dotEnvValues)
	applyDotEnvOverrides(&cfg, dotEnvValues)
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

func mergeConfigFile(cfg *Config, path string, dotEnvValues map[string]string) {
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
	interpolateFileConfig(&parsed, dotEnvValues)
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
			if parsed.Model.Ark.TPM != nil && *parsed.Model.Ark.TPM > 0 {
				cfg.Model.Ark.TPM = *parsed.Model.Ark.TPM
			}
			if parsed.Model.Ark.MaxConcurrent != nil && *parsed.Model.Ark.MaxConcurrent > 0 {
				cfg.Model.Ark.MaxConcurrent = *parsed.Model.Ark.MaxConcurrent
			}
		}
	}
}

func interpolateFileConfig(cfg *fileConfig, dotEnvValues map[string]string) {
	if cfg == nil {
		return
	}
	if cfg.Runtime != nil && cfg.Runtime.Root != nil {
		value := interpolateString(*cfg.Runtime.Root, dotEnvValues)
		cfg.Runtime.Root = &value
	}
	if cfg.Model == nil {
		return
	}
	if cfg.Model.Provider != nil {
		value := interpolateString(*cfg.Model.Provider, dotEnvValues)
		cfg.Model.Provider = &value
	}
	if cfg.Model.Model != nil {
		value := interpolateString(*cfg.Model.Model, dotEnvValues)
		cfg.Model.Model = &value
	}
	if cfg.Model.Ark == nil {
		return
	}
	if cfg.Model.Ark.APIKey != nil {
		value := interpolateString(*cfg.Model.Ark.APIKey, dotEnvValues)
		cfg.Model.Ark.APIKey = &value
	}
	if cfg.Model.Ark.BaseURL != nil {
		value := interpolateString(*cfg.Model.Ark.BaseURL, dotEnvValues)
		cfg.Model.Ark.BaseURL = &value
	}
	if cfg.Model.Ark.ModelID != nil {
		value := interpolateString(*cfg.Model.Ark.ModelID, dotEnvValues)
		cfg.Model.Ark.ModelID = &value
	}
}

func interpolateString(value string, dotEnvValues map[string]string) string {
	if value == "" {
		return value
	}
	return envPlaceholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := envPlaceholderPattern.FindStringSubmatch(match)
		if len(name) != 2 {
			return match
		}
		if envValue := os.Getenv(name[1]); envValue != "" {
			return envValue
		}
		if dotEnvValues != nil {
			if envValue, ok := dotEnvValues[name[1]]; ok {
				return envValue
			}
		}
		return match
	})
}

func applyDotEnvOverrides(cfg *Config, values map[string]string) {
	if len(values) == 0 {
		return
	}
	applyLookupOverrides(cfg, func(key string) string {
		if os.Getenv(key) != "" {
			return ""
		}
		return values[key]
	})
}

func loadDotEnv(path string) (map[string]string, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
				value = strings.Trim(value, `"`)
			} else if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
				value = strings.Trim(value, `'`)
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func applyEnvOverrides(cfg *Config) {
	applyLookupOverrides(cfg, os.Getenv)
}

func applyLookupOverrides(cfg *Config, lookup func(string) string) {
	if value := lookup("HARNESS_RUNTIME_ROOT"); value != "" {
		cfg.Runtime.Root = value
	}
	if value := lookup("HARNESS_PROVIDER"); value != "" {
		cfg.Model.Provider = value
	}
	if value := lookup("HARNESS_MODEL"); value != "" {
		cfg.Model.Model = value
	}
	cfg.Model.TimeoutSeconds = defaultInt(lookup("HARNESS_MODEL_TIMEOUT_SECONDS"), cfg.Model.TimeoutSeconds)
	if value := lookup("ARK_API_KEY"); value != "" {
		cfg.Model.Ark.APIKey = value
	}
	if value := lookup("ARK_BASE_URL"); value != "" {
		cfg.Model.Ark.BaseURL = value
	}
	if value := lookup("ARK_MODEL_ID"); value != "" {
		cfg.Model.Ark.ModelID = value
	}
	cfg.Model.Ark.TPM = defaultInt(lookup("ARK_TPM"), cfg.Model.Ark.TPM)
	cfg.Model.Ark.MaxConcurrent = defaultInt(lookup("ARK_MAX_CONCURRENT"), cfg.Model.Ark.MaxConcurrent)
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
	return filepath.Join(workspace, "config.json")
}

func dotEnvPath(workspace string) string {
	if workspace == "" {
		return ""
	}
	return filepath.Join(workspace, ".env")
}

func resolveConfigPath(baseDir, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
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
