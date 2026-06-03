//go:build !dev

package config

import (
	"strings"
	"testing"
)

// Guards the production build profile. Runs under the default (untagged) build;
// the dev counterpart is profile_dev_test.go behind //go:build dev.
func TestProdBuildProfile(t *testing.T) {
	if IsDev {
		t.Fatal("IsDev = true in a non-dev build")
	}
	if configFileName != "config.yaml" {
		t.Fatalf("configFileName = %q, want config.yaml", configFileName)
	}
	if keyringService != "chatwoot-cli" {
		t.Fatalf("keyringService = %q, want chatwoot-cli", keyringService)
	}

	t.Setenv("HOME", t.TempDir())
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
	if !strings.HasSuffix(path, "/config.yaml") {
		t.Fatalf("ConfigPath() = %q, want suffix /config.yaml", path)
	}
}
