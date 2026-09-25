package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	APIKeyEnv = "CHATWOOT_API_KEY"

	// apiKeyKeyringEntry is the single entry written before multi-account
	// support. It is still read (and never deleted on upgrade) so an upgrade or
	// a downgrade keeps working.
	apiKeyKeyringEntry = "api-key"
)

// keyringService is build-profile specific ("chatwoot-cli" for prod,
// "chatwoot-cli-dev" for dev). Namespacing the whole service per profile keeps
// logout's DeleteAll(keyringService) scoped to the active build, so a dev logout
// can't erase a prod login's token and vice versa. See profile_prod.go /
// profile_dev.go.

type CredentialSource string

const (
	CredentialSourceEnvironment CredentialSource = "environment"
	CredentialSourceKeyring     CredentialSource = "keyring"
	CredentialSourceMissing     CredentialSource = "missing"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

// loginCredential is the keyring value for one login (base URL + user).
type loginCredential struct {
	BaseURL string `json:"base_url"`
	UserID  int    `json:"user_id"`
	APIKey  string `json:"api_key"`
}

// savedCredential is the pre-multi-account keyring value in the "api-key" entry.
type savedCredential struct {
	BaseURL   string `json:"base_url"`
	AccountID int    `json:"account_id"`
	APIKey    string `json:"api_key"`
}

// ResolveAPIKey finds the token for an account. YAML config intentionally
// stores only non-secrets. CHATWOOT_API_KEY wins for CI, coding agents, and
// temporary overrides; otherwise the token comes from the keyring entry of the
// login (base URL + user) that owns the account. Chatwoot tokens are
// user-scoped, so one login serves every account that user can see.
func ResolveAPIKey(acct *Account) (string, CredentialSource, error) {
	if apiKey := strings.TrimSpace(os.Getenv(APIKeyEnv)); apiKey != "" {
		return apiKey, CredentialSourceEnvironment, nil
	}

	if acct == nil || normalizeBaseURL(acct.BaseURL) == "" {
		return "", CredentialSourceMissing, missingAPIKeyError()
	}
	baseURL := normalizeBaseURL(acct.BaseURL)

	if acct.UserID > 0 {
		stored, err := keyring.Get(keyringService, loginKeyringEntry(baseURL, acct.UserID))
		if err == nil {
			var credential loginCredential
			if json.Unmarshal([]byte(stored), &credential) == nil && credential.APIKey != "" &&
				credential.BaseURL == baseURL && credential.UserID == acct.UserID {
				return credential.APIKey, CredentialSourceKeyring, nil
			}
		} else if !errors.Is(err, keyring.ErrNotFound) {
			return "", CredentialSourceMissing, fmt.Errorf("failed to read API key from keyring: %w", err)
		}
	}

	// Fall back to the entry written before multi-account support. Copy it to
	// the login entry once the user is known, but keep the original.
	stored, err := keyring.Get(keyringService, apiKeyKeyringEntry)
	if err == nil {
		apiKey, err := parseSavedCredential(stored, baseURL)
		if err != nil {
			return "", CredentialSourceMissing, err
		}
		if acct.UserID > 0 {
			_ = saveLoginCredential(baseURL, acct.UserID, apiKey)
		}
		return apiKey, CredentialSourceKeyring, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, fmt.Errorf("failed to read API key from keyring: %w", err)
	}

	// TODO(v1): remove this legacy key migration after users have had a release
	// cycle to move from URL/account-scoped keyring entries.
	legacy := legacyCredentialKey(baseURL, acct.ID)
	apiKey, err := keyring.Get(keyringService, legacy)
	if err == nil {
		if acct.UserID > 0 {
			err = saveLoginCredential(baseURL, acct.UserID, apiKey)
		} else {
			err = saveV1Credential(baseURL, acct.ID, apiKey)
		}
		if err != nil {
			return "", CredentialSourceMissing, fmt.Errorf("failed to migrate API key in keyring: %w", err)
		}
		_ = keyring.Delete(keyringService, legacy)
		return apiKey, CredentialSourceKeyring, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, fmt.Errorf("failed to read legacy API key from keyring: %w", err)
	}

	return "", CredentialSourceMissing, missingAPIKeyError()
}

// SaveAPIKey stores the token for the login that owns acct.
func SaveAPIKey(acct *Account, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if acct == nil || normalizeBaseURL(acct.BaseURL) == "" || acct.UserID <= 0 {
		return fmt.Errorf("base URL and user ID are required to save API key")
	}
	if err := saveLoginCredential(normalizeBaseURL(acct.BaseURL), acct.UserID, apiKey); err != nil {
		return fmt.Errorf("failed to save API key to keyring: %w", err)
	}
	return nil
}

func saveLoginCredential(baseURL string, userID int, apiKey string) error {
	data, err := json.Marshal(loginCredential{BaseURL: baseURL, UserID: userID, APIKey: apiKey})
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, loginKeyringEntry(baseURL, userID), string(data))
}

func saveV1Credential(baseURL string, accountID int, apiKey string) error {
	data, err := json.Marshal(savedCredential{BaseURL: baseURL, AccountID: accountID, APIKey: apiKey})
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, apiKeyKeyringEntry, string(data))
}

// parseSavedCredential reads the pre-multi-account entry. It was scoped to one
// account, but the token belongs to the user, so only the instance must match.
func parseSavedCredential(stored, baseURL string) (string, error) {
	var credential savedCredential
	if err := json.Unmarshal([]byte(stored), &credential); err != nil {
		return "", credentialScopeMismatchError()
	}
	if credential.APIKey == "" || normalizeBaseURL(credential.BaseURL) != baseURL {
		return "", credentialScopeMismatchError()
	}
	return credential.APIKey, nil
}

// DeleteAPIKeys removes every credential saved by this CLI service. This avoids
// leaving stale keyring entries behind when config.yaml was edited or removed.
func DeleteAPIKeys() error {
	err := keyring.DeleteAll(keyringService)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete API keys from keyring: %w", err)
}

// DeleteBaseURLAPIKeys removes the logins for these users on one instance,
// plus the pre-multi-account entry if it belongs to that instance.
func DeleteBaseURLAPIKeys(baseURL string, userIDs []int) error {
	baseURL = normalizeBaseURL(baseURL)
	for _, userID := range userIDs {
		if err := deleteEntry(loginKeyringEntry(baseURL, userID)); err != nil {
			return err
		}
	}
	stored, err := keyring.Get(keyringService, apiKeyKeyringEntry)
	if err == nil {
		var credential savedCredential
		if json.Unmarshal([]byte(stored), &credential) == nil && normalizeBaseURL(credential.BaseURL) == baseURL {
			return deleteEntry(apiKeyKeyringEntry)
		}
	}
	return nil
}

func deleteEntry(key string) error {
	err := keyring.Delete(keyringService, key)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete API key from keyring: %w", err)
}

func missingAPIKeyError() error {
	return fmt.Errorf("%w; run 'chatwoot auth login' or set %s", ErrAPIKeyNotFound, APIKeyEnv)
}

func credentialScopeMismatchError() error {
	return fmt.Errorf("%w; saved keyring credential does not match configured instance; run 'chatwoot auth login' for this base URL", ErrAPIKeyNotFound)
}

func loginKeyringEntry(baseURL string, userID int) string {
	return fmt.Sprintf("login:%s#%d", normalizeBaseURL(baseURL), userID)
}

func legacyCredentialKey(baseURL string, accountID int) string {
	return fmt.Sprintf("%s/accounts/%d", normalizeBaseURL(baseURL), accountID)
}
