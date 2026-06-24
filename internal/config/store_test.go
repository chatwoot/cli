package config

import (
	"os"
	"strings"
	"testing"
)

func TestActiveNamePrecedence(t *testing.T) {
	// No env set for this test; flag override beats default beats "default".
	t.Setenv(ProfileEnv, "")

	s := &Store{DefaultProfile: "cloud"}
	if got := s.ActiveName("flagwins"); got != "flagwins" {
		t.Fatalf("override should win, got %q", got)
	}
	if got := s.ActiveName(""); got != "cloud" {
		t.Fatalf("default should win when no override/env, got %q", got)
	}

	t.Setenv(ProfileEnv, "envwins")
	if got := s.ActiveName(""); got != "envwins" {
		t.Fatalf("env should beat default, got %q", got)
	}
	if got := s.ActiveName("flagwins"); got != "flagwins" {
		t.Fatalf("override should beat env, got %q", got)
	}

	empty := &Store{}
	t.Setenv(ProfileEnv, "")
	if got := empty.ActiveName(""); got != DefaultProfileName {
		t.Fatalf("fallback should be %q, got %q", DefaultProfileName, got)
	}
}

func TestLoadStoreMigratesLegacyFlatConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	legacy := "base_url: https://app.chatwoot.com\naccount_id: 123\nuser_id: 7\n" +
		"help_center:\n  default_portal_slug: kb\n  default_locale: en\n"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}
	if store.DefaultProfile != DefaultProfileName {
		t.Fatalf("DefaultProfile = %q, want %q", store.DefaultProfile, DefaultProfileName)
	}
	cfg := store.Get(DefaultProfileName)
	if cfg == nil {
		t.Fatal("legacy flat config was not migrated into the default profile")
	}
	if cfg.BaseURL != "https://app.chatwoot.com" || cfg.AccountID != 123 || cfg.UserID != 7 {
		t.Fatalf("migrated cfg = %#v, want base_url/account/user preserved", cfg)
	}
	if cfg.HelpCenter.DefaultPortalSlug != "kb" || cfg.HelpCenter.DefaultLocale != "en" {
		t.Fatalf("migrated help center = %#v, want preserved", cfg.HelpCenter)
	}

	// Re-saving must produce the nested profile layout, not the flat one.
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "profiles:") || !strings.Contains(string(data), "default:") {
		t.Fatalf("saved config not in profile layout: %s", string(data))
	}
}

func TestSaveProfileRoundTripAndIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(ProfileEnv, "")

	work := &Config{BaseURL: "https://work.example", AccountID: 1}
	personal := &Config{BaseURL: "https://personal.example", AccountID: 2}
	if err := SaveProfile("work", work); err != nil {
		t.Fatalf("SaveProfile(work) error = %v", err)
	}
	if err := SaveProfile("personal", personal); err != nil {
		t.Fatalf("SaveProfile(personal) error = %v", err)
	}

	// First profile saved becomes the default.
	store, err := LoadStore()
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}
	if store.DefaultProfile != "work" {
		t.Fatalf("DefaultProfile = %q, want work (first saved)", store.DefaultProfile)
	}
	if got := store.Get("personal"); got == nil || got.AccountID != 2 {
		t.Fatalf("personal profile = %#v, want account 2", got)
	}
	if got := store.Get("work"); got == nil || got.AccountID != 1 {
		t.Fatalf("work profile = %#v, want account 1", got)
	}
}

func TestStoreRemovePromotesDefault(t *testing.T) {
	s := &Store{
		DefaultProfile: "work",
		Profiles: map[string]*Config{
			"work":     {BaseURL: "https://work.example", AccountID: 1},
			"personal": {BaseURL: "https://personal.example", AccountID: 2},
		},
	}

	if !s.Remove("work") {
		t.Fatal("Remove(work) should report the profile existed")
	}
	if s.Get("work") != nil {
		t.Fatal("work should be gone after Remove")
	}
	if s.DefaultProfile != "personal" {
		t.Fatalf("DefaultProfile = %q, want personal promoted after removing the default", s.DefaultProfile)
	}
	if s.Remove("nope") {
		t.Fatal("Remove of a missing profile should report false")
	}

	if !s.Remove("personal") {
		t.Fatal("Remove(personal) should report it existed")
	}
	if !s.IsEmpty() {
		t.Fatal("store should be empty after removing all profiles")
	}
}
