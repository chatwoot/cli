package cmd

import (
	"bytes"
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

	cfg := &config.Config{BaseURL: "https://app.chatwoot.com", AccountID: 123}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	if err := config.SaveAPIKey(cfg, "super-secret-token"); err != nil {
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
