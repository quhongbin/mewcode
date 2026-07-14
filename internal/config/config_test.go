package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	content := `
providers:
  - name: claude
    protocol: anthropic
    model: claude-3-opus-20240229
    base_url: https://api.anthropic.com
    api_key: sk-ant-test
    thinking: true
  - name: gpt
    protocol: openai
    model: gpt-4
    base_url: https://api.openai.com
    api_key: sk-test
active_provider: claude
`
	path := writeTempConfig(t, content) // writeTempConfig: 本文件中定义的函数
	cfg, err := LoadConfig(path) // LoadConfig: config.go 中定义的函数
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(cfg.Providers))
	}
	if cfg.ActiveProvider != "claude" {
		t.Errorf("expected active_provider 'claude', got '%s'", cfg.ActiveProvider)
	}
	if cfg.Providers[0].Thinking == nil || !*cfg.Providers[0].Thinking {
		t.Error("expected thinking to be true")
	}
}

func TestLoadConfig_MissingName(t *testing.T) {
	content := `
providers:
  - protocol: anthropic
    model: claude-3
    base_url: https://api.anthropic.com
    api_key: sk-test
active_provider: claude
`
	path := writeTempConfig(t, content) // writeTempConfig: 本文件中定义的函数
	_, err := LoadConfig(path) // LoadConfig: config.go 中定义的函数
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoadConfig_InvalidProtocol(t *testing.T) {
	content := `
providers:
  - name: test
    protocol: invalid
    model: test
    base_url: https://api.test.com
    api_key: sk-test
active_provider: test
`
	path := writeTempConfig(t, content) // writeTempConfig: 本文件中定义的函数
	_, err := LoadConfig(path) // LoadConfig: config.go 中定义的函数
	if err == nil {
		t.Fatal("expected error for invalid protocol, got nil")
	}
}

func TestLoadConfig_ActiveProviderNotFound(t *testing.T) {
	content := `
providers:
  - name: claude
    protocol: anthropic
    model: claude-3
    base_url: https://api.anthropic.com
    api_key: sk-test
active_provider: nonexistent
`
	path := writeTempConfig(t, content) // writeTempConfig: 本文件中定义的函数
	_, err := LoadConfig(path) // LoadConfig: config.go 中定义的函数
	if err == nil {
		t.Fatal("expected error for nonexistent active_provider, got nil")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	content := `
providers:
  - name: test
    protocol: anthropic
    model: test
    base_url: https://api.test.com
active_provider: test
`
	path := writeTempConfig(t, content) // writeTempConfig: 本文件中定义的函数
	_, err := LoadConfig(path) // LoadConfig: config.go 中定义的函数
	if err == nil {
		t.Fatal("expected error for missing api_key, got nil")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
