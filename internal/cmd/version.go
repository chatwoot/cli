package cmd

import (
	"fmt"
	"strings"

	"github.com/chatwoot/cli/internal/update"
)

// VersionCmd is `chatwoot version` — prints the CLI version. Equivalent to
// the `--version` flag, exposed as a subcommand for discoverability.
type VersionCmd struct {
	Check bool `help:"Check GitHub for a newer release."`
}

func (c *VersionCmd) Run(app *App) error {
	v := app.Version
	if v == "" {
		v = "dev"
	}
	if !c.Check {
		_, err := fmt.Fprintln(app.Printer.Writer, v)
		return err
	}

	latest, err := update.FetchLatest()
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "chatwoot %s\n", update.DisplayVersion(v))
	fmt.Fprintf(&b, "Latest: %s\n\n", update.DisplayVersion(latest))
	switch {
	case v == "dev":
		b.WriteString("Running a dev build.\n")
	case !update.IsOutdated(v, latest):
		b.WriteString("Up to date.\n")
	default:
		b.WriteString("Update available.\n")
		b.WriteString("  curl -fsSL https://chwt.app/install-cli | sh\n")
	}
	_, err = fmt.Fprint(app.Printer.Writer, b.String())
	return err
}
