package cmd

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

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

const defaultBaseURL = "https://app.chatwoot.com"

type AuthLoginCmd struct {
	URL string `arg:"" optional:"" help:"Chatwoot URL, or any link copied from the dashboard."`

	// reader continues a prompt session started elsewhere (a link's offered
	// login), so buffered input isn't lost between two readers.
	reader *bufio.Reader
}

func (c *AuthLoginCmd) Run(app *App) error {
	reader := c.reader
	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}

	raw := strings.TrimSpace(c.URL)
	if raw == "" {
		fmt.Printf("Chatwoot URL (default: %s): ", defaultBaseURL)
		raw = readLine(reader)
		if raw == "" {
			raw = defaultBaseURL
		}
	}
	baseURL, linkAccount, err := config.ParseInstanceURL(raw)
	if err != nil {
		return err
	}

	apiKey, err := readAPIKey(reader)
	if err != nil {
		return err
	}
	if apiKey == "" {
		return fmt.Errorf("an access token is required")
	}

	// Validate credentials by fetching profile
	profile, err := fetchProfile(sdk.NewClient(baseURL, apiKey, 0))
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// A pasted link names an account; make sure this token can reach it so a
	// wrong link fails here instead of as a cryptic 404 later.
	if linkAccount > 0 {
		if err := verifyAccountAccess(profile, linkAccount); err != nil {
			return err
		}
	}

	memberships := profileMemberships(profile)
	// Older Chatwoot versions omit the accounts list; ask for the account. That
	// one ID is not the full membership list, so it is merged, not synced.
	complete := len(memberships) > 0
	if !complete {
		accountID := linkAccount
		if accountID == 0 {
			fmt.Print("Account ID: ")
			accountID, err = strconv.Atoi(readLine(reader))
			if err != nil || accountID <= 0 {
				return fmt.Errorf("invalid account ID")
			}
		}
		memberships = []config.Membership{{ID: accountID}}
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	otherLogin := len(cfg.UserIDs(baseURL)) > 0 && !slices.Contains(cfg.UserIDs(baseURL), profile.ID)
	userName := output.SanitizeText(profile.Name)
	if complete {
		cfg.SyncAccounts(baseURL, profile.ID, userName, memberships)
	} else {
		cfg.MergeAccounts(baseURL, profile.ID, userName, memberships)
	}

	var mine []*config.Account
	for _, m := range memberships {
		if acct := cfg.FindByID(baseURL, profile.ID, m.ID); acct != nil {
			mine = append(mine, acct)
		}
	}
	if len(mine) == 0 {
		return fmt.Errorf("no accounts were registered")
	}

	if err := config.SaveAPIKey(mine[0], apiKey); err != nil {
		return err
	}

	fmt.Print(loginSuccessMessage(profile.Name, profile.Email))
	if otherLogin {
		fmt.Printf("  Added as another login on %s\n", config.DisplayHost(baseURL))
	}
	printFoundAccounts(mine)

	if cfg.DefaultAccount() == nil {
		def := chooseDefault(reader, mine, linkAccount)
		cfg.Default = def.Name
		fmt.Printf("\n✓ Default set to %s.", def.Name)
		for _, acct := range mine {
			if acct.Name != def.Name {
				fmt.Printf(" Switch per command with @%s, or: chatwoot use %s", acct.Name, acct.Name)
				break
			}
		}
		fmt.Println()
	} else {
		fmt.Printf("\n  Use one with: chatwoot @%s convs   (default stays %s)\n", mine[0].Name, cfg.Default)
	}

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

func printFoundAccounts(accounts []*config.Account) {
	noun := "accounts"
	if len(accounts) == 1 {
		noun = "account"
	}
	fmt.Printf("\n  Found %d %s:\n", len(accounts), noun)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, acct := range accounts {
		_, _ = fmt.Fprintf(w, "    %s\t#%d\n", acct.Name, acct.ID)
	}
	_ = w.Flush()
}

// chooseDefault picks the first login's default account: the one a pasted
// link named, the only one, or the user's pick (Enter takes the first).
func chooseDefault(reader *bufio.Reader, accounts []*config.Account, linkAccount int) *config.Account {
	for _, acct := range accounts {
		if acct.ID == linkAccount {
			return acct
		}
	}
	if len(accounts) == 1 {
		return accounts[0]
	}
	for {
		fmt.Printf("\nDefault account (%s): ", accounts[0].Name)
		answer, err := reader.ReadString('\n')
		answer = strings.TrimPrefix(strings.TrimSpace(answer), "@")
		if answer == "" {
			return accounts[0]
		}
		var matches []*config.Account
		for _, acct := range accounts {
			if acct.Name == answer {
				return acct
			}
			if strings.HasPrefix(acct.Name, answer) {
				matches = append(matches, acct)
			}
		}
		if len(matches) == 1 {
			return matches[0]
		}
		if err != nil {
			return accounts[0]
		}
		fmt.Printf("  %q doesn't match one account; type a name from the list.\n", answer)
	}
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func profileMemberships(profile *sdk.ProfileResponse) []config.Membership {
	memberships := make([]config.Membership, 0, len(profile.Accounts))
	for _, acc := range profile.Accounts {
		memberships = append(memberships, config.Membership{ID: acc.ID, Name: output.SanitizeText(acc.Name)})
	}
	return memberships
}

func loginSuccessMessage(name, email string) string {
	if email == "" {
		return fmt.Sprintf("✓ Logged in as %s\n", output.SanitizeText(name))
	}
	return fmt.Sprintf("✓ Logged in as %s (%s)\n", output.SanitizeText(name), output.SanitizeText(email))
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
	fmt.Print("Access token (Profile Settings → Access Token): ")

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

type AuthLogoutCmd struct {
	URL string `arg:"" optional:"" help:"Log out of one Chatwoot instance only."`
}

func (c *AuthLogoutCmd) Run(app *App) error {
	if strings.TrimSpace(c.URL) != "" {
		return logoutInstance(app, c.URL)
	}

	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	if err := config.DeleteAPIKeys(); err != nil {
		return err
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

// logoutInstance removes one instance's accounts and tokens, keeping the rest.
func logoutInstance(app *App, rawURL string) error {
	baseURL, _, err := config.ParseInstanceURL(rawURL)
	if err != nil {
		return err
	}
	host := config.DisplayHost(baseURL)
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	userIDs := cfg.UserIDs(baseURL)
	removed := cfg.RemoveBaseURL(baseURL)
	if err := config.DeleteBaseURLAPIKeys(baseURL, userIDs); err != nil {
		return err
	}
	if len(removed) == 0 {
		_, _ = fmt.Fprintf(app.Printer.Writer, "Not logged in to %s.\n", host)
		return nil
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintf(app.Printer.Writer, "✓ Logged out of %s (removed %d accounts).\n", host, len(removed))
	if cfg.DefaultAccount() == nil && len(cfg.Accounts) > 0 {
		_, _ = fmt.Fprintln(app.Printer.Writer, "  No default account now. Pick one: chatwoot use <name>")
	}
	return nil
}

type AuthStatusCmd struct{}

func (c *AuthStatusCmd) Run(app *App) error { return runAuthStatus(app) }

// runAuthStatus is the shared implementation behind `auth status`, `me`, and
// `whoami`. They all answer "who am I and where am I logged in?" so they
// share output. It also opportunistically refreshes the cached UserID.
func runAuthStatus(app *App) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if cfg == nil || len(cfg.Accounts) == 0 {
		_, err := fmt.Fprintln(app.Printer.Writer, "Not logged in. Run 'chatwoot auth login' to authenticate.")
		return err
	}
	acct, err := resolveAccount(cfg, app.Selector)
	if err != nil {
		return err
	}

	apiKey, source, err := config.ResolveAPIKey(acct)
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	client := sdk.NewClient(acct.BaseURL, apiKey, acct.ID)
	profile, err := fetchProfile(client)
	if err != nil {
		return fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Self-heal the cached UserID for older saved logins. Environment tokens are
	// temporary overrides and must not rewrite the persisted login identity.
	if source == config.CredentialSourceKeyring && acct.Name != "" && acct.UserID != profile.ID {
		acct.UserID = profile.ID
		_ = saveConfig(cfg)
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Instance", Value: acct.BaseURL},
		{Key: "Account", Value: strconv.Itoa(acct.ID)},
		{Key: "Account Name", Value: acct.Name},
		{Key: "User ID", Value: strconv.Itoa(profile.ID)},
		{Key: "Name", Value: profile.Name},
		{Key: "Email", Value: profile.Email},
		{Key: "Role", Value: profile.Role},
		{Key: "Availability", Value: profile.AvailabilityStatus},
		{Key: "Credential", Value: string(source)},
	})

	return nil
}
