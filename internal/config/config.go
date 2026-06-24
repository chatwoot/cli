package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ProfileEnv overrides the active profile for a single invocation.
	ProfileEnv = "CHATWOOT_PROFILE"
	// DefaultProfileName is used when no profile is named. Its credentials keep
	// the historical keyring entry so pre-profiles logins resolve unchanged.
	DefaultProfileName = "default"
)

// Config holds one profile's non-secret settings (a single Chatwoot instance
// and account). The API token lives in the OS keyring, never here.
type Config struct {
	BaseURL    string           `yaml:"base_url"`
	AccountID  int              `yaml:"account_id"`
	UserID     int              `yaml:"user_id,omitempty"`
	HelpCenter HelpCenterConfig `yaml:"help_center,omitempty"`
}

type HelpCenterConfig struct {
	DefaultPortalSlug string `yaml:"default_portal_slug,omitempty"`
	DefaultLocale     string `yaml:"default_locale,omitempty"`
}

// Store is the on-disk config document: named profiles plus the default.
type Store struct {
	DefaultProfile string             `yaml:"default_profile,omitempty"`
	Profiles       map[string]*Config `yaml:"profiles,omitempty"`
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

func LoadStore() (*Store, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Profiles: map[string]*Config{}}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Decode both layouts so a pre-profiles flat config.yaml migrates on load.
	var doc struct {
		DefaultProfile string             `yaml:"default_profile"`
		Profiles       map[string]*Config `yaml:"profiles"`
		BaseURL        string             `yaml:"base_url"`
		AccountID      int                `yaml:"account_id"`
		UserID         int                `yaml:"user_id"`
		HelpCenter     HelpCenterConfig   `yaml:"help_center"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	store := &Store{DefaultProfile: doc.DefaultProfile, Profiles: doc.Profiles}
	if store.Profiles == nil {
		store.Profiles = map[string]*Config{}
	}
	if len(store.Profiles) == 0 && (strings.TrimSpace(doc.BaseURL) != "" || doc.AccountID != 0) {
		store.Profiles[DefaultProfileName] = &Config{
			BaseURL:    doc.BaseURL,
			AccountID:  doc.AccountID,
			UserID:     doc.UserID,
			HelpCenter: doc.HelpCenter,
		}
		if store.DefaultProfile == "" {
			store.DefaultProfile = DefaultProfileName
		}
	}

	return store, nil
}

func (s *Store) Save() error {
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

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to secure config file: %w", err)
	}
	return nil
}

// ActiveName resolves the profile to use: override (flag) → CHATWOOT_PROFILE →
// configured default → "default".
func (s *Store) ActiveName(override string) string {
	switch {
	case strings.TrimSpace(override) != "":
		return strings.TrimSpace(override)
	case strings.TrimSpace(os.Getenv(ProfileEnv)) != "":
		return strings.TrimSpace(os.Getenv(ProfileEnv))
	case s != nil && s.DefaultProfile != "":
		return s.DefaultProfile
	default:
		return DefaultProfileName
	}
}

func (s *Store) Get(name string) *Config {
	if s == nil || s.Profiles == nil {
		return nil
	}
	return s.Profiles[name]
}

func (s *Store) Set(name string, cfg *Config) {
	if s.Profiles == nil {
		s.Profiles = map[string]*Config{}
	}
	s.Profiles[name] = cfg
}

// Remove deletes a profile, promoting another to default if the removed one was
// the default. Reports whether the profile existed.
func (s *Store) Remove(name string) bool {
	if s == nil || s.Profiles == nil {
		return false
	}
	if _, ok := s.Profiles[name]; !ok {
		return false
	}
	delete(s.Profiles, name)
	if s.DefaultProfile == name {
		s.DefaultProfile = ""
		if names := s.Names(); len(names) > 0 {
			s.DefaultProfile = names[0]
		}
	}
	return true
}

func (s *Store) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for n := range s.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (s *Store) IsEmpty() bool { return len(s.Profiles) == 0 }

// ResolveActiveName loads the store and resolves the active profile for an
// override, for callers that hold only the --profile flag value.
func ResolveActiveName(override string) string {
	store, err := LoadStore()
	if err != nil {
		return (&Store{}).ActiveName(override)
	}
	return store.ActiveName(override)
}

// LoadProfile returns the named profile (empty name → active), or nil if absent.
func LoadProfile(name string) (*Config, error) {
	store, err := LoadStore()
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = store.ActiveName("")
	}
	return store.Get(name), nil
}

// SaveProfile persists cfg as the named profile (empty name → active), selecting
// it as default when it is the first profile.
func SaveProfile(name string, cfg *Config) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	if name == "" {
		name = store.ActiveName("")
	}
	first := store.IsEmpty()
	store.Set(name, cfg)
	if first || store.DefaultProfile == "" {
		store.DefaultProfile = name
	}
	return store.Save()
}

// Load and Save operate on the active profile, for callers without an explicit
// profile.
func Load() (*Config, error) { return LoadProfile("") }
func Save(cfg *Config) error { return SaveProfile("", cfg) }

func (c *Config) IsValid() bool {
	return strings.TrimSpace(c.BaseURL) != "" && c.AccountID > 0
}
