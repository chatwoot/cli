package cmd

import (
	"fmt"
	"strconv"
)

// TeamsCmd is `chatwoot teams` — list teams on the account.
type TeamsCmd struct{}

func (c *TeamsCmd) Run(app *App) error {
	teams, err := app.Client.Teams().List()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(teams)
		return nil
	}

	if len(teams) == 0 {
		fmt.Println("No teams found.")
		return nil
	}

	headers := []string{"ID", "Name", "Description"}
	rows := make([][]string, 0, len(teams))
	for _, t := range teams {
		rows = append(rows, []string{
			strconv.Itoa(t.ID),
			t.Name,
			t.Description,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}
