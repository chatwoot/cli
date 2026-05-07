package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveOmitsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &Config{
		BaseURL:   "https://app.chatwoot.com",
		AccountID: 123,
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if strings.Contains(content, "api_key") {
		t.Fatalf("saved config contains api_key: %s", content)
	}
	if !strings.Contains(content, "base_url") || !strings.Contains(content, "account_id") {
		t.Fatalf("saved config missing expected non-secret fields: %s", content)
	}
}

func TestLegacyAPIKeyIsIgnoredAndRemovedOnSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path := filepath.Join(dir, "config.yaml")
	legacy := "base_url: https://app.chatwoot.com\napi_key: plaintext-token\naccount_id: 123\n"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil || !cfg.IsValid() {
		t.Fatalf("Load() returned invalid config: %#v", cfg)
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "api_key") || strings.Contains(string(data), "plaintext-token") {
		t.Fatalf("legacy api key was not removed: %s", string(data))
	}
}
