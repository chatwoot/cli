package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// currentVersion is the config schema version written by Save. Version 1 files
// predate the field and hold a single flat base_url/account_id login.
const currentVersion = 2

// Config is the on-disk CLI configuration: every registered account plus the
// name of the default one. Secrets never live here; tokens are in the keyring.
type Config struct {
	Version  int       `yaml:"version"`
	Default  string    `yaml:"default,omitempty"`
	Accounts []Account `yaml:"accounts,omitempty"`

	migratedFromV1 bool
}

// Account is one Chatwoot account reachable through a saved login. A login is
// a base URL + user ID pair; its token is shared by every account it can see.
type Account struct {
	Name        string           `yaml:"name"`
	BaseURL     string           `yaml:"base_url"`
	ID          int              `yaml:"id"`
	UserID      int              `yaml:"user_id,omitempty"`
	UserName    string           `yaml:"user_name,omitempty"`
	AccountName string           `yaml:"account_name,omitempty"`
	HelpCenter  HelpCenterConfig `yaml:"help_center,omitempty"`

	// Provisional marks a name assigned offline during migration. It is replaced
	// by the real account name on the first sync; nobody has typed it yet.
	Provisional bool `yaml:"provisional,omitempty"`
}

type HelpCenterConfig struct {
	DefaultPortalSlug string `yaml:"default_portal_slug,omitempty"`
	DefaultLocale     string `yaml:"default_locale,omitempty"`
}

// legacyConfig is the version 1 schema: one login, one account.
type legacyConfig struct {
	BaseURL    string           `yaml:"base_url"`
	AccountID  int              `yaml:"account_id"`
	UserID     int              `yaml:"user_id"`
	HelpCenter HelpCenterConfig `yaml:"help_center"`
}

func ConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".chatwoot"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the config file. A version 1 file is converted in memory without
// touching disk or network; the caller decides when to Save the result.
// It returns nil, nil when no config file exists.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config file exists
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return parse(data)
}

func parse(data []byte) (*Config, error) {
	var probe struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if probe.Version > currentVersion {
		// Reading it would drop fields this version doesn't know, and the next
		// save would write the damaged config back.
		return nil, fmt.Errorf("config was written by a newer chatwoot (version %d); upgrade the CLI to use it", probe.Version)
	}
	if probe.Version < currentVersion {
		var legacy legacyConfig
		if err := yaml.Unmarshal(data, &legacy); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
		return migrateV1(legacy), nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	for i := range cfg.Accounts {
		cfg.Accounts[i].BaseURL = normalizeBaseURL(cfg.Accounts[i].BaseURL)
	}
	return &cfg, nil
}

// migrateV1 converts a single-login config. The account's real name needs a
// profile request, so it gets a provisional name that the first successful
// sync replaces.
func migrateV1(legacy legacyConfig) *Config {
	cfg := &Config{Version: currentVersion, migratedFromV1: true}
	if strings.TrimSpace(legacy.BaseURL) == "" || legacy.AccountID <= 0 {
		return cfg
	}
	acct := Account{
		Name:        fmt.Sprintf("account-%d", legacy.AccountID),
		BaseURL:     normalizeBaseURL(legacy.BaseURL),
		ID:          legacy.AccountID,
		UserID:      legacy.UserID,
		HelpCenter:  legacy.HelpCenter,
		Provisional: true,
	}
	cfg.Accounts = []Account{acct}
	cfg.Default = acct.Name
	return cfg
}

// MigratedFromV1 reports whether Load converted a version 1 file that has not
// been saved in the new format yet.
func (c *Config) MigratedFromV1() bool {
	return c != nil && c.migratedFromV1
}

// Save writes the config atomically. The first time a version 1 file is
// replaced, it is kept as <config>.bak.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("failed to secure config directory: %w", err)
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	cfg.Version = currentVersion
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := backupV1(path); err != nil {
		return err
	}
	if err := writeFileAtomic(dir, path, data); err != nil {
		return err
	}
	cfg.migratedFromV1 = false
	return nil
}

// backupV1 copies a version 1 config to <path>.bak before it is overwritten,
// unless a backup already exists.
func backupV1(path string) error {
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	var probe struct {
		Version int `yaml:"version"`
	}
	if yaml.Unmarshal(existing, &probe) == nil && probe.Version >= currentVersion {
		return nil
	}
	backup := path + ".bak"
	if _, err := os.Stat(backup); err == nil {
		return nil
	}
	if err := os.WriteFile(backup, existing, 0600); err != nil {
		return fmt.Errorf("failed to back up config: %w", err)
	}
	return nil
}

// writeFileAtomic writes to a temp file in the same directory and renames it
// into place, so concurrent readers never see a partially written config.
func writeFileAtomic(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to secure config file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// DefaultAccount returns the account named by Default, or nil.
func (c *Config) DefaultAccount() *Account {
	if c == nil || c.Default == "" {
		return nil
	}
	return c.Find(c.Default)
}

// Find returns the account with exactly this name, or nil.
func (c *Config) Find(name string) *Account {
	if c == nil {
		return nil
	}
	for i := range c.Accounts {
		if c.Accounts[i].Name == name {
			return &c.Accounts[i]
		}
	}
	return nil
}

// FindByID returns the account with this ID seen by one login, or nil.
func (c *Config) FindByID(baseURL string, userID, id int) *Account {
	if c == nil {
		return nil
	}
	baseURL = normalizeBaseURL(baseURL)
	for i := range c.Accounts {
		a := &c.Accounts[i]
		if a.BaseURL == baseURL && a.UserID == userID && a.ID == id {
			return a
		}
	}
	return nil
}
