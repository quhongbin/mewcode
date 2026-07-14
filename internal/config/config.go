package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// ProviderConfig 单个供应商配置
type ProviderConfig struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
	Thinking *bool  `yaml:"thinking,omitempty"`
}

// Config 配置文件结构
type Config struct {
	Providers      []ProviderConfig `yaml:"providers"`
	ActiveProvider string           `yaml:"active_provider"`
}

// LoadConfig 加载并验证配置文件
func LoadConfig(path string) (Config, error) { // Config: 本文件中定义的结构体
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&cfg); err != nil { // validateConfig: 本文件中定义的函数
		return Config{}, err
	}

	return cfg, nil
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error { // Config: 本文件中定义的结构体
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("providers list is empty")
	}

	for i, p := range cfg.Providers {
		if p.Name == "" {
			return fmt.Errorf("provider[%d]: name is required", i)
		}
		if p.Protocol == "" {
			return fmt.Errorf("provider[%d]: protocol is required", i)
		}
		if p.Protocol != "anthropic" && p.Protocol != "openai" {
			return fmt.Errorf("provider[%d]: protocol must be 'anthropic' or 'openai', got '%s'", i, p.Protocol)
		}
		if p.Model == "" {
			return fmt.Errorf("provider[%d]: model is required", i)
		}
		if p.BaseURL == "" {
			return fmt.Errorf("provider[%d]: base_url is required", i)
		}
		if _, err := url.Parse(p.BaseURL); err != nil {
			return fmt.Errorf("provider[%d]: base_url is invalid: %w", i, err)
		}
		if p.APIKey == "" {
			return fmt.Errorf("provider[%d]: api_key is required", i)
		}
	}

	if cfg.ActiveProvider == "" {
		return fmt.Errorf("active_provider is required")
	}

	found := false
	for _, p := range cfg.Providers {
		if p.Name == cfg.ActiveProvider {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("active_provider '%s' not found in providers list", cfg.ActiveProvider)
	}

	return nil
}
