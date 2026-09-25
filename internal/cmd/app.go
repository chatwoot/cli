package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
	"golang.org/x/term"
)

// App holds shared state passed to every command's Run method.
type App struct {
	Client  *sdk.Client
	Printer *output.Printer
	Config  *config.Config
	// Account is the account this command runs against. It points into
	// Config.Accounts, except for an unregistered `-a <id>` override.
	Account *config.Account
	// Selector is the raw account choice (@name, -a, CHATWOOT_ACCOUNT), for
	// commands that load the config themselves.
	Selector string
	Version  string
}

// saveConfig is swapped in tests to simulate a failing disk.
var saveConfig = config.Save

// loadConfig reads the config and, when it was just converted from the
// single-account format, saves it in the new format. That save is best effort:
// on failure the command runs on the in-memory copy and the next run retries.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.MigratedFromV1() {
		_ = saveConfig(cfg)
	}
	return cfg, nil
}

// NewApp creates an App from the parsed CLI flags.
// Commands that don't need auth (auth login/logout, config) pass skipAuth=true.
func NewApp(cli *CLI, skipAuth bool, version string) (*App, error) {
	printer := output.NewPrinter(cli.Output, cli.NoColor, cli.Quiet)

	if skipAuth {
		return &App{Printer: printer, Selector: cli.Account, Version: version}, nil
	}

	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	if len(cfg.Accounts) == 0 && !strings.Contains(cli.Account, "://") {
		return nil, fmt.Errorf("not authenticated. Run 'chatwoot auth login' to set up credentials")
	}

	acct, err := resolveAccount(cfg, cli.Account)
	var notLoggedIn *config.NotLoggedInError
	if errors.As(err, &notLoggedIn) && isInteractive() {
		cfg, acct, err = offerLogin(printer, cli.Account, notLoggedIn)
	}
	if err != nil {
		return nil, err
	}

	apiKey, _, err := config.ResolveAPIKey(acct)
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	client := sdk.NewClient(
		acct.BaseURL,
		apiKey,
		acct.ID,
		sdk.WithVerbose(cli.Verbose),
	)

	return &App{
		Client:   client,
		Printer:  printer,
		Config:   cfg,
		Account:  acct,
		Selector: cli.Account,
		Version:  version,
	}, nil
}

var defaultIsInteractive = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// isInteractive reports whether prompts can be answered; swapped in tests.
var isInteractive = defaultIsInteractive

// offerLogin handles a pasted link to an instance with no saved login: it asks
// to log in there, then resolves the link again so the command carries on.
func offerLogin(printer *output.Printer, selector string, notLoggedIn *config.NotLoggedInError) (*config.Config, *config.Account, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("! You're not logged in to %s\n? Log in now? (Y/n) ", config.DisplayHost(notLoggedIn.BaseURL))
	if answer := strings.ToLower(readLine(reader)); answer != "" && !strings.HasPrefix(answer, "y") {
		return nil, nil, notLoggedIn
	}

	login := &AuthLoginCmd{URL: selector, reader: reader}
	if err := login.Run(&App{Printer: printer}); err != nil {
		return nil, nil, err
	}
	fmt.Println()

	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	acct, err := cfg.Resolve(selector)
	return cfg, acct, err
}

// registered reports whether the app's account is stored in the config, so
// changes to it can be saved.
func (app *App) registered() bool {
	return app.Config != nil && app.Account != nil && app.Account.Name != "" &&
		app.Config.Find(app.Account.Name) == app.Account
}

// rememberUserID caches the logged-in user's ID so `assign --agent me` needs no
// profile request next time. The token belongs to the user, so every account
// on the instance still missing a user ID learns it too.
func (app *App) rememberUserID(userID int) {
	if app.Account != nil {
		app.Account.UserID = userID
	}
	if app.Config == nil || app.Account == nil {
		return
	}
	for i := range app.Config.Accounts {
		a := &app.Config.Accounts[i]
		if a.BaseURL == app.Account.BaseURL && a.UserID == 0 {
			a.UserID = userID
		}
	}
	_ = saveConfig(app.Config)
}
