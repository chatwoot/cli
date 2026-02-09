package cmd

import (
	"strconv"

	"github.com/chatwoot/chatwoot-cli/internal/output"
)

type ProfileCmd struct{}

func (c *ProfileCmd) Run(app *App) error {
	profile, err := app.Client.Profile().Get()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(profile)
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "ID", Value: strconv.Itoa(profile.ID)},
		{Key: "Name", Value: profile.Name},
		{Key: "Email", Value: profile.Email},
		{Key: "Role", Value: profile.Role},
		{Key: "Availability", Value: profile.AvailabilityStatus},
	})

	return nil
}
