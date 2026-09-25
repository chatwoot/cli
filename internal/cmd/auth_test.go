package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
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

	saveTestConfig(t, config.Account{BaseURL: server.URL, ID: 1})
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

	// An account saved before its user ID was known, authenticated by the
	// keyring entry written by earlier releases.
	saveTestConfig(t, config.Account{BaseURL: server.URL, ID: 1})
	seedV1Keyring(t, server.URL, 1, "test-token")

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

func TestVerifyAccountAccess(t *testing.T) {
	accounts := []sdk.ProfileAccount{
		{ID: 7, Name: "Acme", Role: "administrator"},
		{ID: 9, Name: "Beta", Role: "agent"},
	}

	if err := verifyAccountAccess(&sdk.ProfileResponse{Accounts: accounts}, 9); err != nil {
		t.Fatalf("expected access to a member account, got error: %v", err)
	}

	err := verifyAccountAccess(&sdk.ProfileResponse{Accounts: accounts}, 42)
	if err == nil {
		t.Fatal("expected error for non-member account, got nil")
	}
	// The message should name the accessible accounts so the user can correct the ID.
	for _, want := range []string{"42", "7 (Acme)", "9 (Beta)"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}

	// No accounts in payload (older instances) → skip rather than block login.
	if err := verifyAccountAccess(&sdk.ProfileResponse{}, 42); err != nil {
		t.Fatalf("expected skip when no accounts present, got error: %v", err)
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
	seed := &config.Account{BaseURL: "https://app.chatwoot.com", ID: 1, UserID: 5}
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
	if pre.DefaultAccount().UserID != 0 {
		t.Fatalf("setup: expected UserID=0, got %d", pre.DefaultAccount().UserID)
	}

	_ = runAndCapture(t, (&AuthStatusCmd{}).Run)

	post, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (post): %v", err)
	}
	if post.DefaultAccount().UserID != 99 {
		t.Fatalf("expected UserID=99 cached after auth status, got %d", post.DefaultAccount().UserID)
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
	cfg.DefaultAccount().UserID = 42
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	_ = runAndCapture(t, (&AuthStatusCmd{}).Run)

	post, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (post): %v", err)
	}
	if post.DefaultAccount().UserID != 42 {
		t.Fatalf("expected env-token auth status to preserve cached UserID=42, got %d", post.DefaultAccount().UserID)
	}
}

const loginProfileBody = `{"id":5,"name":"Eve","email":"eve@example.com","availability_status":"online","role":"agent",` +
	`"accounts":[{"id":7,"name":"Acme","role":"administrator"},{"id":9,"name":"Beta","role":"agent"}]}`

// TestAuthLoginRegistersEveryAccount drives the full `auth login` flow (stdin →
// profile fetch → account discovery → persist).
func TestAuthLoginRegistersEveryAccount(t *testing.T) {
	server := loginProfileServer(t, loginProfileBody)
	defer server.Close()
	isolateAuthEnv(t)

	// URL, token, then Enter to accept the first account as the default.
	out, err := runLogin(t, &AuthLoginCmd{}, server.URL+"\ntoken\n\n")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		t.Fatalf("config not saved: cfg=%#v err=%v", cfg, err)
	}
	acme, beta := cfg.Find("acme"), cfg.Find("beta")
	if acme == nil || acme.ID != 7 || acme.UserID != 5 || beta == nil || beta.ID != 9 {
		t.Fatalf("accounts = %#v, want acme #7 and beta #9 for user 5", cfg.Accounts)
	}
	if cfg.Default != "acme" {
		t.Fatalf("default = %q, want acme (first account)", cfg.Default)
	}
	for _, acct := range []*config.Account{acme, beta} {
		apiKey, source, err := config.ResolveAPIKey(acct)
		if err != nil || apiKey != "token" || source != config.CredentialSourceKeyring {
			t.Fatalf("ResolveAPIKey(%s) = (%q, %v, %v), want token/keyring", acct.Name, apiKey, source, err)
		}
	}
	for _, want := range []string{"Logged in as Eve", "Found 2 accounts", "acme", "#7", "beta", "#9", "Default set to acme"} {
		if !strings.Contains(out, want) {
			t.Fatalf("login output missing %q:\n%s", want, out)
		}
	}
}

