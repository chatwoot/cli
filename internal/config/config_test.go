package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	// Derive the path from ConfigPath() rather than hardcoding "config.yaml" so
	// tests are correct under any build profile (e.g. dev → config.dev.yaml).
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func readConfigFile(t *testing.T) string {
	t.Helper()
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

func TestSaveWritesVersionedConfigWithoutSecrets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &Config{
		Default: "acme",
		Accounts: []Account{
			{Name: "acme", BaseURL: "https://app.chatwoot.com", ID: 42, UserID: 7},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	content := readConfigFile(t)
	if strings.Contains(content, "api_key") {
		t.Fatalf("saved config contains api_key: %s", content)
	}
	for _, want := range []string{"version: 2", "default: acme", "name: acme", "base_url: https://app.chatwoot.com", "id: 42"} {
		if !strings.Contains(content, want) {
			t.Fatalf("saved config missing %q: %s", want, content)
		}
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &Config{
		Default: "acme",
		Accounts: []Account{
			{
				Name: "acme", BaseURL: "https://app.chatwoot.com", ID: 42, UserID: 7,
				UserName: "Shivam", AccountName: "Acme",
				HelpCenter: HelpCenterConfig{DefaultPortalSlug: "acme-help", DefaultLocale: "en"},
			},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.MigratedFromV1() {
		t.Fatal("a v2 config must not report a migration")
	}
	acct := loaded.DefaultAccount()
	if acct == nil || acct.Name != "acme" || acct.ID != 42 || acct.UserID != 7 {
		t.Fatalf("DefaultAccount() = %#v, want acme #42 user 7", acct)
	}
	if acct.HelpCenter.DefaultPortalSlug != "acme-help" || acct.HelpCenter.DefaultLocale != "en" {
		t.Fatalf("help center defaults not round-tripped: %#v", acct.HelpCenter)
	}
}

func TestLoadMigratesV1ConfigInMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	v1 := "base_url: https://app.chatwoot.com/\naccount_id: 123\nuser_id: 7\n" +
		"help_center:\n  default_portal_slug: docs\n  default_locale: en\n"
	writeConfigFile(t, v1)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.MigratedFromV1() {
		t.Fatal("MigratedFromV1() = false for a v1 config")
	}

	acct := cfg.DefaultAccount()
	if acct == nil {
		t.Fatalf("migrated config has no default account: %#v", cfg)
	}
	if acct.BaseURL != "https://app.chatwoot.com" || acct.ID != 123 || acct.UserID != 7 {
		t.Fatalf("migrated account = %#v", acct)
	}
	// The real account name is unknown offline, so the name stays provisional
	// until the first successful API call fills it in.
	if !acct.Provisional || acct.Name != "account-123" {
		t.Fatalf("migrated account should be provisional account-123, got %#v", acct)
	}
	if acct.HelpCenter.DefaultPortalSlug != "docs" || acct.HelpCenter.DefaultLocale != "en" {
		t.Fatalf("help center defaults not migrated onto the account: %#v", acct.HelpCenter)
	}

	// Load alone never rewrites the file.
	if got := readConfigFile(t); got != v1 {
		t.Fatalf("Load() rewrote the v1 file:\n%s", got)
	}
}

func TestLoadV1WithoutAccountHasNoDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfigFile(t, "base_url: https://app.chatwoot.com\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DefaultAccount() != nil || len(cfg.Accounts) != 0 {
		t.Fatalf("expected no accounts, got %#v", cfg)
	}
}

func TestSaveBacksUpV1ConfigOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	v1 := "base_url: https://app.chatwoot.com\naccount_id: 1\n"
	path := writeConfigFile(t, v1)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(backup) != v1 {
		t.Fatalf("backup = %q, want original v1 content", backup)
	}
	if !strings.Contains(readConfigFile(t), "version: 2") {
		t.Fatalf("config not rewritten as v2: %s", readConfigFile(t))
	}

	// A later save of a v2 config must not clobber the v1 backup.
	cfg.Default = ""
	if err := Save(cfg); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	backup, _ = os.ReadFile(path + ".bak")
	if string(backup) != v1 {
		t.Fatalf("backup overwritten by a v2 save: %q", backup)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}
	if reloaded.MigratedFromV1() {
		t.Fatal("a saved v2 config must not migrate again")
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Save(&Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	dir, _ := ConfigDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || strings.HasPrefix(e.Name(), ".config-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestLegacyAPIKeyIsIgnoredAndRemovedOnSave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfigFile(t, "base_url: https://app.chatwoot.com\napi_key: plaintext-token\naccount_id: 123\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DefaultAccount() == nil {
		t.Fatalf("Load() returned no default account: %#v", cfg)
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	content := readConfigFile(t)
	if strings.Contains(content, "api_key") || strings.Contains(content, "plaintext-token") {
		t.Fatalf("legacy api key was not removed: %s", content)
	}
}
