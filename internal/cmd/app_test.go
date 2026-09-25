package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestNewAppLinkToUnknownInstanceExplainsLogin(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = defaultIsInteractive })

	_, err := NewApp(&CLI{Output: "text", Account: "https://eu.chatwoot.com/app/accounts/5"}, false, "test")
	if err == nil || !strings.Contains(err.Error(), "chatwoot auth login https://eu.chatwoot.com") {
		t.Fatalf("NewApp(unknown instance link) = %v, want a login hint", err)
	}
}

// In a terminal, a link to an instance without a login offers to log in and
// then carries on with the command.
func TestNewAppLinkToUnknownInstanceOffersLogin(t *testing.T) {
	isolateAuthEnv(t)
	server := profileServer(t, 5, `[{"id":7,"name":"Acme"},{"id":9,"name":"Beta"}]`)
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = defaultIsInteractive })

	var app *App
	_, err := runLogin(t, nil, "y\ntoken\n", func() error {
		var err error
		app, err = NewApp(&CLI{Output: "text", Account: server.URL + "/app/accounts/9"}, false, "test")
		return err
	})
	if err != nil {
		t.Fatalf("NewApp after offered login: %v", err)
	}
	if app.Client.BaseURL != server.URL || app.Client.AccountID != 9 || app.Client.APIKey != "token" {
		t.Fatalf("client = %s #%d %q, want the linked account", app.Client.BaseURL, app.Client.AccountID, app.Client.APIKey)
	}
	if cfg, _ := config.Load(); cfg.Default != "beta" {
		t.Fatalf("default = %q, want the linked account beta", cfg.Default)
	}
}

func TestNewAppLinkLoginDeclined(t *testing.T) {
	isolateAuthEnv(t)
	server := profileServer(t, 5, `[{"id":7,"name":"Acme"}]`)
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = defaultIsInteractive })

	_, err := runLogin(t, nil, "n\n", func() error {
		_, err := NewApp(&CLI{Output: "text", Account: server.URL + "/app/accounts/7"}, false, "test")
		return err
	})
	var notLoggedIn *config.NotLoggedInError
	if !errors.As(err, &notLoggedIn) {
		t.Fatalf("declined login error = %v, want NotLoggedInError", err)
	}
}

// finishFixture is an upgraded user: a v1 config and keyring entry, pointing at
// a profile server that lists more accounts than the user had configured.
func finishFixture(t *testing.T) (requests *int) {
	t.Helper()
	isolateAuthEnv(t)
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"Shivam","accounts":[{"id":1,"name":"Chatwoot"},{"id":42,"name":"Acme"},{"id":77,"name":"Globex Inc"}]}`))
	}))
	t.Cleanup(server.Close)
	writeV1Config(t, "base_url: "+server.URL+"\naccount_id: 1\n")
	seedV1Keyring(t, server.URL, 1, "v1-token")
	return &count
}

// The first successful run after an upgrade names the migrated account and
// tells the user, once, about their other accounts.
func TestFinishAnnouncesOtherAccountsOnceAfterUpgrade(t *testing.T) {
	requests := finishFixture(t)

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	var notice bytes.Buffer
	app.Finish(&notice, true)

	for _, want := range []string{"You also have access to", "acme #42", "globex-inc #77", "chatwoot @acme convs"} {
		if !strings.Contains(notice.String(), want) {
			t.Fatalf("notice missing %q:\n%s", want, notice.String())
		}
	}
	cfg, _ := config.Load()
	if def := cfg.DefaultAccount(); def == nil || def.Name != "chatwoot" || def.Provisional || def.UserID != 7 {
		t.Fatalf("default after first run = %#v, want chatwoot for user 7", def)
	}
	if cfg.Find("acme") == nil || cfg.Find("globex-inc") == nil {
		t.Fatalf("other accounts not registered: %v", cfg.Accounts)
	}

	// Next run: nothing to announce and no extra request.
	before := *requests
	app, err = NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("second NewApp: %v", err)
	}
	notice.Reset()
	app.Finish(&notice, true)
	if notice.Len() != 0 || *requests != before {
		t.Fatalf("second run announced again (%q) or made %d requests", notice.String(), *requests-before)
	}
}

func TestFinishRegistersQuietlyWhenNoticeNotWanted(t *testing.T) {
	finishFixture(t)

	app, err := NewApp(&CLI{Output: "json"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	var notice bytes.Buffer
	app.Finish(&notice, false)
	if notice.Len() != 0 {
		t.Fatalf("notice printed when not wanted: %q", notice.String())
	}
	if cfg, _ := config.Load(); cfg.Find("acme") == nil {
		t.Fatal("accounts should still be registered")
	}
}

func TestFinishSkipsEnvironmentTokens(t *testing.T) {
	requests := finishFixture(t)
	t.Setenv(config.APIKeyEnv, "someone-elses-token")

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	var notice bytes.Buffer
	app.Finish(&notice, true)
	if *requests != 0 || notice.Len() != 0 {
		t.Fatalf("env-token run synced accounts (%d requests, notice %q)", *requests, notice.String())
	}
}

func TestTargetNotice(t *testing.T) {
	cfg := &config.Config{Default: "chatwoot", Accounts: []config.Account{
		{Name: "chatwoot", BaseURL: "https://app.chatwoot.com", ID: 1},
		{Name: "acme", BaseURL: "https://app.chatwoot.com", ID: 42},
	}}
	def := &App{Config: cfg, Account: cfg.Find("chatwoot")}
	other := &App{Config: cfg, Account: cfg.Find("acme")}
	adhoc := &App{Config: cfg, Account: &config.Account{BaseURL: "https://app.chatwoot.com", ID: 99}}

	cases := []struct {
		app     *App
		command string
		cli     CLI
		want    string
	}{
		{other, "conv reply <id> <text>", CLI{}, "→ acme"},
		{other, "conv resolve <id>", CLI{}, "→ acme"},
		{other, "conv assign <id>", CLI{}, "→ acme"},
		{adhoc, "conv label <id> <labels>", CLI{}, "→ account #99 on app.chatwoot.com"},
		{def, "conv reply <id> <text>", CLI{}, ""}, // default: nothing
		{other, "convs", CLI{}, ""},                // read
		{other, "conv view <id>", CLI{}, ""},       // read
		{other, "api <path>", CLI{}, ""},           // GET
		{other, "api <path>", CLI{Api: ApiCmd{Data: "{}"}}, "→ acme"},
		{other, "api <path>", CLI{Api: ApiCmd{Method: "delete"}}, "→ acme"},
	}
	for _, tc := range cases {
		if got := TargetNotice(tc.app, tc.command, &tc.cli); got != tc.want {
			t.Errorf("TargetNotice(%s, %q) = %q, want %q", tc.app.Account.Name, tc.command, got, tc.want)
		}
	}
}

func TestExplainErrorAddsLoginHintOn401(t *testing.T) {
	app := &App{Account: &config.Account{BaseURL: "https://app.chatwoot.com", ID: 1, UserName: "Shivam"}}
	err := ExplainError(app, fmt.Errorf("list: %w", &sdk.APIError{StatusCode: 401, Body: "Invalid token"}))
	for _, want := range []string{"rejected", "Shivam", "app.chatwoot.com", "chatwoot auth login https://app.chatwoot.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}

	other := errors.New("boom")
	if ExplainError(app, other) != other {
		t.Fatal("non-401 errors must pass through unchanged")
	}
	if ExplainError(&App{}, &sdk.APIError{StatusCode: 401}) == nil {
		t.Fatal("an app without an account must still return the error")
	}
}
