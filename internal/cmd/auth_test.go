package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/zalando/go-keyring"
)

// setupAuthStatusEnv isolates HOME and the keyring, stubs the Profile
// endpoint, saves a config pointing at the stub, and authenticates via the
// CHATWOOT_API_KEY env override (so source = "environment").
//
// Returns a cleanup function that closes the stub server.
func setupAuthStatusEnv(t *testing.T, profileBody string) func() {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(profileBody))
	}))

	cfg := &config.Config{BaseURL: server.URL, AccountID: 1}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	t.Setenv(config.APIKeyEnv, "test-token")

	return server.Close
}

func setupAuthStatusKeyring(t *testing.T, profileBody string) func() {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(profileBody))
	}))

	cfg := &config.Config{BaseURL: server.URL, AccountID: 1}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	if err := config.SaveAPIKey(cfg, "test-token"); err != nil {
		t.Fatalf("config.SaveAPIKey: %v", err)
	}

	return server.Close
}

// runAndCapture runs a Cmd's Run with a buffered Printer and returns stdout.
func runAndCapture(t *testing.T, run func(*App) error) string {
	t.Helper()
	var out bytes.Buffer
	printer := output.NewPrinter("text", false, false)
	printer.Writer = &out
	if err := run(&App{Printer: printer}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return out.String()
}

func TestAuthStatusReportsIdentityAndCredentialSource(t *testing.T) {
	profile := `{
		"id": 42,
		"name": "Ada Lovelace",
		"email": "ada@example.com",
		"role": "agent",
		"availability_status": "online"
	}`
	defer setupAuthStatusEnv(t, profile)()

	got := runAndCapture(t, (&AuthStatusCmd{}).Run)

	wants := []string{
		"Ada Lovelace",
		"ada@example.com",
		"agent",
		"online",
		"User ID:", "42",
		"Account:", "1",
		"Credential:", "environment",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestLoginSuccessMessageStripsTerminalControls(t *testing.T) {
	got := loginSuccessMessage("Eve\x1b]52;c;Zm9v\a", "eve@example.com\x1b[31m")
	for _, disallowed := range []string{"\x1b", "\a", "]52", "[31m"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("login success message contained terminal control %q: %q", disallowed, got)
		}
	}
	if !strings.Contains(got, "Logged in as Eve (eve@example.com)") {
		t.Fatalf("login success message stripped printable content: %q", got)
	}
}

func TestMeAndWhoamiAliasAuthStatus(t *testing.T) {
	profile := `{
		"id": 7,
		"name": "Grace Hopper",
		"email": "grace@example.com",
		"role": "administrator",
		"availability_status": "offline"
	}`
	defer setupAuthStatusEnv(t, profile)()

	authOut := runAndCapture(t, (&AuthStatusCmd{}).Run)
	meOut := runAndCapture(t, (&MeCmd{}).Run)
	whoamiOut := runAndCapture(t, (&WhoamiCmd{}).Run)

	if authOut != meOut {
		t.Fatalf("'me' diverged from 'auth status':\nauth status:\n%s\nme:\n%s", authOut, meOut)
	}
	if authOut != whoamiOut {
		t.Fatalf("'whoami' diverged from 'auth status':\nauth status:\n%s\nwhoami:\n%s", authOut, whoamiOut)
	}
}

func TestAuthStatusNotLoggedIn(t *testing.T) {
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")
	// no config saved → IsValid() returns false

	got := runAndCapture(t, (&AuthStatusCmd{}).Run)
	if !strings.Contains(got, "Not logged in") {
		t.Fatalf("expected 'Not logged in', got: %s", got)
	}
	if !strings.Contains(got, "chatwoot auth login") {
		t.Fatalf("expected the message to point at 'chatwoot auth login', got: %s", got)
	}
}

func TestAuthLogoutRemovesKeyringTokenWithoutConfig(t *testing.T) {
	keyring.MockInit()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")

	// Seed the token through the production path so it lands under whichever
	// keyring service the active build profile uses (prod vs dev), without
	// writing config.yaml — this exercises logout with no config present.
	seed := &config.Config{BaseURL: "https://app.chatwoot.com", AccountID: 1}
	if err := config.SaveAPIKey(seed, "stale-token"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}

	if err := (&AuthLogoutCmd{}).Run(&App{}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, _, err := config.ResolveAPIKey(seed); !errors.Is(err, config.ErrAPIKeyNotFound) {
		t.Fatalf("expected logout to delete the keyring token, err = %v", err)
	}
}

func TestAuthStatusSelfHealsCachedUserID(t *testing.T) {
	profile := `{
		"id": 99,
		"name": "Alan Turing",
		"email": "alan@example.com",
		"role": "agent",
		"availability_status": "online"
	}`
	defer setupAuthStatusKeyring(t, profile)()

	pre, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (pre): %v", err)
	}
	if pre.UserID != 0 {
		t.Fatalf("setup: expected UserID=0, got %d", pre.UserID)
	}

	_ = runAndCapture(t, (&AuthStatusCmd{}).Run)

	post, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (post): %v", err)
	}
	if post.UserID != 99 {
		t.Fatalf("expected UserID=99 cached after auth status, got %d", post.UserID)
	}
}

func TestAuthStatusDoesNotCacheUserIDFromEnvironmentToken(t *testing.T) {
	profile := `{
		"id": 99,
		"name": "Alan Turing",
		"email": "alan@example.com",
		"role": "agent",
		"availability_status": "online"
	}`
	defer setupAuthStatusEnv(t, profile)()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (pre): %v", err)
	}
	cfg.UserID = 42
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	_ = runAndCapture(t, (&AuthStatusCmd{}).Run)

	post, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (post): %v", err)
	}
	if post.UserID != 42 {
		t.Fatalf("expected env-token auth status to preserve cached UserID=42, got %d", post.UserID)
	}
}
