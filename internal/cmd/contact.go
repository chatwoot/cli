package cmd

import (
	"fmt"
	"strconv"

	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// -----------------------------------------------------------------------------
// Plural: `chatwoot contacts` — list or search contacts.
// -----------------------------------------------------------------------------

type ContactsCmd struct {
	Search string `help:"Search query (name, email, phone)."`
	Page   int    `short:"p" default:"1" help:"Page number."`
	Sort   string `help:"Sort by name, email, phone_number, last_activity_at; '-' prefix for descending."`
}

func (c *ContactsCmd) Run(app *App) error {
	var resp *sdk.ContactsListResponse
	var err error

	if c.Search != "" {
		resp, err = app.Client.Contacts().Search(sdk.ContactsSearchOptions{
			Query: c.Search,
			Page:  c.Page,
			Sort:  c.Sort,
		})
	} else {
		resp, err = app.Client.Contacts().List(sdk.ContactsListOptions{
			Page: c.Page,
			Sort: c.Sort,
		})
	}
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

// -----------------------------------------------------------------------------
// Singular: `chatwoot contact <verb> <id>` — context for one contact.
// -----------------------------------------------------------------------------

type ContactCmd struct {
	View          ContactViewCmd          `cmd:"" default:"withargs" help:"View a contact (default)."`
	Conversations ContactConversationsCmd `cmd:"" help:"List conversations for the contact."`
}

type ContactViewCmd struct {
	ID int `arg:"" help:"Contact ID."`
}

func (c *ContactViewCmd) Run(app *App) error {
	contact, err := app.Client.Contacts().Get(c.ID)
	if err != nil {
		return err
	}
	return renderContact(app, contact)
}

func renderContact(app *App, contact *sdk.ContactFull) error {
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

type ContactConversationsCmd struct {
	ID int `arg:"" help:"Contact ID."`
}

func (c *ContactConversationsCmd) Run(app *App) error {
	resp, err := app.Client.Contacts().Conversations(c.ID)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No conversations found for this contact.")
		return nil
	}

	headers := []string{"ID", "Status", "Assignee", "Inbox", "Last Activity"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, conv := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(conv.ID),
			conv.Status,
			assigneeName(conv),
			conv.Meta.Channel,
			formatTimestamp(conv.LastActivityAt),
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}
