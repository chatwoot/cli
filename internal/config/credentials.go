package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	APIKeyEnv      = "CHATWOOT_API_KEY"
	keyringService = "chatwoot-cli"
)

type CredentialSource string

const (
	CredentialSourceEnvironment CredentialSource = "environment"
	CredentialSourceKeyring     CredentialSource = "keyring"
	CredentialSourceMissing     CredentialSource = "missing"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

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

	apiKey, err := keyring.Get(keyringService, credentialKey(cfg))
	if err == nil {
		return apiKey, CredentialSourceKeyring, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", CredentialSourceMissing, missingAPIKeyError()
	}
	return "", CredentialSourceMissing, fmt.Errorf("failed to read API key from keyring: %w", err)
}

func SaveAPIKey(cfg *Config, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if cfg == nil || !cfg.IsValid() {
		return fmt.Errorf("valid config is required to save API key")
	}
	if err := keyring.Set(keyringService, credentialKey(cfg), apiKey); err != nil {
		return fmt.Errorf("failed to save API key to keyring: %w", err)
	}
	return nil
}

func DeleteAPIKey(cfg *Config) error {
	if cfg == nil || !cfg.IsValid() {
		return nil
	}
	if err := keyring.Delete(keyringService, credentialKey(cfg)); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("failed to delete API key from keyring: %w", err)
	}
	return nil
}

func missingAPIKeyError() error {
	return fmt.Errorf("%w; run 'chatwoot auth login' or set %s", ErrAPIKeyNotFound, APIKeyEnv)
}

func credentialKey(cfg *Config) string {
	return fmt.Sprintf("%s/accounts/%d", normalizeBaseURL(cfg.BaseURL), cfg.AccountID)
}

func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}
