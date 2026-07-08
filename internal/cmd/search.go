package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chatwoot/cli/internal/sdk"
)

type SearchCmd struct {
	Query string `arg:"" help:"Search query."`
	Page  int    `short:"p" default:"1" help:"Page number (applies to all result buckets)."`
	Only  string `help:"Restrict to one bucket: conversations, contacts, messages, or articles." enum:",conversations,contacts,messages,articles" default:""`
}

func (c *SearchCmd) Run(app *App) error {
	resp, err := app.Client.Search().Global(sdk.SearchOptions{
		Query: c.Query,
		Page:  c.Page,
	})
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(payloadOnly(resp, c.Only))
		return nil
	}

	if app.Printer.Format == "csv" {
		if c.Only == "" {
			return fmt.Errorf("csv output needs a single bucket; use --only=<bucket> or -o json")
		}
		return printSearchSection(app, resp, c.Only, false)
	}

	if app.Printer.Quiet {
		return printSearchQuiet(app, resp, c.Only)
	}

	return printSearchText(app, c.Query, resp, c.Only)
}

func printSearchText(app *App, query string, resp *sdk.SearchResponse, only string) error {
	p := resp.Payload
	total := len(p.Conversations) + len(p.Contacts) + len(p.Messages) + len(p.Articles)

	if only != "" {
		return printSearchSection(app, resp, only, false)
	}

	if total == 0 {
		_, _ = fmt.Fprintf(app.Printer.Writer, "No results for %q.\n", query)
		return nil
	}

	first := true
	if len(p.Conversations) > 0 {
		first = printSectionHeader(app, first, "CONVERSATIONS")
		printConversationRows(app, p.Conversations)
	}
	if len(p.Contacts) > 0 {
		first = printSectionHeader(app, first, "CONTACTS")
		printContactRows(app, p.Contacts)
	}
	if len(p.Messages) > 0 {
		first = printSectionHeader(app, first, "MESSAGES")
		printMessageRows(app, p.Messages)
	}
	if len(p.Articles) > 0 {
		_ = printSectionHeader(app, first, "ARTICLES")
		printArticleRows(app, p.Articles)
	}
	return nil
}

// payloadOnly returns a response restricted to a single bucket so that --only is
// honored consistently across every output format (text, csv, quiet, and json).
// An empty bucket returns the response unchanged.
func payloadOnly(resp *sdk.SearchResponse, only string) *sdk.SearchResponse {
	if only == "" {
		return resp
	}
	out := &sdk.SearchResponse{}
	switch only {
	case "conversations":
		out.Payload.Conversations = resp.Payload.Conversations
	case "contacts":
		out.Payload.Contacts = resp.Payload.Contacts
	case "messages":
		out.Payload.Messages = resp.Payload.Messages
	case "articles":
		out.Payload.Articles = resp.Payload.Articles
	}
	return out
}

func printSearchSection(app *App, resp *sdk.SearchResponse, only string, prefixIDs bool) error {
	_ = prefixIDs // reserved for future use; quiet mode handles prefixing separately
	// In CSV mode an empty bucket must still emit a header row (valid CSV) rather
	// than the human-readable "No X found." message, so scripts get parseable output.
	prose := app.Printer.Format != "csv"
	switch only {
	case "conversations":
		if len(resp.Payload.Conversations) == 0 && prose {
			_, _ = fmt.Fprintln(app.Printer.Writer, "No conversations found.")
			return nil
		}
		printConversationRows(app, resp.Payload.Conversations)
	case "contacts":
		if len(resp.Payload.Contacts) == 0 && prose {
			_, _ = fmt.Fprintln(app.Printer.Writer, "No contacts found.")
			return nil
		}
		printContactRows(app, resp.Payload.Contacts)
	case "messages":
		if len(resp.Payload.Messages) == 0 && prose {
			_, _ = fmt.Fprintln(app.Printer.Writer, "No messages found.")
			return nil
		}
		printMessageRows(app, resp.Payload.Messages)
	case "articles":
		if len(resp.Payload.Articles) == 0 && prose {
			_, _ = fmt.Fprintln(app.Printer.Writer, "No articles found.")
			return nil
		}
		printArticleRows(app, resp.Payload.Articles)
	default:
		return fmt.Errorf("unknown bucket: %q", only)
	}
	return nil
}

