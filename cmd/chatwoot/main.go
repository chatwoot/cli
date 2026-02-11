package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/chatwoot/chatwoot-cli/internal/cmd"
	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	"github.com/chatwoot/chatwoot-cli/internal/tui"
	"github.com/willabides/kongplete"
)

var version = "dev"

func main() {
	// No args → launch interactive TUI
	if len(os.Args) == 1 {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if cfg == nil || !cfg.IsValid() {
			fmt.Fprintln(os.Stderr, "Not authenticated. Run: chatwoot auth login")
			os.Exit(1)
		}
		client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)
		if err := tui.Run(client, cfg.AccountID, version); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var cli cmd.CLI
	parser := kong.Must(&cli,
		kong.Name("chatwoot"),
		kong.Description("Read-only CLI for Chatwoot."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	)

	// Enable shell completions (must be called before Parse)
	kongplete.Complete(parser)

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	// Commands that don't require authentication
	cmdStr := ctx.Command()
	skipAuth := strings.HasPrefix(cmdStr, "auth") ||
		strings.HasPrefix(cmdStr, "config") ||
		strings.HasPrefix(cmdStr, "install-completions")

	app, err := cmd.NewApp(&cli, skipAuth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ctx.Run(app); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
