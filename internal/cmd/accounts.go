package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// fetchProfile is swapped in tests that must not reach the network.
var fetchProfile = func(client *sdk.Client) (*sdk.ProfileResponse, error) {
	return client.Profile().Get()
}

// AccountsCmd is `chatwoot accounts` — list, refresh, and rename accounts.
type AccountsCmd struct {
	List   AccountsListCmd   `cmd:"" default:"withargs" help:"List your accounts. You can leave out 'list': 'chatwoot accounts' works too."`
	Rename AccountsRenameCmd `cmd:"" help:"Give an account a name you prefer."`
}

type AccountsListCmd struct {
	Refresh bool `help:"Check for accounts you've joined or left since you logged in."`
}

func (c *AccountsCmd) Help() string {
	return `Accounts show up here when you log in. The one marked * is your default.

Examples:
  chatwoot accounts                                List your accounts
  chatwoot accounts --refresh                      Find new or removed ones
  chatwoot accounts rename chatwoot-staging stg    Use a shorter name
  chatwoot @stg convs                              Use one for one command`
}

// accountView is the JSON shape of one listed account.
type accountView struct {
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	ID          int    `json:"id"`
	AccountName string `json:"account_name,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	UserName    string `json:"user_name,omitempty"`
	Default     bool   `json:"default"`
}

func (c *AccountsListCmd) Run(app *App) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.Accounts) == 0 {
		_, _ = fmt.Fprintln(app.Printer.Writer, "No accounts. Run 'chatwoot auth login' to add one.")
		return nil
	}

	if c.Refresh {
		return refreshAccounts(app, cfg)
	}

	switch {
	case app.Printer.Format == "json" && !app.Printer.Quiet:
		views := make([]accountView, 0, len(cfg.Accounts))
		for _, a := range cfg.Accounts {
			views = append(views, accountView{
				Name: a.Name, BaseURL: a.BaseURL, ID: a.ID, AccountName: a.AccountName,
				UserID: a.UserID, UserName: a.UserName, Default: a.Name == cfg.Default,
			})
		}
		app.Printer.PrintJSON(views)
	case app.Printer.Quiet || app.Printer.Format == "csv":
		rows := make([][]string, 0, len(cfg.Accounts))
		for _, a := range cfg.Accounts {
			rows = append(rows, []string{a.Name, a.BaseURL, strconv.Itoa(a.ID), a.UserName, strconv.FormatBool(a.Name == cfg.Default)})
		}
		app.Printer.PrintTable([]string{"Name", "Base URL", "ID", "User", "Default"}, rows)
	default:
		w := tabwriter.NewWriter(app.Printer.Writer, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "  NAME\tHOST\tID\tUSER")
		for _, a := range cfg.Accounts {
			marker := " "
			if a.Name == cfg.Default {
				marker = "*"
			}
			_, _ = fmt.Fprintf(w, "%s %s\t%s\t%d\t%s\n", marker, a.Name, config.DisplayHost(a.BaseURL), a.ID, output.SanitizeText(a.UserName))
		}
		_ = w.Flush()
	}
	return nil
}

// loginRefresh is the outcome of re-syncing one saved login.
type loginRefresh struct {
	baseURL string
	added   []config.Account
	removed []config.Account
	err     error
}

// refreshLogins re-syncs every saved login (base URL + user) with its profile.
// A login that can't be reached keeps its accounts. It needs the saved tokens,
// so it does nothing when CHATWOOT_API_KEY overrides them.
func refreshLogins(cfg *config.Config) []loginRefresh {
	if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
		return nil
	}
	type login struct {
		baseURL string
		userID  int
	}
	var logins []login
	firstAccount := map[login]config.Account{}
	for _, a := range cfg.Accounts {
		l := login{a.BaseURL, a.UserID}
		if _, ok := firstAccount[l]; !ok {
			firstAccount[l] = a
			logins = append(logins, l)
		}
	}

	results := make([]loginRefresh, 0, len(logins))
	for _, l := range logins {
		result := loginRefresh{baseURL: l.baseURL}
		acct := firstAccount[l]
		apiKey, _, err := config.ResolveAPIKey(&acct)
		if err == nil {
			var profile *sdk.ProfileResponse
			profile, err = fetchProfile(sdk.NewClient(l.baseURL, apiKey, 0))
			if err == nil {
				result.added, result.removed = cfg.SyncAccounts(l.baseURL, profile.ID, output.SanitizeText(profile.Name), profileMemberships(profile))
			}
		}
		result.err = err
		results = append(results, result)
	}
	return results
}

func refreshAccounts(app *App, cfg *config.Config) error {
	if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
		return fmt.Errorf("refresh uses your saved logins; unset %s to run it", config.APIKeyEnv)
	}
	hadDefault := cfg.Default
	results := refreshLogins(cfg)
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	w := tabwriter.NewWriter(app.Printer.Writer, 0, 0, 2, ' ', 0)
	for _, r := range results {
		host := config.DisplayHost(r.baseURL)
		lines := make([]string, 0, len(r.added)+len(r.removed))
		for _, a := range r.added {
			lines = append(lines, fmt.Sprintf("+ %s   #%d", a.Name, a.ID))
		}
		for _, a := range r.removed {
			lines = append(lines, fmt.Sprintf("- %s   (no longer a member)", a.Name))
		}
		switch {
		case r.err != nil:
			lines = []string{"✗ could not refresh: " + r.err.Error()}
		case len(lines) == 0:
			lines = []string{"no changes"}
		}
		for i, line := range lines {
			if i > 0 {
				host = ""
			}
			_, _ = fmt.Fprintf(w, "  %s\t%s\n", host, output.SanitizeText(line))
		}
	}
	_ = w.Flush()

	if hadDefault != "" && cfg.DefaultAccount() == nil {
		_, _ = fmt.Fprintf(app.Printer.Writer, "\n  Your default account %s was removed. Pick one: chatwoot use <name>\n", hadDefault)
	}
	return nil
}

// resolveAccount resolves a selector, refreshing the saved logins once when a
// name is unknown so accounts added in Chatwoot since login just work.
func resolveAccount(cfg *config.Config, selector string) (*config.Account, error) {
	acct, err := cfg.Resolve(selector)
	if !errors.Is(err, config.ErrUnknownAccount) {
		return acct, err
	}
	changed := false
	for _, r := range refreshLogins(cfg) {
		changed = changed || len(r.added) > 0 || len(r.removed) > 0
	}
	if changed {
		_ = saveConfig(cfg)
	}
	return cfg.Resolve(selector)
}

type AccountsRenameCmd struct {
	Old string `arg:"" help:"The account's current name."`
	New string `arg:"" help:"The name you want. Letters, numbers, and dashes."`
}

func (c *AccountsRenameCmd) Run(app *App) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no accounts. Run 'chatwoot auth login' to add one")
	}
	if err := cfg.Rename(c.Old, c.New); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	_, _ = fmt.Fprintf(app.Printer.Writer, "✓ %s → %s\n", c.Old, c.New)
	return nil
}

// UseCmd is `chatwoot use <name>` — change the saved default account.
type UseCmd struct {
	Name string `arg:"" optional:"" help:"The account name, or just its first few letters. Leave out to see your current default."`
}

func (c *UseCmd) Help() string {
	return `Your default account is used whenever a command doesn't name one. This
changes it everywhere: every terminal and script on this machine.

To switch just for one command, put @name first. For one terminal session or
a script, set CHATWOOT_ACCOUNT instead.

Examples:
  chatwoot use acme                       Make acme the default
  chatwoot use                            Show the current default
  chatwoot @acme convs                    Use acme for one command only
  export CHATWOOT_ACCOUNT=acme            Use acme in this terminal only`
}

func (c *UseCmd) Run(app *App) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts. Run 'chatwoot auth login' to add one")
	}

	if strings.TrimSpace(c.Name) == "" {
		def := cfg.DefaultAccount()
		if def == nil {
			_, _ = fmt.Fprintln(app.Printer.Writer, "No default account. Pick one: chatwoot use <name>   (see: chatwoot accounts)")
			return nil
		}
		_, _ = fmt.Fprintf(app.Printer.Writer, "%s (%s #%d)\n", def.Name, config.DisplayHost(def.BaseURL), def.ID)
		return nil
	}

	acct, err := resolveAccount(cfg, c.Name)
	if err != nil {
		return err
	}
	if acct.Name == "" {
		return fmt.Errorf("account %d is not registered (see: chatwoot accounts)", acct.ID)
	}
	cfg.Default = acct.Name
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	_, _ = fmt.Fprintf(app.Printer.Writer, "✓ Default account: %s (%s #%d)\n", acct.Name, config.DisplayHost(acct.BaseURL), acct.ID)
	return nil
}
