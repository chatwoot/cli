package cmd

import (
	"fmt"
	"strings"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/output"
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg == nil {
		fmt.Println("No configuration found. Run 'chatwoot auth login' to set up.")
		return nil
	}

	maskedKey := maskAPIKey(cfg.APIKey)

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Base URL", Value: cfg.BaseURL},
		{Key: "API Key", Value: maskedKey},
		{Key: "Account ID", Value: fmt.Sprintf("%d", cfg.AccountID)},
	})

	return nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
