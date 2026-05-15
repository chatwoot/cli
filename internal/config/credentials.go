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
	keyringService     = "chatwoot-cli"
	apiKeyKeyringEntry = "api-key"
)

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

// ResolveAPIKey implements the auth flow for the CLI. YAML config intentionally
// stores only non-secrets, and plaintext api_key values from older configs are
// ignored. CHATWOOT_API_KEY wins for CI, coding agents, and temporary overrides;
// otherwise saved interactive logins read the token from the OS keyring.
func ResolveAPIKey(cfg *Config) (string, CredentialSource, error) {
	if apiKey := strings.TrimSpace(os.Getenv(APIKeyEnv)); apiKey != "" {
		return apiKey, CredentialSourceEnvironment, nil
	}

	if cfg == nil || !cfg.IsValid() {
		return "", CredentialSourceMissing, missingAPIKeyError()
	}

	stored, err := keyring.Get(keyringService, apiKeyKeyringEntry)
	if err == nil {
		apiKey, err := parseSavedCredential(stored, cfg)
		if err != nil {
			return "", CredentialSourceMissing, err
		}
		return apiKey, CredentialSourceKeyring, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, fmt.Errorf("failed to read API key from keyring: %w", err)
	}

	// TODO(v1): remove this legacy key migration after users have had a release
	// cycle to move from URL/account-scoped keyring entries to api-key.
	apiKey, err := keyring.Get(keyringService, legacyCredentialKey(cfg))
	if err == nil {
		if err := saveAPIKeyToKeyring(cfg, apiKey); err != nil {
			return "", CredentialSourceMissing, fmt.Errorf("failed to migrate API key in keyring: %w", err)
		}
		_ = keyring.Delete(keyringService, legacyCredentialKey(cfg))
		return apiKey, CredentialSourceKeyring, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, fmt.Errorf("failed to read legacy API key from keyring: %w", err)
	}

	return "", CredentialSourceMissing, missingAPIKeyError()
}

func SaveAPIKey(cfg *Config, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if cfg == nil || !cfg.IsValid() {
		return fmt.Errorf("valid config is required to save API key")
	}
	if err := saveAPIKeyToKeyring(cfg, apiKey); err != nil {
		return fmt.Errorf("failed to save API key to keyring: %w", err)
	}
	return nil
}

func saveAPIKeyToKeyring(cfg *Config, apiKey string) error {
	credential := savedCredential{
		BaseURL:   normalizeBaseURL(cfg.BaseURL),
		AccountID: cfg.AccountID,
		APIKey:    apiKey,
	}

	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, apiKeyKeyringEntry, string(data))
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

// DeleteAPIKey removes every credential saved by this CLI service. This avoids
// leaving stale keyring entries behind when config.yaml was edited or removed.
func DeleteAPIKey(_ *Config) error {
	err := keyring.DeleteAll(keyringService)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("failed to delete API keys from keyring: %w", err)
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
