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
	APIKeyEnv          = "CHATWOOT_API_KEY"
	apiKeyKeyringEntry = "api-key"
)

// keyringService is build-profile specific ("chatwoot-cli" for prod,
// "chatwoot-cli-dev" for dev); see profile_prod.go / profile_dev.go. Named
// profiles are distinct entries within that service.

type CredentialSource string

const (
	CredentialSourceEnvironment CredentialSource = "environment"
	CredentialSourceKeyring     CredentialSource = "keyring"
	CredentialSourceMissing     CredentialSource = "missing"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

type savedCredential struct {
	BaseURL   string `json:"base_url"`
	AccountID int    `json:"account_id"`
	APIKey    string `json:"api_key"`
}

// keyringEntry maps a profile to its keyring entry. The default profile keeps
// the historical "api-key" entry so pre-profiles logins resolve unchanged;
// named profiles get their own namespaced entry.
func keyringEntry(profile string) string {
	if profile == "" || profile == DefaultProfileName {
		return apiKeyKeyringEntry
	}
	return "profile:" + profile
}

// ResolveAPIKeyFor returns a profile's token. CHATWOOT_API_KEY wins for CI,
// coding agents, and temporary overrides; otherwise the token is read from the
// profile's keyring entry. Plaintext api_key values in YAML are ignored.
func ResolveAPIKeyFor(profile string, cfg *Config) (string, CredentialSource, error) {
	if apiKey := strings.TrimSpace(os.Getenv(APIKeyEnv)); apiKey != "" {
		return apiKey, CredentialSourceEnvironment, nil
	}

	if cfg == nil || !cfg.IsValid() {
		return "", CredentialSourceMissing, missingAPIKeyError()
	}

	entry := keyringEntry(profile)
	stored, err := keyring.Get(keyringService, entry)
	if err == nil {
		apiKey, perr := parseSavedCredential(stored, cfg)
		if perr != nil {
			return "", CredentialSourceMissing, perr
		}
		return apiKey, CredentialSourceKeyring, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, fmt.Errorf("failed to read API key from keyring: %w", err)
	}

	// TODO(v1): drop this once users have migrated. Only the default profile had
	// the older URL/account-scoped entry; migrate it forward to its stable entry.
	if entry == apiKeyKeyringEntry {
		legacyToken, lerr := keyring.Get(keyringService, legacyCredentialKey(cfg))
		if lerr == nil {
			if serr := saveAPIKeyToKeyring(entry, cfg, legacyToken); serr != nil {
				return "", CredentialSourceMissing, fmt.Errorf("failed to migrate API key in keyring: %w", serr)
			}
			_ = keyring.Delete(keyringService, legacyCredentialKey(cfg))
			return legacyToken, CredentialSourceKeyring, nil
		}
		if !errors.Is(lerr, keyring.ErrNotFound) {
			return "", CredentialSourceMissing, fmt.Errorf("failed to read legacy API key from keyring: %w", lerr)
		}
	}

	return "", CredentialSourceMissing, missingAPIKeyError()
}

func SaveAPIKeyFor(profile string, cfg *Config, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if cfg == nil || !cfg.IsValid() {
		return fmt.Errorf("valid config is required to save API key")
	}
	if err := saveAPIKeyToKeyring(keyringEntry(profile), cfg, apiKey); err != nil {
		return fmt.Errorf("failed to save API key to keyring: %w", err)
	}
	return nil
}

// DeleteAPIKeyFor removes a single profile's token, leaving other profiles intact.
func DeleteAPIKeyFor(profile string) error {
	err := keyring.Delete(keyringService, keyringEntry(profile))
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete API key from keyring: %w", err)
}

// ResolveAPIKey and SaveAPIKey operate on the active profile, for callers
// without an explicit profile.
func ResolveAPIKey(cfg *Config) (string, CredentialSource, error) {
	return ResolveAPIKeyFor(ResolveActiveName(""), cfg)
}

func SaveAPIKey(cfg *Config, apiKey string) error {
	return SaveAPIKeyFor(ResolveActiveName(""), cfg, apiKey)
}

// DeleteAPIKey removes every credential under this build's keyring service (a
// full reset); per-profile logout uses DeleteAPIKeyFor.
func DeleteAPIKey(_ *Config) error {
	err := keyring.DeleteAll(keyringService)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete API keys from keyring: %w", err)
}

func saveAPIKeyToKeyring(entry string, cfg *Config, apiKey string) error {
	credential := savedCredential{
		BaseURL:   normalizeBaseURL(cfg.BaseURL),
		AccountID: cfg.AccountID,
		APIKey:    apiKey,
	}

	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, entry, string(data))
}

func parseSavedCredential(stored string, cfg *Config) (string, error) {
	var credential savedCredential
	if err := json.Unmarshal([]byte(stored), &credential); err != nil {
		return "", credentialScopeMismatchError()
	}
	if credential.APIKey == "" ||
		credential.BaseURL != normalizeBaseURL(cfg.BaseURL) ||
		credential.AccountID != cfg.AccountID {
		return "", credentialScopeMismatchError()
	}
	return credential.APIKey, nil
}

func missingAPIKeyError() error {
	return fmt.Errorf("%w; run 'chatwoot auth login' or set %s", ErrAPIKeyNotFound, APIKeyEnv)
}

func credentialScopeMismatchError() error {
	return fmt.Errorf("%w; saved keyring credential does not match configured instance; run 'chatwoot auth login' for this base URL and account", ErrAPIKeyNotFound)
}

func legacyCredentialKey(cfg *Config) string {
	return fmt.Sprintf("%s/accounts/%d", normalizeBaseURL(cfg.BaseURL), cfg.AccountID)
}

func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}
