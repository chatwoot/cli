package cmd

import (
	"fmt"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// App holds shared state passed to every command's Run method.
type App struct {
	Client  *sdk.Client
	Printer *output.Printer
	Config  *config.Config
	// ProfileName is the resolved active profile (--profile → CHATWOOT_PROFILE →
	// default), set for every command including those that skip auth.
	ProfileName string
	Version     string
}

// NewApp creates an App from the parsed CLI flags.
// Commands that don't need auth (auth login/logout, config, profile) pass
// skipAuth=true.
func NewApp(cli *CLI, skipAuth bool, version string) (*App, error) {
	printer := output.NewPrinter(cli.Output, cli.NoColor, cli.Quiet)

	// Skip-auth commands (auth, config, profile, version) must keep working when
	// the config file is missing or corrupt, so they don't load it here; they
	// resolve the active profile themselves, non-fatally.
	if skipAuth {
		return &App{Printer: printer, ProfileName: cli.Profile, Version: version}, nil
	}

	store, err := config.LoadStore()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	profileName := store.ActiveName(cli.Profile)

	cfg := store.Get(profileName)
	if cfg == nil || !cfg.IsValid() {
		return nil, notAuthenticatedError(profileName, cli.Profile)
	}

	effectiveCfg := *cfg
	if cli.Account > 0 {
		effectiveCfg.AccountID = cli.Account
	}

	apiKey, _, err := config.ResolveAPIKeyFor(profileName, &effectiveCfg)
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	client := sdk.NewClient(
		effectiveCfg.BaseURL,
		apiKey,
		effectiveCfg.AccountID,
		sdk.WithVerbose(cli.Verbose),
	)

	return &App{
		Client:      client,
		Printer:     printer,
		Config:      cfg,
		ProfileName: profileName,
		Version:     version,
	}, nil
}

// notAuthenticatedError points the user at the right login command for the
// requested profile.
func notAuthenticatedError(profileName, override string) error {
	if override != "" || profileName != config.DefaultProfileName {
		return fmt.Errorf("not authenticated for profile %q. Run 'chatwoot auth login --profile %s' to set up credentials", profileName, profileName)
	}
	return fmt.Errorf("not authenticated. Run 'chatwoot auth login' to set up credentials")
}
