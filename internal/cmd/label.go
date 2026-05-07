package cmd

import (
	"fmt"
	"strconv"
)

// LabelsCmd is `chatwoot labels` — list labels defined on the account.
type LabelsCmd struct{}

func (c *LabelsCmd) Run(app *App) error {
	labels, err := app.Client.AccountLabels().List()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(labels)
		return nil
	}

	if len(labels) == 0 {
		fmt.Println("No labels found.")
		return nil
	}

	headers := []string{"ID", "Title", "Color", "Description"}
	rows := make([][]string, 0, len(labels))
	for _, l := range labels {
		rows = append(rows, []string{
			strconv.Itoa(l.ID),
			l.Title,
			l.Color,
			l.Description,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}
