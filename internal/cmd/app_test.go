package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/zalando/go-keyring"
)

// TestNewAppSkipAuthToleratesUnreadableConfig guards the recovery path: a
// corrupt config.yaml must not stop skip-auth commands (auth login, config,
// profile, version) from running, since those are how a user fixes it.
func TestNewAppSkipAuthToleratesUnreadableConfig(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".chatwoot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path, err := config.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if err := os.WriteFile(path, []byte("{ this: is not: valid yaml ["), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	app, err := NewApp(&CLI{Output: "text"}, true, "test")
	if err != nil {
		t.Fatalf("skip-auth NewApp must not fail on a corrupt config, got: %v", err)
	}
	if app == nil || app.Printer == nil {
		t.Fatal("skip-auth NewApp returned an unusable App")
	}
}

func TestNewAppSkipAuthCarriesProfileFlag(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())

	app, err := NewApp(&CLI{Output: "text", Profile: "staging"}, true, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.ProfileName != "staging" {
		t.Fatalf("ProfileName = %q, want staging", app.ProfileName)
	}
}
