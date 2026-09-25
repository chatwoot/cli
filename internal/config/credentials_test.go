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

// seedV1Credential writes the keyring entry format used before multi-account
// support: one stable "api-key" entry scoped to a base URL + account ID.
func seedV1Credential(t *testing.T, baseURL string, accountID int, apiKey string) {
	t.Helper()
	data, err := json.Marshal(savedCredential{BaseURL: baseURL, AccountID: accountID, APIKey: apiKey})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := keyring.Set(keyringService, apiKeyKeyringEntry, string(data)); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}
}

func TestResolveAPIKeyPrefersEnvironment(t *testing.T) {
	initMockKeyring(t)
	acct := &Account{BaseURL: appURL, ID: 123, UserID: 7}

	if err := SaveAPIKey(acct, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}
	t.Setenv(APIKeyEnv, "env-token")

	apiKey, source, err := ResolveAPIKey(acct)
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if apiKey != "env-token" || source != CredentialSourceEnvironment {
		t.Fatalf("ResolveAPIKey() = (%q, %q), want env-token/environment", apiKey, source)
	}
}

func TestResolveAPIKeyFromKeyring(t *testing.T) {
	initMockKeyring(t)
	if err := SaveAPIKey(&Account{BaseURL: appURL + "/", ID: 124, UserID: 7}, "keyring-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	apiKey, source, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 124, UserID: 7})
	if err != nil {
		t.Fatalf("ResolveAPIKey() error = %v", err)
	}
	if apiKey != "keyring-token" || source != CredentialSourceKeyring {
		t.Fatalf("ResolveAPIKey() = (%q, %q), want keyring-token/keyring", apiKey, source)
	}
}

// A Chatwoot access token belongs to a user, so one saved login serves every
// account that user can see on the same instance.
func TestResolveAPIKeySharesLoginAcrossAccountsOnSameInstance(t *testing.T) {
	initMockKeyring(t)
	if err := SaveAPIKey(&Account{BaseURL: appURL, ID: 1, UserID: 7}, "user-7-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	apiKey, _, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 42, UserID: 7})
	if err != nil || apiKey != "user-7-token" {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want user-7-token", apiKey, err)
	}
}

func TestResolveAPIKeySeparatesLogins(t *testing.T) {
	initMockKeyring(t)
	logins := map[string]*Account{
		"app-user-7":     {BaseURL: appURL, ID: 1, UserID: 7},
		"app-user-19":    {BaseURL: appURL, ID: 1, UserID: 19},
		"staging-user-7": {BaseURL: stagingURL, ID: 1, UserID: 7},
	}
	for token, acct := range logins {
		if err := SaveAPIKey(acct, token); err != nil {
			t.Fatalf("SaveAPIKey(%s) error = %v", token, err)
		}
	}
	for token, acct := range logins {
		got, _, err := ResolveAPIKey(acct)
		if err != nil || got != token {
			t.Fatalf("ResolveAPIKey(%#v) = (%q, %v), want %q", acct, got, err, token)
		}
	}
}

func TestSaveAPIKeyRequiresLoginIdentity(t *testing.T) {
	initMockKeyring(t)
	if err := SaveAPIKey(&Account{BaseURL: appURL, ID: 1}, "token"); err == nil {
		t.Fatal("SaveAPIKey without a user ID succeeded")
	}
	if err := SaveAPIKey(&Account{ID: 1, UserID: 7}, "token"); err == nil {
		t.Fatal("SaveAPIKey without a base URL succeeded")
	}
	if err := SaveAPIKey(&Account{BaseURL: appURL, ID: 1, UserID: 7}, "  "); err == nil {
		t.Fatal("SaveAPIKey with a blank token succeeded")
	}
}

// Upgrading must keep working with the token saved by the previous release,
// and copy (not move) it so the old entry survives.
func TestResolveAPIKeyCopiesV1CredentialToLoginEntry(t *testing.T) {
	initMockKeyring(t)
	seedV1Credential(t, appURL, 1, "v1-token")
	acct := &Account{BaseURL: appURL, ID: 1, UserID: 7}

	apiKey, source, err := ResolveAPIKey(acct)
	if err != nil || apiKey != "v1-token" || source != CredentialSourceKeyring {
		t.Fatalf("ResolveAPIKey() = (%q, %q, %v), want v1-token/keyring", apiKey, source, err)
	}
	if _, err := keyring.Get(keyringService, loginKeyringEntry(appURL, 7)); err != nil {
		t.Fatalf("v1 token not copied to the login entry: %v", err)
	}
	if _, err := keyring.Get(keyringService, apiKeyKeyringEntry); err != nil {
		t.Fatalf("v1 entry must be kept after copying: %v", err)
	}
}

func TestResolveAPIKeyUsesV1CredentialWithoutUserID(t *testing.T) {
	initMockKeyring(t)
	seedV1Credential(t, appURL, 1, "v1-token")

	apiKey, _, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 1})
	if err != nil || apiKey != "v1-token" {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want v1-token", apiKey, err)
	}
}

