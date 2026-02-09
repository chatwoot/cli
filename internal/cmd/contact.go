package cmd

import (
	"fmt"
	"strconv"

	"github.com/chatwoot/chatwoot-cli/internal/output"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

type ContactCmd struct {
	List   ContactListCmd   `cmd:"" default:"1" help:"List contacts."`
	View   ContactViewCmd   `cmd:"" help:"View a contact."`
	Search ContactSearchCmd `cmd:"" help:"Search contacts."`
}

type ContactListCmd struct {
	Page int `short:"p" default:"1" help:"Page number."`
}

func (c *ContactListCmd) Run(app *App) error {
	resp, err := app.Client.Contacts().List(sdk.ContactsListOptions{
		Page: c.Page,
	})
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No contacts found.")
		return nil
	}

	headers := []string{"ID", "Name", "Email", "Phone"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, ct := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(ct.ID),
			ct.Name,
			ct.Email,
			ct.PhoneNumber,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

type ContactViewCmd struct {
	ID int `arg:"" help:"Contact ID."`
}

func (c *ContactViewCmd) Run(app *App) error {
	contact, err := app.Client.Contacts().Get(c.ID)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(contact)
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "ID", Value: strconv.Itoa(contact.ID)},
		{Key: "Name", Value: contact.Name},
		{Key: "Email", Value: contact.Email},
		{Key: "Phone", Value: contact.PhoneNumber},
		{Key: "Company", Value: contact.CompanyName},
		{Key: "Conversations", Value: strconv.Itoa(contact.ConversationsCount)},
		{Key: "Last Activity", Value: formatTimestamp(contact.LastActivityAt)},
		{Key: "Created", Value: formatTimestamp(contact.CreatedAt)},
	})

	return nil
}

type ContactSearchCmd struct {
	Query string `arg:"" help:"Search query (name, email, or phone)."`
	Page  int    `short:"p" default:"1" help:"Page number."`
}

func (c *ContactSearchCmd) Run(app *App) error {
	resp, err := app.Client.Contacts().Search(sdk.ContactsSearchOptions{
		Query: c.Query,
		Page:  c.Page,
	})
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No contacts found.")
		return nil
	}

	headers := []string{"ID", "Name", "Email", "Phone"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, ct := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(ct.ID),
			ct.Name,
			ct.Email,
			ct.PhoneNumber,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}
