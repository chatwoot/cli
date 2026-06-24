package cmd

import (
	"errors"
	"fmt"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
)

type ConfigCmd struct {
	Path ConfigPathCmd `cmd:"" help:"Print the config file path."`
	View ConfigViewCmd `cmd:"" help:"Print current configuration."`
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
	name := config.ResolveActiveName(app.ProfileName)
	cfg, err := config.LoadProfile(name)
	if err != nil {
		return err
	}

	if cfg == nil {
		fmt.Println("No configuration found. Run 'chatwoot auth login' to set up.")
		return nil
	}

	detail := []output.KeyValue{
		{Key: "Profile", Value: name},
		{Key: "Base URL", Value: cfg.BaseURL},
		{Key: "Account ID", Value: fmt.Sprintf("%d", cfg.AccountID)},
		{Key: "Credential", Value: credentialStatus(name, cfg)},
	}
	if config.IsDev {
		detail = append(detail, output.KeyValue{Key: "Build", Value: "dev"})
	}
	app.Printer.PrintDetail(detail)

	return nil
}

func credentialStatus(profile string, cfg *config.Config) string {
	_, source, err := config.ResolveAPIKeyFor(profile, cfg)
	if err == nil {
		return string(source)
	}
	if errors.Is(err, config.ErrAPIKeyNotFound) {
		return string(config.CredentialSourceMissing)
	}
	return "error: " + err.Error()
}
