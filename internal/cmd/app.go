package cmd

import (
	"fmt"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/output"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

// App holds shared state passed to every command's Run method.
type App struct {
	Client  *sdk.Client
	Printer *output.Printer
	Config  *config.Config
}

// NewApp creates an App from the parsed CLI flags.
// Commands that don't need auth (auth login/logout, config) pass skipAuth=true.
func NewApp(cli *CLI, skipAuth bool) (*App, error) {
	printer := output.NewPrinter(cli.Output, cli.NoColor, cli.Quiet)

	if skipAuth {
		return &App{Printer: printer}, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil || !cfg.IsValid() {
		return nil, fmt.Errorf("not authenticated. Run 'chatwoot auth login' to set up credentials")
	}

	if cli.Account > 0 {
		cfg.AccountID = cli.Account
	}

	client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)

	return &App{
		Client:  client,
		Printer: printer,
		Config:  cfg,
	}, nil
}
