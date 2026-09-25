package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// Finish runs after a successful command. The first run after an upgrade from
// a single-account config names the migrated account and registers the
// user's other accounts, announcing them once when notice is true. It is best
// effort: any failure leaves things as they were for the next run.
func (app *App) Finish(w io.Writer, notice bool) {
	if app.Account == nil || !app.Account.Provisional || app.Client == nil || app.Config == nil {
		return
	}
	// An environment token may belong to someone else; never let it decide
	// which accounts the saved login has.
	if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
		return
	}
	profile, err := fetchProfile(app.Client)
	if err != nil {
		return
	}
	added, _ := app.Config.SyncAccounts(app.Account.BaseURL, profile.ID, output.SanitizeText(profile.Name), profileMemberships(profile))
	if saveConfig(app.Config) != nil || !notice || len(added) == 0 {
		return
	}

	names := make([]string, len(added))
	for i, a := range added {
		names[i] = fmt.Sprintf("%s #%d", a.Name, a.ID)
	}
	_, _ = fmt.Fprintf(w, "\n✓ Multiple accounts are here. You also have access to:\n    %s\n  Use one with: chatwoot @%s convs      (see: chatwoot accounts)\n",
		strings.Join(names, ", "), added[0].Name)
}

var mutatingConvVerbs = []string{"reply", "resolve", "open", "pending", "snooze", "assign", "unassign", "label", "priority"}

// TargetNotice returns the line announcing which account a write goes to, or
// "" when the command reads or runs on the default account.
func TargetNotice(app *App, command string, cli *CLI) string {
	if app == nil || app.Account == nil || app.Config == nil {
		return ""
	}
	if app.Account.Name != "" && app.Account.Name == app.Config.Default {
		return ""
	}
	if !isMutating(command, cli) {
		return ""
	}
	if app.Account.Name != "" {
		return "→ " + app.Account.Name
	}
	return fmt.Sprintf("→ account #%d on %s", app.Account.ID, config.DisplayHost(app.Account.BaseURL))
}

func isMutating(command string, cli *CLI) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "conv", "conversation":
		return len(fields) > 1 && slices.Contains(mutatingConvVerbs, fields[1])
	case "api":
		method := strings.ToUpper(cli.Api.Method)
		if method == "" {
			method = http.MethodGet
			if cli.Api.Data != "" {
				method = http.MethodPost
			}
		}
		return method != http.MethodGet && method != http.MethodHead
	}
	return false
}

// ExplainError adds what to do next when the API rejects the token.
func ExplainError(app *App, err error) error {
	var apiErr *sdk.APIError
	if err == nil || !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized ||
		app == nil || app.Account == nil {
		return err
	}
	base := app.Account.BaseURL
	if strings.TrimSpace(os.Getenv(config.APIKeyEnv)) != "" {
		return fmt.Errorf("%w\n  %s was rejected by %s (401). Check the token, or unset it to use your saved login",
			err, config.APIKeyEnv, config.DisplayHost(base))
	}
	who := "Your token"
	if app.Account.UserName != "" {
		who = "Token for " + app.Account.UserName
	}
	return fmt.Errorf("%w\n  %s on %s was rejected (401). Run: chatwoot auth login %s",
		err, who, config.DisplayHost(base), base)
}
