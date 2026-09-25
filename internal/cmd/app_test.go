package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/sdk"
)

// writeV1Config writes a config file in the format used before multi-account
// support, as an upgrading user would have it.
func writeV1Config(t *testing.T, content string) string {
	t.Helper()
	path, err := config.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// Upgrading from a single-account release must just work: same account, same
// token, no prompt, and the config quietly rewritten in the new format.
func TestNewAppUpgradesV1ConfigTransparently(t *testing.T) {
	isolateAuthEnv(t)
	path := writeV1Config(t, "base_url: https://app.chatwoot.com\naccount_id: 42\nuser_id: 7\n")
	seedV1Keyring(t, "https://app.chatwoot.com", 42, "v1-token")

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp after upgrade: %v", err)
	}
	if app.Client.BaseURL != "https://app.chatwoot.com" || app.Client.AccountID != 42 || app.Client.APIKey != "v1-token" {
		t.Fatalf("client = %s #%d key %q, want the v1 login", app.Client.BaseURL, app.Client.AccountID, app.Client.APIKey)
	}

	saved, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.MigratedFromV1() {
		t.Fatal("migrated config was not saved in the new format")
	}
	if def := saved.DefaultAccount(); def == nil || def.ID != 42 || def.UserID != 7 {
		t.Fatalf("default account = %#v, want account 42 for user 7", def)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("v1 config not backed up: %v", err)
	}
}

// If the migrated config can't be written, the command still runs on the
// in-memory copy and the migration is retried next time.
func TestNewAppRunsWhenMigrationSaveFails(t *testing.T) {
	isolateAuthEnv(t)
	v1 := "base_url: https://app.chatwoot.com\naccount_id: 42\n"
	path := writeV1Config(t, v1)
	seedV1Keyring(t, "https://app.chatwoot.com", 42, "v1-token")

	saveConfig = func(*config.Config) error { return errors.New("disk full") }
	t.Cleanup(func() { saveConfig = config.Save })

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp must not fail when the migration can't be saved: %v", err)
	}
	if app.Client.AccountID != 42 || app.Client.APIKey != "v1-token" {
		t.Fatalf("client = #%d key %q, want the v1 login", app.Client.AccountID, app.Client.APIKey)
	}
	if got, _ := os.ReadFile(path); string(got) != v1 {
		t.Fatalf("v1 config changed despite the failed save: %q", got)
	}
}

func TestNewAppWithoutConfigAsksToLogIn(t *testing.T) {
	isolateAuthEnv(t)
	if _, err := NewApp(&CLI{Output: "text"}, false, "test"); err == nil {
		t.Fatal("NewApp without a config succeeded")
	}
}

// saveTwoLogins registers accounts on two instances with separate tokens.
func saveTwoLogins(t *testing.T) *config.Config {
	t.Helper()
	// These instances are real domains; an unknown-name refresh must never
	// reach them from a test.
	fetchProfile = func(*sdk.Client) (*sdk.ProfileResponse, error) { return nil, errors.New("offline") }
	t.Cleanup(func() { fetchProfile = func(c *sdk.Client) (*sdk.ProfileResponse, error) { return c.Profile().Get() } })
	cfg := &config.Config{}
	cfg.SyncAccounts("https://app.chatwoot.com", 7, "Shivam", []config.Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})
	cfg.SyncAccounts("https://staging.chatwoot.com", 3, "Shivam", []config.Membership{{ID: 1, Name: "Chatwoot"}})
	cfg.Default = "chatwoot"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	for _, name := range []string{"chatwoot", "chatwoot-staging"} {
		acct := cfg.Find(name)
		if err := config.SaveAPIKey(acct, name+"-token"); err != nil {
			t.Fatalf("SaveAPIKey(%s): %v", name, err)
		}
	}
	return cfg
}

func TestNewAppUsesSelectedAccount(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	cases := []struct {
		selector string
		baseURL  string
		id       int
		token    string
	}{
		{"", "https://app.chatwoot.com", 1, "chatwoot-token"},
		{"acme", "https://app.chatwoot.com", 42, "chatwoot-token"},
		{"chatwoot-s", "https://staging.chatwoot.com", 1, "chatwoot-staging-token"},
		{"42", "https://app.chatwoot.com", 42, "chatwoot-token"},
		{"99", "https://app.chatwoot.com", 99, "chatwoot-token"},
	}
	for _, tc := range cases {
		app, err := NewApp(&CLI{Output: "text", Account: tc.selector}, false, "test")
		if err != nil {
			t.Fatalf("NewApp(%q): %v", tc.selector, err)
		}
		c := app.Client
		if c.BaseURL != tc.baseURL || c.AccountID != tc.id || c.APIKey != tc.token {
			t.Errorf("NewApp(%q) client = %s #%d %q, want %s #%d %q",
				tc.selector, c.BaseURL, c.AccountID, c.APIKey, tc.baseURL, tc.id, tc.token)
		}
	}
}

func TestNewAppRejectsUnknownOrAmbiguousAccount(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	for _, selector := range []string{"nope", "chat"} {
		if _, err := NewApp(&CLI{Output: "text", Account: selector}, false, "test"); err == nil {
			t.Errorf("NewApp(%q) succeeded, want an error", selector)
		}
	}
}

func TestNewAppWithoutDefaultAccountExplainsHowToPick(t *testing.T) {
	isolateAuthEnv(t)
	cfg := saveTwoLogins(t)
	cfg.Default = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	_, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err == nil || !strings.Contains(err.Error(), "chatwoot use <name>") {
		t.Fatalf("NewApp without a default = %v, want a hint to run chatwoot use", err)
	}
}
