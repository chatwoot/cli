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
	if !strings.Contains(got, "Writes:") || !strings.Contains(got, "disabled") {
		t.Fatalf("config view did not report disabled writes by default: %s", got)
	}
}

func TestConfigWritesTogglePersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{BaseURL: "https://app.chatwoot.com", AccountID: 123}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	var out bytes.Buffer
	printer := output.NewPrinter("text", false, false)
	printer.Writer = &out

	if err := (&ConfigWritesOnCmd{}).Run(&App{Printer: printer}); err != nil {
		t.Fatalf("ConfigWritesOnCmd.Run() error = %v", err)
	}
	enabled, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if !enabled.WritesEnabled {
		t.Fatal("writes should be enabled")
	}
	if !strings.Contains(out.String(), "Writes enabled") {
		t.Fatalf("enable output = %q", out.String())
	}

	out.Reset()
	if err := (&ConfigWritesOffCmd{}).Run(&App{Printer: printer}); err != nil {
		t.Fatalf("ConfigWritesOffCmd.Run() error = %v", err)
	}
	disabled, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if disabled.WritesEnabled {
		t.Fatal("writes should be disabled")
	}
	if !strings.Contains(out.String(), "Writes disabled") {
		t.Fatalf("disable output = %q", out.String())
	}
}
