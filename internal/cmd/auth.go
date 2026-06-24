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

	// The token is valid, but that doesn't mean it can access the account ID the
	// user typed. Verify membership now so a typo/wrong account fails here with a
	// clear message instead of as a cryptic 404 on the first account-scoped call.
	if err := verifyAccountAccess(profile, cfg.AccountID); err != nil {
		return err
	}
	cfg.UserID = profile.ID

	profileName := config.ResolveActiveName(app.ProfileName)

	if err := config.SaveAPIKeyFor(profileName, cfg, apiKey); err != nil {
		return err
	}

	if err := config.SaveProfile(profileName, cfg); err != nil {
		_ = config.DeleteAPIKeyFor(profileName)
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Print(loginSuccessMessage(profile.Name, profile.Email))
	if profileName != config.DefaultProfileName {
		fmt.Printf("Saved as profile %q.\n", profileName)
	}
	return nil
}

func loginSuccessMessage(name, email string) string {
	return fmt.Sprintf("Logged in as %s (%s)\n", output.SanitizeText(name), output.SanitizeText(email))
}

// verifyAccountAccess fails login when the entered account ID is not one the
// authenticated user belongs to. The profile payload's accounts array is the
// source of truth. If the instance returns no accounts (older Chatwoot, or a
// token type that omits them), the check is skipped rather than block login.
func verifyAccountAccess(profile *sdk.ProfileResponse, accountID int) error {
	if len(profile.Accounts) == 0 {
		return nil
	}
	for _, acc := range profile.Accounts {
		if acc.ID == accountID {
			return nil
		}
	}
	return fmt.Errorf("account %d is not accessible with this API key; %s", accountID, accessibleAccountsHint(profile.Accounts))
}

func accessibleAccountsHint(accounts []sdk.ProfileAccount) string {
	parts := make([]string, 0, len(accounts))
	for _, acc := range accounts {
		if name := output.SanitizeText(acc.Name); name != "" {
			parts = append(parts, fmt.Sprintf("%d (%s)", acc.ID, name))
		} else {
			parts = append(parts, strconv.Itoa(acc.ID))
		}
	}
	if len(parts) == 1 {
		return "this key can access account " + parts[0]
	}
	return "this key can access accounts: " + strings.Join(parts, ", ")
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
	profileName := config.ResolveActiveName(app.ProfileName)

	// Always clear the keyring entry first, even if no config file exists, so a
	// stale token can't linger after logout.
	if err := config.DeleteAPIKeyFor(profileName); err != nil {
		return err
	}

	store, err := config.LoadStore()
	if err != nil {
		return err
	}
	removed := store.Remove(profileName)

	if store.IsEmpty() {
		path, perr := config.ConfigPath()
		if perr != nil {
			return perr
		}
		if rerr := os.Remove(path); rerr != nil && !os.IsNotExist(rerr) {
			return fmt.Errorf("failed to remove config: %w", rerr)
		}
	} else if err := store.Save(); err != nil {
		return err
	}

	if removed {
		fmt.Printf("Logged out of profile %q.\n", profileName)
	} else {
		fmt.Println("Not logged in.")
	}
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
	name := config.ResolveActiveName(app.ProfileName)
	cfg, err := config.LoadProfile(name)
	if err != nil {
		return err
	}

	if cfg == nil || !cfg.IsValid() {
		_, err := fmt.Fprintln(app.Printer.Writer, "Not logged in. Run 'chatwoot auth login' to authenticate.")
		return err
	}

	apiKey, source, err := config.ResolveAPIKeyFor(name, cfg)
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
	profile, err := client.Profile().Get()
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Self-heal the cached UserID for older saved logins. Environment tokens are
	// temporary overrides and must not rewrite the persisted login identity.
	if source == config.CredentialSourceKeyring && cfg.UserID != profile.ID {
		cfg.UserID = profile.ID
		_ = config.SaveProfile(name, cfg)
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Profile", Value: name},
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
