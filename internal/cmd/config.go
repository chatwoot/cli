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
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	acct := cfg.DefaultAccount()
	if acct == nil {
		fmt.Println("No configuration found. Run 'chatwoot auth login' to set up.")
		return nil
	}

	credential := credentialStatus(acct)

	detail := []output.KeyValue{
		{Key: "Account", Value: acct.Name},
		{Key: "Base URL", Value: acct.BaseURL},
		{Key: "Account ID", Value: fmt.Sprintf("%d", acct.ID)},
		{Key: "Credential", Value: credential},
	}
	if config.IsDev {
		detail = append(detail, output.KeyValue{Key: "Profile", Value: "dev"})
	}
	app.Printer.PrintDetail(detail)

	return nil
}

func credentialStatus(acct *config.Account) string {
	_, source, err := config.ResolveAPIKey(acct)
	if err == nil {
		return string(source)
	}
	if errors.Is(err, config.ErrAPIKeyNotFound) {
		return string(config.CredentialSourceMissing)
	}
	return "error: " + err.Error()
}
