package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/zalando/go-keyring"
)

// seedProfiles isolates HOME + the keyring and writes two profiles, with "work"
// as the default. Returns nothing; the active build's config file is used.
func seedProfiles(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")
	t.Setenv(config.ProfileEnv, "")

	if err := config.SaveProfile("work", &config.Config{BaseURL: "https://work.example", AccountID: 1}); err != nil {
		t.Fatalf("seed work: %v", err)
	}
	if err := config.SaveProfile("personal", &config.Config{BaseURL: "https://personal.example", AccountID: 2}); err != nil {
		t.Fatalf("seed personal: %v", err)
	}
}

func runProfileCmd(t *testing.T, format string, run func(*App) error) string {
	t.Helper()
	var out bytes.Buffer
	printer := output.NewPrinter(format, false, false)
	printer.Writer = &out
	if err := run(&App{Printer: printer}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.String()
}

func TestProfilesListMarksDefault(t *testing.T) {
	seedProfiles(t)

	got := runProfileCmd(t, "text", (&ProfilesCmd{}).Run)

	for _, want := range []string{"work", "personal", "https://work.example", "https://personal.example"} {
		if !strings.Contains(got, want) {
			t.Fatalf("profiles output missing %q:\n%s", want, got)
		}
	}
	// The default profile (work, first saved) is marked.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "work") && !strings.Contains(line, "*") {
			t.Fatalf("default profile row not marked:\n%s", got)
		}
	}
}

func TestProfilesListEmpty(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.ProfileEnv, "")

	got := runProfileCmd(t, "text", (&ProfilesCmd{}).Run)
	if !strings.Contains(got, "No profiles configured") {
		t.Fatalf("expected empty-state message, got: %s", got)
	}
}

func TestProfileUseSwitchesDefault(t *testing.T) {
	seedProfiles(t)

	_ = runProfileCmd(t, "text", (&ProfileUseCmd{Name: "personal"}).Run)

	store, err := config.LoadStore()
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.DefaultProfile != "personal" {
		t.Fatalf("DefaultProfile = %q, want personal", store.DefaultProfile)
	}
}

func TestProfileUseUnknownErrors(t *testing.T) {
	seedProfiles(t)

	printer := output.NewPrinter("text", false, false)
	err := (&ProfileUseCmd{Name: "ghost"}).Run(&App{Printer: printer})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected error naming the unknown profile, got %v", err)
	}
}

func TestProfileRemoveDeletesProfileAndToken(t *testing.T) {
	seedProfiles(t)

	// Give personal a stored token so we can prove removal clears it.
	if err := config.SaveAPIKeyFor("personal", &config.Config{BaseURL: "https://personal.example", AccountID: 2}, "tok"); err != nil {
		t.Fatalf("SaveAPIKeyFor: %v", err)
	}

	_ = runProfileCmd(t, "text", (&ProfileRemoveCmd{Name: "personal"}).Run)

	store, err := config.LoadStore()
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.Get("personal") != nil {
		t.Fatal("personal profile should be removed")
	}
	if _, err := keyring.Get("chatwoot-cli", "profile:personal"); err == nil {
		t.Fatal("personal profile token should be deleted from keyring")
	}
	// work survives.
	if store.Get("work") == nil {
		t.Fatal("work profile should survive removing personal")
	}
}

func TestProfileShowReportsDefaultFlag(t *testing.T) {
	seedProfiles(t)

	got := runProfileCmd(t, "text", (&ProfileShowCmd{Name: "work"}).Run)
	for _, want := range []string{"Profile:", "work", "Default:", "yes", "https://work.example"} {
		if !strings.Contains(got, want) {
			t.Fatalf("profile show output missing %q:\n%s", want, got)
		}
	}
}
