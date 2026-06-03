//go:build dev

package config

import (
	"strings"
	"testing"
)

// Guards the dev build profile. Run with `go test -tags dev ./internal/config/`.
// Dev builds must read a separate config file and keyring entry so development
// never touches production credentials.
func TestDevBuildProfile(t *testing.T) {
	if !IsDev {
		t.Fatal("IsDev = false in a dev build")
	}
	if configFileName != "config.dev.yaml" {
		t.Fatalf("configFileName = %q, want config.dev.yaml", configFileName)
	}
	if keyringService != "chatwoot-cli-dev" {
		t.Fatalf("keyringService = %q, want chatwoot-cli-dev", keyringService)
	}

	t.Setenv("HOME", t.TempDir())
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	if !strings.HasSuffix(path, "/config.dev.yaml") {
		t.Fatalf("ConfigPath() = %q, want suffix /config.dev.yaml", path)
	}
}
