package cmd

import (
	"errors"
	"fmt"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
)

type ConfigCmd struct {
	Path   ConfigPathCmd   `cmd:"" help:"Print the config file path."`
	View   ConfigViewCmd   `cmd:"" help:"Print current configuration."`
	Writes ConfigWritesCmd `cmd:"" help:"Enable or disable write commands."`
}

type ConfigPathCmd struct{}

func (c *ConfigPathCmd) Run(app *App) error {
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

type ConfigViewCmd struct{}

func (c *ConfigViewCmd) Run(app *App) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg == nil {
		fmt.Println("No configuration found. Run 'chatwoot auth login' to set up.")
		return nil
	}

	credential := credentialStatus(cfg)

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Base URL", Value: cfg.BaseURL},
		{Key: "Account ID", Value: fmt.Sprintf("%d", cfg.AccountID)},
		{Key: "Writes", Value: writeStatus(cfg.WritesEnabled)},
		{Key: "Credential", Value: credential},
	})

	return nil
}

type ConfigWritesCmd struct {
	On  ConfigWritesOnCmd  `cmd:"" help:"Enable write commands."`
	Off ConfigWritesOffCmd `cmd:"" help:"Disable write commands."`
}

type ConfigWritesOnCmd struct{}

func (c *ConfigWritesOnCmd) Run(app *App) error {
	return setWritesEnabled(app, true)
}

type ConfigWritesOffCmd struct{}

func (c *ConfigWritesOffCmd) Run(app *App) error {
	return setWritesEnabled(app, false)
}

func setWritesEnabled(app *App, enabled bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.IsValid() {
		return fmt.Errorf("no valid configuration found. Run 'chatwoot auth login' to set up")
	}

	cfg.WritesEnabled = enabled
	if err := config.Save(cfg); err != nil {
		return err
	}

	_, err = fmt.Fprintf(app.Printer.Writer, "Writes %s.\n", writeStatus(enabled))
	return err
}

func writeStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func credentialStatus(cfg *config.Config) string {
	_, source, err := config.ResolveAPIKey(cfg)
	if err == nil {
		return string(source)
	}
	if errors.Is(err, config.ErrAPIKeyNotFound) {
		return string(config.CredentialSourceMissing)
	}
	return "error: " + err.Error()
}
