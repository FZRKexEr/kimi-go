// Package config provides configuration management for kimi-go.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Version != "1.0" {
		t.Errorf("Expected version '1.0', got %s", cfg.Version)
	}

	if cfg.DefaultThinking != false {
		t.Error("Expected DefaultThinking to be false")
	}

	if cfg.DefaultYOLO != false {
		t.Error("Expected DefaultYOLO to be false")
	}

	if cfg.LoopControl.MaxStepsPerTurn != 100 {
		t.Errorf("Expected MaxStepsPerTurn to be 100, got %d", cfg.LoopControl.MaxStepsPerTurn)
	}

	if cfg.LoopControl.MaxRetriesPerStep != 3 {
		t.Errorf("Expected MaxRetriesPerStep to be 3, got %d", cfg.LoopControl.MaxRetriesPerStep)
	}
}

func TestLoadConfig_NotExist(t *testing.T) {
	// Load from non-existent path should return default config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nonexistent.toml")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	// Should be default values
	if cfg.Version != "1.0" {
		t.Errorf("Expected version '1.0', got %s", cfg.Version)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `
version = "2.0"
default_model = "test-model"
default_thinking = true
default_yolo = true

[loop_control]
max_steps_per_turn = 50
max_retries_per_step = 5
max_ralph_iterations = 10
reserved_context_size = 10000

[providers.test]
type = "kimi"
base_url = "https://api.example.com"
api_key = "test-key"

[models.test-model]
provider = "test"
model = "kimi-k2.5"
max_context_size = 100000
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Version != "2.0" {
		t.Errorf("Expected version '2.0', got %s", cfg.Version)
	}

	if cfg.DefaultModel != "test-model" {
		t.Errorf("Expected default_model 'test-model', got %s", cfg.DefaultModel)
	}

	if !cfg.DefaultThinking {
		t.Error("Expected default_thinking to be true")
	}

	if !cfg.DefaultYOLO {
		t.Error("Expected default_yolo to be true")
	}

	if cfg.LoopControl.MaxStepsPerTurn != 50 {
		t.Errorf("Expected MaxStepsPerTurn to be 50, got %d", cfg.LoopControl.MaxStepsPerTurn)
	}

	if cfg.LoopControl.MaxRetriesPerStep != 5 {
		t.Errorf("Expected MaxRetriesPerStep to be 5, got %d", cfg.LoopControl.MaxRetriesPerStep)
	}

	if cfg.LoopControl.MaxRalphIterations != 10 {
		t.Errorf("Expected MaxRalphIterations to be 10, got %d", cfg.LoopControl.MaxRalphIterations)
	}

	// Check providers
	if len(cfg.Providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	provider, ok := cfg.Providers["test"]
	if !ok {
		t.Fatal("Expected 'test' provider to exist")
	}

	if provider.Type != "kimi" {
		t.Errorf("Expected provider type 'kimi', got %s", provider.Type)
	}

	if provider.BaseURL != "https://api.example.com" {
		t.Errorf("Expected base_url 'https://api.example.com', got %s", provider.BaseURL)
	}

	// Check models
	if len(cfg.Models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(cfg.Models))
	}

	model, ok := cfg.Models["test-model"]
	if !ok {
		t.Fatal("Expected 'test-model' model to exist")
	}

	if model.Provider != "test" {
		t.Errorf("Expected model provider 'test', got %s", model.Provider)
	}

	if model.Model != "kimi-k2.5" {
		t.Errorf("Expected model 'kimi-k2.5', got %s", model.Model)
	}

	if model.MaxContextSize != 100000 {
		t.Errorf("Expected max_context_size 100000, got %d", model.MaxContextSize)
	}
}

func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	cfg := &Config{
		Version:         "1.0",
		DefaultModel:    "default",
		DefaultThinking: false,
		DefaultYOLO:     false,
		Providers: map[string]ProviderConfig{
			"default": {
				Type:    "kimi",
				BaseURL: "https://api.example.com",
				APIKey:  "secret-key",
			},
		},
		Models: map[string]ModelConfig{
			"default": {
				Provider:       "default",
				Model:          "kimi-k2.5",
				MaxContextSize: 100000,
			},
		},
		LoopControl: LoopControl{
			MaxStepsPerTurn:    100,
			MaxRetriesPerStep:  3,
			MaxRalphIterations: 0,
		},
	}

	if err := SaveConfig(cfg, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load and verify
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loaded.Version != cfg.Version {
		t.Errorf("Expected version %s, got %s", cfg.Version, loaded.Version)
	}

	if loaded.DefaultModel != cfg.DefaultModel {
		t.Errorf("Expected default model %s, got %s", cfg.DefaultModel, loaded.DefaultModel)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := defaultConfigPath()
	if path == "" {
		t.Error("Expected non-empty default config path")
	}
}