// The v1 entry was scoped to one account, but the token is user-scoped, so
// `-a <other id>` on the same instance keeps working after the upgrade.
func TestResolveAPIKeyAcceptsV1CredentialForOtherAccountOnSameInstance(t *testing.T) {
	initMockKeyring(t)
	seedV1Credential(t, appURL, 1, "v1-token")

	apiKey, _, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 2})
	if err != nil || apiKey != "v1-token" {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want v1-token", apiKey, err)
	}
}

func TestResolveAPIKeyRejectsV1CredentialForOtherInstance(t *testing.T) {
	initMockKeyring(t)
	seedV1Credential(t, appURL, 124, "v1-token")

	_, source, err := ResolveAPIKey(&Account{BaseURL: "https://evil.example", ID: 124})
	if !errors.Is(err, ErrAPIKeyNotFound) || source != CredentialSourceMissing {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want ErrAPIKeyNotFound", source, err)
	}
}

func TestResolveAPIKeyRejectsRawV1Credential(t *testing.T) {
	initMockKeyring(t)
	if err := keyring.Set(keyringService, apiKeyKeyringEntry, "raw-token"); err != nil {
		t.Fatalf("keyring.Set() error = %v", err)
	}

	_, source, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 124})
	if !errors.Is(err, ErrAPIKeyNotFound) || source != CredentialSourceMissing {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want ErrAPIKeyNotFound", source, err)
	}
}

func TestResolveAPIKeyMigratesLegacyKeyringToken(t *testing.T) {
	initMockKeyring(t)
	legacy := legacyCredentialKey(appURL+"/", 127)
	if err := keyring.Set(keyringService, legacy, "legacy-token"); err != nil {
		t.Fatalf("keyring.Set() error = %v", err)
	}

	apiKey, source, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 127, UserID: 7})
	if err != nil || apiKey != "legacy-token" || source != CredentialSourceKeyring {
		t.Fatalf("ResolveAPIKey() = (%q, %q, %v), want legacy-token/keyring", apiKey, source, err)
	}
	if _, err := keyring.Get(keyringService, loginKeyringEntry(appURL, 7)); err != nil {
		t.Fatalf("legacy token not migrated to the login entry: %v", err)
	}
	if _, err := keyring.Get(keyringService, legacy); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("legacy token still present after migration, err = %v", err)
	}
}

func TestResolveAPIKeyMissing(t *testing.T) {
	initMockKeyring(t)

	_, source, err := ResolveAPIKey(&Account{BaseURL: appURL, ID: 125, UserID: 7})
	if !errors.Is(err, ErrAPIKeyNotFound) || source != CredentialSourceMissing {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want ErrAPIKeyNotFound", source, err)
	}
	if _, _, err := ResolveAPIKey(nil); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("ResolveAPIKey(nil) error = %v, want ErrAPIKeyNotFound", err)
	}
}