func TestAuthLoginAsksForDefaultAccount(t *testing.T) {
	server := loginProfileServer(t, loginProfileBody)
	defer server.Close()
	isolateAuthEnv(t)

	if _, err := runLogin(t, &AuthLoginCmd{}, server.URL+"\ntoken\nbeta\n"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if cfg, _ := config.Load(); cfg.Default != "beta" {
		t.Fatalf("default = %q, want beta", cfg.Default)
	}
}

// A pasted dashboard link names the account, so no prompt is needed.
func TestAuthLoginURLArgumentWithAccountSetsDefault(t *testing.T) {
	server := loginProfileServer(t, loginProfileBody)
	defer server.Close()
	isolateAuthEnv(t)

	cmd := &AuthLoginCmd{URL: server.URL + "/app/accounts/9/conversations/1"}
	if _, err := runLogin(t, cmd, "token\n"); err != nil {
		t.Fatalf("login: %v", err)
	}
	cfg, _ := config.Load()
	if cfg.Default != "beta" || cfg.Find("beta").BaseURL != server.URL {
		t.Fatalf("default = %q (%#v), want beta on %s", cfg.Default, cfg.Find("beta"), server.URL)
	}
}

func TestAuthLoginRejectsInaccessibleAccountInURL(t *testing.T) {
	server := loginProfileServer(t, loginProfileBody)
	defer server.Close()
	isolateAuthEnv(t)

	_, err := runLogin(t, &AuthLoginCmd{URL: server.URL + "/app/accounts/42"}, "token\n")
	if err == nil {
		t.Fatal("expected login to fail for an inaccessible account")
	}
	for _, want := range []string{"42", "Acme", "Beta"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q should name entered + accessible accounts", err.Error())
		}
	}
	// Nothing must be persisted when login is rejected.
	if cfg, _ := config.Load(); cfg != nil {
		t.Fatalf("config was saved despite a rejected login: %#v", cfg)
	}
}

func TestAuthLoginSingleAccountNeedsNoPrompt(t *testing.T) {
	server := loginProfileServer(t, `{"id":5,"name":"Eve","accounts":[{"id":7,"name":"Acme"}]}`)
	defer server.Close()
	isolateAuthEnv(t)

	if _, err := runLogin(t, &AuthLoginCmd{URL: server.URL}, "token\n"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if cfg, _ := config.Load(); cfg.Default != "acme" {
		t.Fatalf("default = %q, want acme", cfg.Default)
	}
}

// Older Chatwoot versions omit the accounts list; fall back to asking.
func TestAuthLoginAsksForAccountIDWhenProfileHasNoAccounts(t *testing.T) {
	server := loginProfileServer(t, `{"id":5,"name":"Eve"}`)
	defer server.Close()
	isolateAuthEnv(t)

	if _, err := runLogin(t, &AuthLoginCmd{URL: server.URL}, "token\n7\n"); err != nil {
		t.Fatalf("login: %v", err)
	}
	cfg, _ := config.Load()
	def := cfg.DefaultAccount()
	if def == nil || def.ID != 7 || def.UserID != 5 {
		t.Fatalf("default = %#v, want account 7 for user 5", def)
	}
}

// Logging in to a second instance adds its accounts and keeps the default.
func TestAuthLoginSecondInstanceKeepsDefault(t *testing.T) {
	isolateAuthEnv(t)
	app := loginProfileServer(t, loginProfileBody)
	defer app.Close()
	staging := loginProfileServer(t, `{"id":3,"name":"Eve","accounts":[{"id":7,"name":"Acme"}]}`)
	defer staging.Close()

	if _, err := runLogin(t, &AuthLoginCmd{URL: app.URL}, "token\n\n"); err != nil {
		t.Fatalf("first login: %v", err)
	}
	out, err := runLogin(t, &AuthLoginCmd{URL: staging.URL}, "staging-token\n")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	cfg, _ := config.Load()
	if cfg.Default != "acme" {
		t.Fatalf("default = %q, want the first login's acme kept", cfg.Default)
	}
	second := cfg.FindByID(staging.URL, 3, 7)
	if second == nil || second.Name == "acme" {
		t.Fatalf("staging account = %#v, want a qualified name", second)
	}
	if !strings.Contains(out, second.Name) {
		t.Fatalf("output should list %s:\n%s", second.Name, out)
	}
	if key, _, _ := config.ResolveAPIKey(second); key != "staging-token" {
		t.Fatalf("staging token = %q, want staging-token", key)
	}
	if key, _, _ := config.ResolveAPIKey(cfg.Find("acme")); key != "token" {
		t.Fatalf("first login token = %q, want token", key)
	}
}

// seedV1Keyring writes the keyring entry format used before multi-account
// support, as an upgrading user would have it.
func seedV1Keyring(t *testing.T, baseURL string, accountID int, apiKey string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"base_url": baseURL, "account_id": accountID, "api_key": apiKey})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := keyring.Set("chatwoot-cli", "api-key", string(data)); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}
}

// isolateAuthEnv gives a test its own HOME + mocked keyring and clears the
// CHATWOOT_API_KEY override so credential resolution exercises the keyring path.
func isolateAuthEnv(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "")
}

func loginProfileServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// runLogin feeds scripted answers to the interactive login prompts via os.Stdin
// and returns everything the command printed, plus its error.
func runLogin(t *testing.T, cmd *AuthLoginCmd, stdin string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := io.WriteString(w, stdin); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	oldStdin, oldStdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = r, stdout
	defer func() {
		os.Stdin, os.Stdout = oldStdin, oldStdout
		_ = r.Close()
		_ = stdout.Close()
	}()

	printer := output.NewPrinter("text", false, false)
	printer.Writer = stdout
	runErr := cmd.Run(&App{Printer: printer})
	out, _ := os.ReadFile(stdout.Name())
	return string(out), runErr
}
