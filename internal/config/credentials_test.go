package config

import (
	"encoding/json"
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

	stored, err := keyring.Get(keyringService, apiKeyKeyringEntry)
	if err != nil {
		t.Fatalf("keyring.Get() error = %v", err)
	}
	var credential savedCredential
	if err := json.Unmarshal([]byte(stored), &credential); err != nil {
		t.Fatalf("stored credential is not JSON: %v", err)
	}
	if credential.BaseURL != "https://app.chatwoot.com" || credential.AccountID != 124 || credential.APIKey != "keyring-token" {
		t.Fatalf("stored credential = %#v, want scoped api key", credential)
	}
}

func TestResolveAPIKeyRejectsMismatchedStableCredential(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 124}

	if err := SaveAPIKey(cfg, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	_, source, err := ResolveAPIKey(&Config{BaseURL: "https://evil.example", AccountID: 124})
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("ResolveAPIKey() error = %v, want ErrAPIKeyNotFound", err)
	}
	if source != CredentialSourceMissing {
		t.Fatalf("source = %q, want %q", source, CredentialSourceMissing)
	}
}

func TestResolveAPIKeyRejectsRawStableCredential(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 124}

	if err := keyring.Set(keyringService, apiKeyKeyringEntry, "raw-token"); err != nil {
		t.Fatalf("keyring.Set() error = %v", err)
	}

	_, source, err := ResolveAPIKey(cfg)
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("ResolveAPIKey() error = %v, want ErrAPIKeyNotFound", err)
	}
	if source != CredentialSourceMissing {
		t.Fatalf("source = %q, want %q", source, CredentialSourceMissing)
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
	var credential savedCredential
	if err := json.Unmarshal([]byte(stable), &credential); err != nil {
		t.Fatalf("stable keyring token is not JSON after migration: %v", err)
	}
	if credential.BaseURL != "https://app.chatwoot.com" || credential.AccountID != 127 || credential.APIKey != "legacy-token" {
		t.Fatalf("stable credential = %#v, want migrated scoped credential", credential)
	}
	if _, err := keyring.Get(keyringService, legacyCredentialKey(cfg)); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("legacy token still present after migration, err = %v", err)
	}
}

func TestNamedProfilesUseDistinctKeyringEntries(t *testing.T) {
	initMockKeyring(t)

	work := &Config{BaseURL: "https://work.example", AccountID: 1}
	personal := &Config{BaseURL: "https://personal.example", AccountID: 2}
	if err := SaveAPIKeyFor("work", work, "work-token"); err != nil {
		t.Fatalf("SaveAPIKeyFor(work) error = %v", err)
	}
	if err := SaveAPIKeyFor("personal", personal, "personal-token"); err != nil {
		t.Fatalf("SaveAPIKeyFor(personal) error = %v", err)
	}

	// The default profile keeps the historical "api-key" entry; named profiles
	// are namespaced, so neither collides.
	if entry := keyringEntry(DefaultProfileName); entry != apiKeyKeyringEntry {
		t.Fatalf("default entry = %q, want %q", entry, apiKeyKeyringEntry)
	}
	if entry := keyringEntry("work"); entry == apiKeyKeyringEntry {
		t.Fatalf("named profile entry must differ from the default entry, got %q", entry)
	}

	wk, src, err := ResolveAPIKeyFor("work", work)
	if err != nil || wk != "work-token" || src != CredentialSourceKeyring {
		t.Fatalf("ResolveAPIKeyFor(work) = (%q, %v, %v), want work-token/keyring", wk, src, err)
	}
	pk, _, err := ResolveAPIKeyFor("personal", personal)
	if err != nil || pk != "personal-token" {
		t.Fatalf("ResolveAPIKeyFor(personal) = (%q, %v), want personal-token", pk, err)
	}

	// Removing one profile's token must leave the other intact.
	if err := DeleteAPIKeyFor("work"); err != nil {
		t.Fatalf("DeleteAPIKeyFor(work) error = %v", err)
	}
	if _, _, err := ResolveAPIKeyFor("work", work); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("work token should be gone, err = %v", err)
	}
	if pk, _, err := ResolveAPIKeyFor("personal", personal); err != nil || pk != "personal-token" {
		t.Fatalf("personal token should survive work logout, got (%q, %v)", pk, err)
	}
}

func TestDefaultProfileMapsToLegacyEntry(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 9}

	// Saving the default profile must write the legacy "api-key" entry so that
	// pre-profiles single-instance logins keep resolving.
	if err := SaveAPIKeyFor(DefaultProfileName, cfg, "default-token"); err != nil {
		t.Fatalf("SaveAPIKeyFor(default) error = %v", err)
	}
	if _, err := keyring.Get(keyringService, apiKeyKeyringEntry); err != nil {
		t.Fatalf("default profile token not stored at legacy entry: %v", err)
	}
	// An empty profile name resolves to the default too.
	key, src, err := ResolveAPIKeyFor("", cfg)
	if err != nil || key != "default-token" || src != CredentialSourceKeyring {
		t.Fatalf("ResolveAPIKeyFor(\"\") = (%q, %v, %v), want default-token/keyring", key, src, err)
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

// TestDeleteAPIKeyLeavesOtherProfileServiceIntact guards the dev/prod isolation
// guarantee: keyringService is namespaced per build profile, so logging out of
// one build must not delete the other build's saved token. DeleteAPIKey wipes
// only keyringService, never another service's entries.
func TestDeleteAPIKeyLeavesOtherProfileServiceIntact(t *testing.T) {
	initMockKeyring(t)
	cfg := &Config{BaseURL: "https://app.chatwoot.com", AccountID: 130}

	// Stand in for the other build profile's keyring namespace (prod's
	// "chatwoot-cli" vs dev's "chatwoot-cli-dev"); the exact name doesn't matter,
	// only that it differs from the active keyringService.
	const otherProfileService = "chatwoot-cli-other-profile"
	if err := keyring.Set(otherProfileService, apiKeyKeyringEntry, "other-profile-token"); err != nil {
		t.Fatalf("seed other-profile service: %v", err)
	}
	if err := SaveAPIKey(cfg, "this-profile-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	if err := DeleteAPIKey(cfg); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}

	// Active profile's token is gone...
	if _, _, err := ResolveAPIKey(cfg); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("active profile token survived logout, err = %v", err)
	}
	// ...but the other profile's token must be untouched.
	got, err := keyring.Get(otherProfileService, apiKeyKeyringEntry)
	if err != nil {
		t.Fatalf("other profile token erased by logout: %v", err)
	}
	if got != "other-profile-token" {
		t.Fatalf("other profile token = %q, want other-profile-token", got)
	}
}
