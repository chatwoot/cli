package cmd

// WhoamiCmd is `chatwoot whoami` — alias of `chatwoot auth status`.
type WhoamiCmd struct{}

func (c *WhoamiCmd) Run(app *App) error { return runAuthStatus(app) }
