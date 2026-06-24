package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
)

// -----------------------------------------------------------------------------
// Plural: `chatwoot profiles` — list saved profiles.
// -----------------------------------------------------------------------------

type ProfilesCmd struct{}

func (c *ProfilesCmd) Run(app *App) error {
	store, err := config.LoadStore()
	if err != nil {
		return err
	}

	names := store.Names()
	if len(names) == 0 {
		_, err := fmt.Fprintln(app.Printer.Writer, "No profiles configured. Run 'chatwoot auth login' to add one.")
		return err
	}

	defaultName := store.DefaultProfile
	if defaultName == "" {
		defaultName = config.DefaultProfileName
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		type profileView struct {
			Name      string `json:"name"`
			BaseURL   string `json:"base_url"`
			AccountID int    `json:"account_id"`
			Default   bool   `json:"default"`
		}
		views := make([]profileView, 0, len(names))
		for _, n := range names {
			p := store.Get(n)
			views = append(views, profileView{Name: n, BaseURL: p.BaseURL, AccountID: p.AccountID, Default: n == defaultName})
		}
		app.Printer.PrintJSON(views)
		return nil
	}

	headers := []string{"Name", "Default", "Base URL", "Account"}
	rows := make([][]string, 0, len(names))
	for _, n := range names {
		p := store.Get(n)
		mark := ""
		if n == defaultName {
			mark = "*"
		}
		rows = append(rows, []string{n, mark, p.BaseURL, strconv.Itoa(p.AccountID)})
	}
	app.Printer.PrintTable(headers, rows)
	return nil
}

// -----------------------------------------------------------------------------
// Singular: `chatwoot profile <name> <verb>` — act on one profile.
// -----------------------------------------------------------------------------

type ProfileCmd struct {
	Show   ProfileShowCmd   `cmd:"" default:"withargs" help:"Show a profile (default)."`
	Use    ProfileUseCmd    `cmd:"" help:"Set a profile as the default."`
	Remove ProfileRemoveCmd `cmd:"" aliases:"rm,delete" help:"Remove a saved profile and its stored token."`
}

// -- show ---------------------------------------------------------------------

type ProfileShowCmd struct {
	Name string `arg:"" optional:"" help:"Profile name (default: the active profile)."`
}

func (c *ProfileShowCmd) Run(app *App) error {
	store, err := config.LoadStore()
	if err != nil {
		return err
	}

	name := c.Name
	if name == "" {
		name = store.ActiveName(app.ProfileName)
	}

	cfg := store.Get(name)
	if cfg == nil {
		return fmt.Errorf("profile %q not found; run 'chatwoot profiles' to list saved profiles", name)
	}

	defaultName := store.DefaultProfile
	if defaultName == "" {
		defaultName = config.DefaultProfileName
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Profile", Value: name},
		{Key: "Default", Value: yesNo(name == defaultName)},
		{Key: "Base URL", Value: cfg.BaseURL},
		{Key: "Account ID", Value: strconv.Itoa(cfg.AccountID)},
		{Key: "Credential", Value: credentialStatus(name, cfg)},
	})
	return nil
}

// -- use ----------------------------------------------------------------------

type ProfileUseCmd struct {
	Name string `arg:"" help:"Profile name to set as the default."`
}

func (c *ProfileUseCmd) Run(app *App) error {
	store, err := config.LoadStore()
	if err != nil {
		return err
	}
	if store.Get(c.Name) == nil {
		return fmt.Errorf("profile %q not found; run 'chatwoot profiles' to list saved profiles", c.Name)
	}

	store.DefaultProfile = c.Name
	if err := store.Save(); err != nil {
		return err
	}

	if app.Printer != nil && app.Printer.Quiet {
		fmt.Println(c.Name)
		return nil
	}
	fmt.Printf("Default profile set to %q.\n", c.Name)
	return nil
}

// -- remove -------------------------------------------------------------------

type ProfileRemoveCmd struct {
	Name string `arg:"" help:"Profile name to remove."`
}

func (c *ProfileRemoveCmd) Run(app *App) error {
	store, err := config.LoadStore()
	if err != nil {
		return err
	}
	if store.Get(c.Name) == nil {
		return fmt.Errorf("profile %q not found", c.Name)
	}

	if err := config.DeleteAPIKeyFor(c.Name); err != nil {
		return err
	}
	store.Remove(c.Name)

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

	fmt.Printf("Removed profile %q.\n", c.Name)
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