func TestDeleteAPIKeysRemovesAllServiceEntries(t *testing.T) {
	initMockKeyring(t)
	if err := SaveAPIKey(&Account{BaseURL: appURL, ID: 1, UserID: 7}, "token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}
	seedV1Credential(t, appURL, 1, "v1-token")
	if err := keyring.Set(keyringService, "stale-entry", "stale-token"); err != nil {
		t.Fatalf("keyring.Set(stale) error = %v", err)
	}

	if err := DeleteAPIKeys(); err != nil {
		t.Fatalf("DeleteAPIKeys() error = %v", err)
	}

	for _, key := range []string{loginKeyringEntry(appURL, 7), apiKeyKeyringEntry, "stale-entry"} {
		if _, err := keyring.Get(keyringService, key); !errors.Is(err, keyring.ErrNotFound) {
			t.Fatalf("%s still present after DeleteAPIKeys, err = %v", key, err)
		}
	}
}

func TestDeleteBaseURLAPIKeysLeavesOtherInstances(t *testing.T) {
	initMockKeyring(t)
	app := &Account{BaseURL: appURL, ID: 1, UserID: 7}
	staging := &Account{BaseURL: stagingURL, ID: 1, UserID: 3}
	for _, acct := range []*Account{app, staging} {
		if err := SaveAPIKey(acct, "token"); err != nil {
			t.Fatalf("SaveAPIKey() error = %v", err)
		}
	}
	seedV1Credential(t, appURL, 1, "v1-token")

	if err := DeleteBaseURLAPIKeys(appURL+"/", []int{7}); err != nil {
		t.Fatalf("DeleteBaseURLAPIKeys() error = %v", err)
	}

	if _, _, err := ResolveAPIKey(app); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("app login survived, err = %v", err)
	}
	if _, _, err := ResolveAPIKey(staging); err != nil {
		t.Fatalf("staging login removed: %v", err)
	}
}

// TestDeleteAPIKeysLeavesOtherProfileServiceIntact guards the dev/prod
// isolation guarantee: keyringService is namespaced per build profile, so
// logging out of one build must not delete the other build's saved token.
func TestDeleteAPIKeysLeavesOtherProfileServiceIntact(t *testing.T) {
	initMockKeyring(t)
	acct := &Account{BaseURL: appURL, ID: 130, UserID: 7}

	// Stand in for the other build profile's keyring namespace (prod's
	// "chatwoot-cli" vs dev's "chatwoot-cli-dev"); the exact name doesn't matter,
	// only that it differs from the active keyringService.
	const otherProfileService = "chatwoot-cli-other-profile"
	if err := keyring.Set(otherProfileService, apiKeyKeyringEntry, "other-profile-token"); err != nil {
		t.Fatalf("seed other-profile service: %v", err)
	}
	if err := SaveAPIKey(acct, "this-profile-token"); err != nil {
		t.Fatalf("SaveAPIKey() error = %v", err)
	}

	if err := DeleteAPIKeys(); err != nil {
		t.Fatalf("DeleteAPIKeys() error = %v", err)
	}

	if _, _, err := ResolveAPIKey(acct); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("active profile token survived logout, err = %v", err)
	}
	got, err := keyring.Get(otherProfileService, apiKeyKeyringEntry)
	if err != nil || got != "other-profile-token" {
		t.Fatalf("other profile token = (%q, %v), want other-profile-token", got, err)
	}
}

// The pre-upgrade keyring entry kept the host as the user typed it.
func TestResolveAPIKeyMatchesV1CredentialRegardlessOfHostCase(t *testing.T) {
	initMockKeyring(t)
	seedV1Credential(t, "https://App.Chatwoot.com/", 1, "v1-token")

	apiKey, _, err := ResolveAPIKey(&Account{BaseURL: "https://app.chatwoot.com", ID: 1, UserID: 7})
	if err != nil || apiKey != "v1-token" {
		t.Fatalf("ResolveAPIKey() = (%q, %v), want v1-token", apiKey, err)
	}
}
