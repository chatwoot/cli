package config

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func initMockKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll(keyringService); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	t.Setenv(APIKeyEnv, "")
}

func TestResolveAPIKeyPrefersEnvironment(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 123}

	if err := SaveAPIKey(cfg, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}
	t.Setenv(APIKeyEnv, "env-token")

	apiKey, source, err := ResolveAPIKey(cfg)
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if apiKey != "env-token" {
		t.Fatalf("apiKey = %q, want env-token", apiKey)
	}
	if source != CredentialSourceEnvironment {
		t.Fatalf("source = %q, want %q", source, CredentialSourceEnvironment)
	}
}

func TestResolveAPIKeyFromKeyring(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com/", AccountID: 124}

	if err := SaveAPIKey(cfg, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	apiKey, source, err := ResolveAPIKey(&Config{BaseURL: "https://app.chatwoot.com", AccountID: 124})
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if apiKey != "keyring-token" {
		t.Fatalf("apiKey = %q, want keyring-token", apiKey)
	}
	if source != CredentialSourceKeyring {
		t.Fatalf("source = %q, want %q", source, CredentialSourceKeyring)
	}
}

func TestResolveAPIKeyMissing(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 125}

	_, source, err := ResolveAPIKey(cfg)
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("ResolveAPIKey() error = %v, want ErrAPIKeyNotFound", err)
	}
	if source != CredentialSourceMissing {
		t.Fatalf("source = %q, want %q", source, CredentialSourceMissing)
	}
}

func TestDeleteAPIKeyRemovesKeyringToken(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 126}

	if err := SaveAPIKey(cfg, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}
	if err := DeleteAPIKey(cfg); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}

	_, _, err := ResolveAPIKey(cfg)
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("ResolveAPIKey() error = %v, want ErrAPIKeyNotFound", err)
	}
}
