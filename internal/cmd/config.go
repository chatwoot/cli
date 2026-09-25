package cmd

import (
	"errors"
	"fmt"
	"strconv"

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

// configView is the JSON shape of `config view`.
type configView struct {
	Path     string              `json:"path"`
	Profile  string              `json:"profile,omitempty"`
	Default  string              `json:"default"`
	Accounts []configAccountView `json:"accounts"`
}

type configAccountView struct {
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	ID          int    `json:"id"`
	AccountName string `json:"account_name,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	UserName    string `json:"user_name,omitempty"`
	Credential  string `json:"credential"`
	Default     bool   `json:"default"`
	HelpCenter  *struct {
		Portal string `json:"portal"`
		Locale string `json:"locale,omitempty"`
	} `json:"help_center,omitempty"`
}

func (c *ConfigViewCmd) Run(app *App) error {
	w := app.Printer.Writer
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.Accounts) == 0 {
		_, _ = fmt.Fprintln(w, "No configuration found. Run 'chatwoot auth login' to set up.")
		return nil
	}
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}

	view := configView{Path: path, Default: cfg.Default}
	if config.IsDev {
		view.Profile = "dev"
	}
	// One keyring lookup per login, not per account.
	credentials := map[string]string{}
	for i := range cfg.Accounts {
		a := &cfg.Accounts[i]
		key := fmt.Sprintf("%s#%d", a.BaseURL, a.UserID)
		if _, ok := credentials[key]; !ok {
			credentials[key] = credentialStatus(a)
		}
		av := configAccountView{
			Name: a.Name, BaseURL: a.BaseURL, ID: a.ID, AccountName: a.AccountName,
			UserID: a.UserID, UserName: a.UserName, Credential: credentials[key],
			Default: a.Name == cfg.Default,
		}
		if a.HelpCenter.DefaultPortalSlug != "" {
			av.HelpCenter = &struct {
				Portal string `json:"portal"`
				Locale string `json:"locale,omitempty"`
			}{a.HelpCenter.DefaultPortalSlug, a.HelpCenter.DefaultLocale}
		}
		view.Accounts = append(view.Accounts, av)
	}

	if app.Printer.Format == "json" {
		app.Printer.PrintJSON(view)
		return nil
	}

	line := func(indent, key, value string) {
		_, _ = fmt.Fprintf(w, "%s%-14s %s\n", indent, key+":", output.SanitizeText(value))
	}
	line("", "Config file", view.Path)
	if view.Profile != "" {
		line("", "Profile", view.Profile)
	}
	if view.Default != "" && cfg.DefaultAccount() != nil {
		line("", "Default", view.Default)
	} else {
		line("", "Default", "none — pick one: chatwoot use <name>")
	}

	for _, a := range view.Accounts {
		title := a.Name
		if a.Default {
			title += " (default)"
		}
		_, _ = fmt.Fprintf(w, "\n%s\n", output.SanitizeText(title))
		line("  ", "Base URL", a.BaseURL)
		line("  ", "Account ID", strconv.Itoa(a.ID))
		if a.AccountName != "" {
			line("  ", "Account name", a.AccountName)
		}
		switch {
		case a.UserID == 0:
			line("  ", "User", "not known yet")
		case a.UserName != "":
			line("  ", "User", fmt.Sprintf("%s (#%d)", a.UserName, a.UserID))
		default:
			line("  ", "User", fmt.Sprintf("#%d", a.UserID))
		}
		line("  ", "Credential", a.Credential)
		if a.HelpCenter != nil {
			hc := a.HelpCenter.Portal
			if a.HelpCenter.Locale != "" {
				hc += " (" + a.HelpCenter.Locale + ")"
			}
			line("  ", "Help center", hc)
		}
	}
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
