package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/chatwoot/chatwoot-cli/internal/cmd"
)

var version = "dev"

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("chatwoot"),
		kong.Description("Read-only CLI for Chatwoot."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	)

	// Commands that don't require authentication
	cmdStr := ctx.Command()
	skipAuth := strings.HasPrefix(cmdStr, "auth") ||
		strings.HasPrefix(cmdStr, "config")

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