func printSearchQuiet(app *App, resp *sdk.SearchResponse, only string) error {
	w := app.Printer.Writer
	p := resp.Payload

	if only != "" {
		switch only {
		case "conversations":
			for _, c := range p.Conversations {
				_, _ = fmt.Fprintln(w, c.ID)
			}
		case "contacts":
			for _, c := range p.Contacts {
				_, _ = fmt.Fprintln(w, c.ID)
			}
		case "messages":
			for _, m := range p.Messages {
				_, _ = fmt.Fprintln(w, m.ID)
			}
		case "articles":
			for _, a := range p.Articles {
				_, _ = fmt.Fprintln(w, a.ID)
			}
		default:
			return fmt.Errorf("unknown bucket: %q", only)
		}
		return nil
	}

	for _, c := range p.Conversations {
		_, _ = fmt.Fprintf(w, "conversation:%d\n", c.ID)
	}
	for _, c := range p.Contacts {
		_, _ = fmt.Fprintf(w, "contact:%d\n", c.ID)
	}
	for _, m := range p.Messages {
		_, _ = fmt.Fprintf(w, "message:%d\n", m.ID)
	}
	for _, a := range p.Articles {
		_, _ = fmt.Fprintf(w, "article:%d\n", a.ID)
	}
	return nil
}

func printSectionHeader(app *App, first bool, title string) bool {
	if !first {
		_, _ = fmt.Fprintln(app.Printer.Writer)
	}
	_, _ = fmt.Fprintln(app.Printer.Writer, title)
	return false
}

func printConversationRows(app *App, convs []sdk.ConversationSearchResult) {
	headers := []string{"ID", "Contact", "Inbox", "Channel", "Created"}
	rows := make([][]string, 0, len(convs))
	for _, c := range convs {
		contact := ""
		if c.Contact != nil {
			contact = c.Contact.Name
		}
		inboxName := ""
		channel := ""
		if c.Inbox != nil {
			inboxName = c.Inbox.Name
			channel = c.Inbox.ChannelType
		}
		rows = append(rows, []string{
			strconv.Itoa(c.ID),
			contact,
			inboxName,
			channel,
			formatTimestamp(c.CreatedAt),
		})
	}
	app.Printer.PrintTable(headers, rows)
}

func printContactRows(app *App, contacts []sdk.ContactSearchResult) {
	headers := []string{"ID", "Name", "Email", "Phone", "Last Activity"}
	rows := make([][]string, 0, len(contacts))
	for _, c := range contacts {
		rows = append(rows, []string{
			strconv.Itoa(c.ID),
			c.Name,
			c.Email,
			c.PhoneNumber,
			formatTimestamp(c.LastActivityAt),
		})
	}
	app.Printer.PrintTable(headers, rows)
}

func printMessageRows(app *App, messages []sdk.MessageSearchResult) {
	headers := []string{"ID", "Conv", "Sender", "Time", "Content"}
	rows := make([][]string, 0, len(messages))
	for _, m := range messages {
		sender := ""
		if m.Sender != nil {
			sender = m.Sender.Name
		}
		content := truncate(sanitizeCell(m.Content), 60)
		rows = append(rows, []string{
			strconv.Itoa(m.ID),
			strconv.Itoa(m.ConversationID),
			sender,
			formatTimestamp(m.CreatedAt),
			content,
		})
	}
	app.Printer.PrintTable(headers, rows)
}

func printArticleRows(app *App, articles []sdk.ArticleSearchResult) {
	headers := []string{"ID", "Title", "Locale", "Status", "Updated"}
	rows := make([][]string, 0, len(articles))
	for _, a := range articles {
		rows = append(rows, []string{
			strconv.Itoa(a.ID),
			truncate(sanitizeCell(a.Title), 60),
			a.Locale,
			a.Status,
			formatTimestamp(a.UpdatedAt),
		})
	}
	app.Printer.PrintTable(headers, rows)
}

// sanitizeCell collapses newlines, tabs, and carriage returns into single
// spaces so that tabwriter alignment stays intact.
func sanitizeCell(s string) string {
	r := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}
