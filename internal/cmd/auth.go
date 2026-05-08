package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
	"golang.org/x/term"
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

	fmt.Print("Base URL (default: https://app.chatwoot.com): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://app.chatwoot.com"
	}

	apiKey, err := readAPIKey(reader)
	if err != nil {
		return err
	}

	fmt.Print("Account ID: ")
	accountIDStr, _ := reader.ReadString('\n')
	accountIDStr = strings.TrimSpace(accountIDStr)

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	cfg := &config.Config{
		BaseURL:   baseURL,
		AccountID: accountID,
	}

	if !cfg.IsValid() {
		return fmt.Errorf("all fields are required")
	}

	// Validate credentials by fetching profile
	client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
	profile, err := client.Profile().Get()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	cfg.UserID = profile.ID

	if err := config.SaveAPIKey(cfg, apiKey); err != nil {
		return err
	}

	if err := config.Save(cfg); err != nil {
		_ = config.DeleteAPIKey(cfg)
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Logged in as %s (%s)\n", profile.Name, profile.Email)
	return nil
}

func readAPIKey(reader *bufio.Reader) (string, error) {
	fmt.Print("API Key: ")

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		apiKey, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read API key: %w", err)
		}
		return strings.TrimSpace(string(apiKey)), nil
	}

	apiKey, err := reader.ReadString('\n')
	if err != nil && apiKey == "" {
		return "", fmt.Errorf("failed to read API key: %w", err)
	}
	return strings.TrimSpace(apiKey), nil
}

type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(app *App) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	if cfg != nil {
		if err := config.DeleteAPIKey(cfg); err != nil {
			return err
		}
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Not logged in.")
			if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
				fmt.Printf("%s is set in your environment; logout cannot remove environment-provided credentials.\n", config.APIKeyEnv)
			}
			return nil
		}
		return fmt.Errorf("failed to remove config: %w", err)
	}

	fmt.Println("Logged out successfully.")
	if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
		fmt.Printf("%s is set in your environment; logout cannot remove environment-provided credentials.\n", config.APIKeyEnv)
	}
	return nil
}

type AuthStatusCmd struct{}

func (c *AuthStatusCmd) Run(app *App) error { return runAuthStatus(app) }

// runAuthStatus is the shared implementation behind `auth status`, `me`, and
// `whoami`. They all answer "who am I and where am I logged in?" so they
// share output. It also opportunistically refreshes the cached UserID.
func runAuthStatus(app *App) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg == nil || !cfg.IsValid() {
		fmt.Println("Not logged in. Run 'chatwoot auth login' to authenticate.")
		return nil
	}

	apiKey, source, err := config.ResolveAPIKey(cfg)
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
	profile, err := client.Profile().Get()
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Self-heal the cached UserID for older configs that predate the cache.
	if cfg.UserID != profile.ID {
		cfg.UserID = profile.ID
		_ = config.Save(cfg)
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Instance", Value: cfg.BaseURL},
		{Key: "Account", Value: strconv.Itoa(cfg.AccountID)},
		{Key: "User ID", Value: strconv.Itoa(profile.ID)},
		{Key: "Name", Value: profile.Name},
		{Key: "Email", Value: profile.Email},
		{Key: "Role", Value: profile.Role},
		{Key: "Availability", Value: profile.AvailabilityStatus},
		{Key: "Credential", Value: string(source)},
	})

	return nil
}
