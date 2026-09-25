package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/zalando/go-keyring"
)

func TestConfigViewReportsCredentialSourceWithoutToken(t *testing.T) {
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")

	cfg := saveTestConfig(t, config.Account{BaseURL: "https://app.chatwoot.com", ID: 123, UserID: 7})
	if err := config.SaveAPIKey(cfg.DefaultAccount(), "super-secret-token"); err != nil {
		t.Fatalf("config.SaveAPIKey() error = %v", err)
	}

	var out bytes.Buffer
	printer := output.NewPrinter("text", false, false)
	printer.Writer = &out

	cmd := ConfigViewCmd{}
	if err := cmd.Run(&App{Printer: printer}); err != nil {
		t.Fatalf("ConfigViewCmd.Run() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "super-secret-token") {
		t.Fatalf("config view exposed token: %s", got)
	}
	if !strings.Contains(got, "Credential:") || !strings.Contains(got, "keyring") {
		t.Fatalf("config view did not report keyring credential source: %s", got)
	}
}

func configViewFixture(t *testing.T) *config.Config {
	t.Helper()
	isolateAuthEnv(t)
	cfg := &config.Config{}
	cfg.SyncAccounts("https://app.chatwoot.com", 7, "Shivam", []config.Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})
	cfg.SyncAccounts("https://staging.chatwoot.com", 3, "Shivam", []config.Membership{{ID: 47, Name: "Demo Cast"}})
	cfg.Default = "demo-cast"
	cfg.Find("acme").HelpCenter = config.HelpCenterConfig{DefaultPortalSlug: "acme-help", DefaultLocale: "en"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	// Only the staging login has a saved token.
	if err := config.SaveAPIKey(cfg.Find("demo-cast"), "staging-token"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	return cfg
}

func TestConfigViewShowsEveryAccount(t *testing.T) {
	configViewFixture(t)
	path, _ := config.ConfigPath()

	out, err := runCapture(t, "text", false, &App{}, (&ConfigViewCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{
		"Config file:", path,
		"Default:", "demo-cast",
		"demo-cast (default)", "https://staging.chatwoot.com", "47", "Demo Cast",
		"chatwoot", "acme", "https://app.chatwoot.com", "Acme",
		"Shivam (#7)", "Shivam (#3)",
		"Help center:", "acme-help (en)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("config view missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "staging-token") {
		t.Fatalf("config view exposed a token:\n%s", out)
	}

	// Credentials are reported per login: staging has one, app does not.
	staging := out[strings.Index(out, "demo-cast (default)"):]
	app := out[strings.Index(out, "\nchatwoot"):strings.Index(out, "demo-cast (default)")]
	if !strings.Contains(staging, "keyring") || !strings.Contains(app, "missing") {
		t.Fatalf("per-login credential status wrong:\n%s", out)
	}
}

func TestConfigViewWithoutDefaultStillListsAccounts(t *testing.T) {
	cfg := configViewFixture(t)
	cfg.Default = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	out, err := runCapture(t, "text", false, &App{}, (&ConfigViewCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "No configuration found") || !strings.Contains(out, "acme") || !strings.Contains(out, "chatwoot use <name>") {
		t.Fatalf("config view without a default:\n%s", out)
	}
}

func TestConfigViewJSON(t *testing.T) {
	configViewFixture(t)

	out, err := runCapture(t, "json", false, &App{}, (&ConfigViewCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got struct {
		Path     string `json:"path"`
		Default  string `json:"default"`
		Accounts []struct {
			Name       string `json:"name"`
			ID         int    `json:"id"`
			Default    bool   `json:"default"`
			Credential string `json:"credential"`
			HelpCenter struct {
				Portal string `json:"portal"`
				Locale string `json:"locale"`
			} `json:"help_center"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Path == "" || got.Default != "demo-cast" || len(got.Accounts) != 3 {
		t.Fatalf("json = %+v", got)
	}
	acme := got.Accounts[1]
	if acme.Name != "acme" || acme.HelpCenter.Portal != "acme-help" || acme.Credential != "missing" || acme.Default {
		t.Fatalf("acme = %+v", acme)
	}
	if demo := got.Accounts[2]; !demo.Default || demo.Credential != "keyring" {
		t.Fatalf("demo-cast = %+v", demo)
	}
}

func TestConfigViewWithoutConfig(t *testing.T) {
	isolateAuthEnv(t)
	out, err := runCapture(t, "text", false, &App{}, (&ConfigViewCmd{}).Run)
	if err != nil || !strings.Contains(out, "chatwoot auth login") {
		t.Fatalf("output = %q, err = %v", out, err)
	}
}
