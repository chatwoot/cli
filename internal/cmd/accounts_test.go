package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
)

func runCapture(t *testing.T, format string, quiet bool, app *App, run func(*App) error) (string, error) {
	t.Helper()
	var out bytes.Buffer
	printer := output.NewPrinter(format, false, quiet)
	printer.Writer = &out
	app.Printer = printer
	err := run(app)
	return out.String(), err
}

func TestAccountsListMarksDefault(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", false, &App{}, (&AccountsListCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"NAME", "HOST", "* chatwoot ", "acme", "chatwoot-staging", "app.chatwoot.com", "staging.chatwoot.com", "42", "Shivam"} {
		if !strings.Contains(out, want) {
			t.Fatalf("accounts output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "* acme") {
		t.Fatalf("non-default account marked:\n%s", out)
	}
}

func TestAccountsListQuietPrintsNames(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", true, &App{}, (&AccountsListCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "chatwoot\nacme\nchatwoot-staging\n" {
		t.Fatalf("quiet output = %q", out)
	}
}

func TestAccountsListJSON(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "json", false, &App{}, (&AccountsListCmd{}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		ID      int    `json:"id"`
		Default bool   `json:"default"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if len(got) != 3 || got[0].Name != "chatwoot" || !got[0].Default || got[1].Default || got[1].ID != 42 {
		t.Fatalf("json accounts = %#v", got)
	}
}

func TestAccountsListWithoutAccounts(t *testing.T) {
	isolateAuthEnv(t)
	out, err := runCapture(t, "text", false, &App{}, (&AccountsListCmd{}).Run)
	if err != nil || !strings.Contains(out, "chatwoot auth login") {
		t.Fatalf("output = %q, err = %v, want a login hint", out, err)
	}
}

func TestAccountsRename(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", false, &App{}, (&AccountsRenameCmd{Old: "chatwoot-staging", New: "stg"}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "chatwoot-staging → stg") {
		t.Fatalf("output = %q", out)
	}
	cfg, _ := config.Load()
	if cfg.Find("stg") == nil || cfg.Find("chatwoot-staging") != nil {
		t.Fatalf("rename not saved: %#v", cfg.Accounts)
	}

	if _, err := runCapture(t, "text", false, &App{}, (&AccountsRenameCmd{Old: "stg", New: "acme"}).Run); err == nil {
		t.Fatal("rename onto an existing name succeeded")
	}
}

func TestUseSetsDefault(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", false, &App{}, (&UseCmd{Name: "acme"}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "Default account: acme (app.chatwoot.com #42)") {
		t.Fatalf("output = %q", out)
	}
	if cfg, _ := config.Load(); cfg.Default != "acme" {
		t.Fatalf("default = %q, want acme", cfg.Default)
	}

	// Unique prefixes work like @name.
	if _, err := runCapture(t, "text", false, &App{}, (&UseCmd{Name: "chatwoot-s"}).Run); err != nil {
		t.Fatalf("use by prefix: %v", err)
	}
	if cfg, _ := config.Load(); cfg.Default != "chatwoot-staging" {
		t.Fatalf("default = %q, want chatwoot-staging", cfg.Default)
	}

	for _, bad := range []string{"nope", "chat", "99"} {
		if _, err := runCapture(t, "text", false, &App{}, (&UseCmd{Name: bad}).Run); err == nil {
			t.Errorf("use %q succeeded, want error", bad)
		}
	}
}

func TestUseWithoutNameShowsDefault(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", false, &App{}, (&UseCmd{}).Run)
	if err != nil || !strings.Contains(out, "chatwoot (app.chatwoot.com #1)") {
		t.Fatalf("output = %q, err = %v", out, err)
	}
}

// profileServer serves /api/v1/profile with the given accounts for user id.
func profileServer(t *testing.T, userID int, accounts string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":` + itoa(userID) + `,"name":"Shivam","accounts":` + accounts + `}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// saveLogin registers memberships for one login and stores its token.
func saveLogin(t *testing.T, cfg *config.Config, baseURL string, userID int, memberships ...config.Membership) {
	t.Helper()
	cfg.SyncAccounts(baseURL, userID, "Shivam", memberships)
	acct := cfg.FindByID(baseURL, userID, memberships[0].ID)
	if err := config.SaveAPIKey(acct, "token"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
}

func TestAccountsRefreshSyncsEveryLogin(t *testing.T) {
	isolateAuthEnv(t)
	app := profileServer(t, 7, `[{"id":1,"name":"Chatwoot"},{"id":91,"name":"Initech"}]`)
	down := profileServer(t, 3, `[]`)
	down.Close() // this login can't be reached; refresh must carry on

	cfg := &config.Config{}
	saveLogin(t, cfg, app.URL, 7, config.Membership{ID: 1, Name: "Chatwoot"}, config.Membership{ID: 77, Name: "Globex"})
	saveLogin(t, cfg, down.URL, 3, config.Membership{ID: 5, Name: "QA"})
	cfg.Default = "chatwoot"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	out, err := runCapture(t, "text", false, &App{}, (&AccountsListCmd{Refresh: true}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"+ initech", "#91", "- globex", "no longer a member", config.DisplayHost(down.URL)} {
		if !strings.Contains(out, want) {
			t.Fatalf("refresh output missing %q:\n%s", want, out)
		}
	}

	saved, _ := config.Load()
	if saved.Find("initech") == nil || saved.Find("globex") != nil {
		t.Fatalf("refresh not saved: %v", saved.Accounts)
	}
	if saved.Find("qa") == nil {
		t.Fatal("an unreachable login's accounts must be kept")
	}
}

// An @name the config doesn't know triggers one refresh before failing, so an
// account added in Chatwoot since login just works.
func TestNewAppRefreshesOnceForUnknownName(t *testing.T) {
	isolateAuthEnv(t)
	server := profileServer(t, 7, `[{"id":1,"name":"Chatwoot"},{"id":91,"name":"Newco"}]`)

	cfg := &config.Config{}
	saveLogin(t, cfg, server.URL, 7, config.Membership{ID: 1, Name: "Chatwoot"})
	cfg.Default = "chatwoot"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	app, err := NewApp(&CLI{Output: "text", Account: "newco"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp(newco): %v", err)
	}
	if app.Client.AccountID != 91 {
		t.Fatalf("client account = %d, want 91", app.Client.AccountID)
	}
	if saved, _ := config.Load(); saved.Find("newco") == nil {
		t.Fatal("refreshed account was not saved")
	}

	if _, err := NewApp(&CLI{Output: "text", Account: "still-missing"}, false, "test"); !errors.Is(err, config.ErrUnknownAccount) {
		t.Fatalf("NewApp(still-missing) error = %v, want ErrUnknownAccount", err)
	}
}

func TestAuthLogoutURLRemovesOnlyThatInstance(t *testing.T) {
	isolateAuthEnv(t)
	saveTwoLogins(t)

	out, err := runCapture(t, "text", false, &App{}, (&AuthLogoutCmd{URL: "staging.chatwoot.com"}).Run)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "staging.chatwoot.com") {
		t.Fatalf("output = %q", out)
	}

	cfg, _ := config.Load()
	if cfg == nil || cfg.Find("chatwoot-staging") != nil || cfg.Find("chatwoot") == nil {
		t.Fatalf("after logout: %#v", cfg)
	}
	if _, _, err := config.ResolveAPIKey(&config.Account{BaseURL: "https://staging.chatwoot.com", ID: 1, UserID: 3}); !errors.Is(err, config.ErrAPIKeyNotFound) {
		t.Fatalf("staging token survived logout: %v", err)
	}
	if _, _, err := config.ResolveAPIKey(cfg.Find("chatwoot")); err != nil {
		t.Fatalf("app token removed: %v", err)
	}
}
