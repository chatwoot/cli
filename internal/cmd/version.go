package cmd

import "fmt"

// VersionCmd is `chatwoot version` — prints the CLI version. Equivalent to
// the `--version` flag, exposed as a subcommand for discoverability.
type VersionCmd struct{}

func (c *VersionCmd) Run(app *App) error {
	v := app.Version
	if v == "" {
		v = "dev"
	}
	_, err := fmt.Fprintln(app.Printer.Writer, v)
	return err
}
