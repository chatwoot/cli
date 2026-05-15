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

func TestResolveAPIKeyMigratesLegacyKeyringToken(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com/", AccountID: 127}

	if err := keyring.Set(keyringService, legacyCredentialKey(cfg), "legacy-token"); err != nil {
		t.Fatalf("keyring.Set() error = %v", err)
	}

	apiKey, source, err := ResolveAPIKey(&Config{BaseURL: "https://app.chatwoot.com", AccountID: 127})
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if apiKey != "legacy-token" {
		t.Fatalf("apiKey = %q, want legacy-token", apiKey)
	}
	if source != CredentialSourceKeyring {
		t.Fatalf("source = %q, want %q", source, CredentialSourceKeyring)
	}

	stable, err := keyring.Get(keyringService, apiKeyKeyringEntry)
	if err != nil {
		t.Fatalf("stable keyring token missing after migration: %v", err)
	}
	if stable != "legacy-token" {
		t.Fatalf("stable token = %q, want legacy-token", stable)
	}
	if _, err := keyring.Get(keyringService, legacyCredentialKey(cfg)); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("legacy token still present after migration, err = %v", err)
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

func TestDeleteAPIKeyRemovesAllServiceEntries(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 128}

	if err := keyring.Set(keyringService, apiKeyKeyringEntry, "stable-token"); err != nil {
		t.Fatalf("keyring.Set(stable) error = %v", err)
	}
	if err := keyring.Set(keyringService, "stale-entry", "stale-token"); err != nil {
		t.Fatalf("keyring.Set(stale) error = %v", err)
	}

	if err := DeleteAPIKey(cfg); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}

	for name, key := range map[string]string{
		"stable": apiKeyKeyringEntry,
		"stale":  "stale-entry",
	} {
		if _, err := keyring.Get(keyringService, key); !errors.Is(err, keyring.ErrNotFound) {
			t.Fatalf("%s token still present after DeleteAPIKey, err = %v", name, err)
		}
	}
}
