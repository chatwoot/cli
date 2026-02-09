package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/output"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Login with your Chatwoot credentials."`
	Logout AuthLogoutCmd `cmd:"" help:"Logout and remove saved credentials."`
	Status AuthStatusCmd `cmd:"" help:"Show current authentication status."`
}

type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(app *App) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Chatwoot CLI Login")
	fmt.Println("==================")

	fmt.Print("Base URL (e.g. https://app.chatwoot.com): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)

	fmt.Print("API Key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	fmt.Print("Account ID: ")
	accountIDStr, _ := reader.ReadString('\n')
	accountIDStr = strings.TrimSpace(accountIDStr)

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	cfg := &config.Config{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		AccountID: accountID,
	}

	if !cfg.IsValid() {
		return fmt.Errorf("all fields are required")
	}

	// Validate credentials by fetching profile
	client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)
	profile, err := client.Profile().Get()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Logged in as %s (%s)\n", profile.Name, profile.Email)
	return nil
}

type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(app *App) error {
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Not logged in.")
			return nil
		}
		return fmt.Errorf("failed to remove config: %w", err)
	}

	fmt.Println("Logged out successfully.")
	return nil
}

type AuthStatusCmd struct{}

func (c *AuthStatusCmd) Run(app *App) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg == nil || !cfg.IsValid() {
		fmt.Println("Not logged in. Run 'chatwoot auth login' to authenticate.")
		return nil
	}

	client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)
	profile, err := client.Profile().Get()
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Instance", Value: cfg.BaseURL},
		{Key: "Account", Value: strconv.Itoa(cfg.AccountID)},
		{Key: "Name", Value: profile.Name},
		{Key: "Email", Value: profile.Email},
		{Key: "Role", Value: profile.Role},
		{Key: "Availability", Value: profile.AvailabilityStatus},
	})

	return nil
}
