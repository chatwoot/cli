package cmd

import (
	"fmt"
	"strconv"

	"github.com/chatwoot/chatwoot-cli/internal/output"
)

type InboxCmd struct {
	List InboxListCmd `cmd:"" default:"1" help:"List inboxes."`
	View InboxViewCmd `cmd:"" help:"View an inbox."`
}

type InboxListCmd struct{}

func (c *InboxListCmd) Run(app *App) error {
	resp, err := app.Client.Inboxes().List()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No inboxes found.")
		return nil
	}

	headers := []string{"ID", "Name", "Channel Type"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, inbox := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(inbox.ID),
			inbox.Name,
			inbox.ChannelType,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

type InboxViewCmd struct {
	ID int `arg:"" help:"Inbox ID."`
}

func (c *InboxViewCmd) Run(app *App) error {
	inbox, err := app.Client.Inboxes().Get(c.ID)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(inbox)
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "ID", Value: strconv.Itoa(inbox.ID)},
		{Key: "Name", Value: inbox.Name},
		{Key: "Channel Type", Value: inbox.ChannelType},
		{Key: "Greeting", Value: inbox.GreetingMessage},
	})

	return nil
}
