package cmd

// MeCmd is `chatwoot me` — alias of `chatwoot auth status`.
type MeCmd struct{}

func (c *MeCmd) Run(app *App) error { return runAuthStatus(app) }
