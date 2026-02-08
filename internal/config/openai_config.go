// Package config provides configuration management for kimi-go.
// This version uses OpenAI-compatible API format with environment variables for secrets.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the main configuration structure.
// API keys are NOT stored here - they are read from environment variables.
type Config struct {
	Version         string                    `toml:"version"`
	DefaultProvider string                    `toml:"default_provider"`
	DefaultModel    string                    `toml:"default_model"`
	DefaultThinking bool                      `toml:"default_thinking"`
	DefaultYOLO     bool                      `toml:"default_yolo"`
	Providers       map[string]ProviderConfig `toml:"providers"`
	Models          map[string]ModelConfig    `toml:"models"`
	LoopControl     LoopControl               `toml:"loop_control"`
}

// ModelConfig represents a model configuration.
type ModelConfig struct {
	Provider       string `toml:"provider"`
	Model          string `toml:"model"`
	MaxContextSize int    `toml:"max_context_size"`
}

// ProviderConfig represents an API provider configuration.
type ProviderConfig struct {
	// Provider type: "openai", "anthropic", "custom", etc.
	Type string `toml:"type"`

	// BaseURL for the API (e.g., "https://api.openai.com/v1")
	BaseURL string `toml:"base_url"`

	// APIKey stores the API key directly (alternative to env var)
	APIKey string `toml:"api_key,omitempty"`

	// EnvKey is the environment variable name for the API key
	// Default: PROVIDER_NAME_API_KEY (e.g., OPENAI_API_KEY)
	EnvKey string `toml:"env_key,omitempty"`

	// Timeout in seconds for API requests
	Timeout int `toml:"timeout,omitempty"`

	// Custom headers to add to requests
	Headers map[string]string `toml:"headers,omitempty"`
	
	// Retry configuration for this provider
	Retry *RetryConfig `toml:"retry,omitempty"`
}

// GetAPIKey retrieves the API key. It checks the direct APIKey field first,
// then falls back to the environment variable.
func (p ProviderConfig) GetAPIKey() (string, error) {
	// Check direct APIKey first
	if p.APIKey != "" {
		return p.APIKey, nil
	}

	envKey := p.EnvKey
	if envKey == "" {
		// Default convention: uppercase provider type + _API_KEY
		envKey = fmt.Sprintf("%s_API_KEY", upper(p.Type))
	}

	apiKey := os.Getenv(envKey)
	if apiKey == "" {
		return "", fmt.Errorf("API key not found in environment variable: %s", envKey)
	}

	return apiKey, nil
}

// LoopControl contains loop execution parameters.
type LoopControl struct {
	MaxStepsPerTurn     int `toml:"max_steps_per_turn"`
	MaxRetriesPerStep   int `toml:"max_retries_per_step"`
	MaxRalphIterations  int `toml:"max_ralph_iterations"`
	ReservedContextSize int `toml:"reserved_context_size"`
}

// RetryConfig contains retry strategy configuration for LLM requests.
type RetryConfig struct {
	MaxRetries       int `toml:"max_retries"`         // 最大重试次数，默认 3
	InitialWaitMs    int `toml:"initial_wait_ms"`     // 初始等待时间（毫秒），默认 300
	MaxWaitMs        int `toml:"max_wait_ms"`         // 最大等待时间（毫秒），默认 5000
	ExponentialBase  float64 `toml:"exponential_base"`  // 指数基数，默认 2.0
	JitterMs         int `toml:"jitter_ms"`           // 抖动范围（毫秒），默认 500
}

// DefaultConfig returns a default configuration.
// This creates a template with common providers.
func DefaultConfig() *Config {
	return &Config{
		Version:         "1.0",
		DefaultProvider: "openai",
		DefaultModel:    "gpt-4",
		Providers: map[string]ProviderConfig{
			"openai": {
				Type:    "openai",
				BaseURL: "https://api.openai.com/v1",
				EnvKey:  "OPENAI_API_KEY",
				Timeout: 60,
			},
			"anthropic": {
				Type:    "anthropic",
				BaseURL: "https://api.anthropic.com/v1",
				EnvKey:  "ANTHROPIC_API_KEY",
				Timeout: 60,
			},
			"custom": {
				Type:    "openai",
				BaseURL: "https://your-api-endpoint.com/v1",
				EnvKey:  "CUSTOM_API_KEY",
				Timeout: 60,
			},
		},
		Models: map[string]ModelConfig{},
		LoopControl: LoopControl{
			MaxStepsPerTurn:    100,
			MaxRetriesPerStep:  3,
			MaxRalphIterations: 3,
		},
	}
}

// LoadConfig loads configuration from file or returns default.
// Environment variables are NOT stored in the config file.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	config := &Config{}
	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Initialize nil maps
	if config.Providers == nil {
		config.Providers = make(map[string]ProviderConfig)
	}
	if config.Models == nil {
		config.Models = make(map[string]ModelConfig)
	}

	return config, nil
}

// SaveConfig saves configuration to file.
// Note: API keys are NOT saved - they must be set via environment variables.
func SaveConfig(config *Config, path string) error {
	if path == "" {
		path = defaultConfigPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Add a comment about environment variables
	if _, err := file.WriteString("# Kimi-Go Configuration\n"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := file.WriteString("# NOTE: API keys are NOT stored here.\n"); err != nil {
		return fmt.Errorf("failed to write note: %w", err)
	}
	if _, err := file.WriteString("# Set them via environment variables (e.g., OPENAI_API_KEY)\n\n"); err != nil {
		return fmt.Errorf("failed to write env note: %w", err)
	}

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// GetProvider returns a provider by name.
func (c *Config) GetProvider(name string) (ProviderConfig, bool) {
	provider, ok := c.Providers[name]
	return provider, ok
}

// GetDefaultProvider returns the default provider configuration.
func (c *Config) GetDefaultProvider() (ProviderConfig, bool) {
	return c.GetProvider(c.DefaultProvider)
}

// defaultConfigPath returns the default config file path.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kimi/config.toml"
	}
	return filepath.Join(home, ".kimi", "config.toml")
}

// upper converts a string to uppercase.
func upper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		result[i] = c
	}
	return string(result)
}
